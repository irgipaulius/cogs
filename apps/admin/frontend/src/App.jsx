import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js';
import { Line } from 'react-chartjs-2';
import { AppDetailView } from './appDetail.jsx';

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler);

const PERMS = ['metrics.read', 'logs.read', 'sql.read', 'sql.write', 'redeploy', 'users.manage'];
const MAX_POINTS = 900;
const CHART_GRID = 'rgba(255, 255, 255, 0.06)';
const CHART_TICK = 'rgba(244, 244, 245, 0.45)';

function parseHash() {
  const raw = (window.location.hash || '#overview').slice(1);
  if (raw.startsWith('app/')) {
    const rest = raw.slice(4);
    const slash = rest.indexOf('/');
    if (slash === -1) {
      return { panel: 'app', app: rest, appTab: 'hosting' };
    }
    return { panel: 'app', app: rest.slice(0, slash), appTab: rest.slice(slash + 1) || 'hosting' };
  }
  return { panel: raw || 'overview', app: null, appTab: null };
}

async function api(path, opts = {}) {
  const r = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  if (r.status === 401) throw new Error('unauthorized');
  if (!r.ok) throw new Error(await r.text());
  const ct = r.headers.get('content-type') || '';
  if (ct.includes('json')) return r.json();
  return r.text();
}

function fmtBytes(n) {
  if (!n) return '—';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}

function StreamingChart({ title, labels, data, color }) {
  const chartRef = useRef(null);
  const dataset = useMemo(
    () => ({
      labels,
      datasets: [
        {
          label: title,
          data,
          borderColor: color,
          backgroundColor: `${color}33`,
          fill: true,
          tension: 0.25,
          pointRadius: 0,
          borderWidth: 2,
        },
      ],
    }),
    [title, labels, data, color],
  );
  return (
    <div className="card chart-card">
      <h3>{title}</h3>
      <div className="chart-wrap">
        <Line
          ref={chartRef}
          data={dataset}
          options={{
            responsive: true,
            maintainAspectRatio: false,
            animation: false,
            interaction: { intersect: false, mode: 'index' },
            plugins: { legend: { display: false } },
            scales: {
              x: { display: false },
              y: {
                grid: { color: CHART_GRID },
                ticks: { color: CHART_TICK, font: { size: 10, family: 'JetBrains Mono' } },
                border: { display: false },
              },
            },
          }}
        />
      </div>
    </div>
  );
}

function PageHeader({ title, subtitle }) {
  return (
    <header className="page-head">
      <h2>{title}</h2>
      {subtitle && <p>{subtitle}</p>}
    </header>
  );
}

export default function App() {
  const [{ panel, app: selectedApp, appTab }, setRoute] = useState(parseHash);
  const [me, setMe] = useState(null);
  const [loginErr, setLoginErr] = useState('');
  const [board, setBoard] = useState(null);
  const [cpuHist, setCpuHist] = useState([]);
  const [memHist, setMemHist] = useState([]);
  const [timeLabels, setTimeLabels] = useState([]);
  const [users, setUsers] = useState([]);
  const [audit, setAudit] = useState([]);
  const [sqlText, setSqlText] = useState('SELECT name FROM sqlite_master WHERE type=\'table\'');
  const [sqlResult, setSqlResult] = useState(null);
  const [sqlError, setSqlError] = useState('');
  const [sqlLoading, setSqlLoading] = useState(false);
  const [tables, setTables] = useState([]);

  useEffect(() => {
    const onHash = () => setRoute(parseHash());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  const go = (hash) => {
    window.location.hash = hash;
    setRoute(parseHash());
  };

  const loadMe = useCallback(async () => {
    try {
      setMe(await api('/api/me'));
    } catch {
      setMe(null);
    }
  }, []);

  useEffect(() => {
    loadMe();
  }, [loadMe]);

  const applyTick = useCallback((sample) => {
    if (!sample?.host) return;
    const label = new Date(sample.at || Date.now()).toLocaleTimeString();
    setTimeLabels((l) => [...l, label].slice(-MAX_POINTS));
    setCpuHist((d) => [...d, sample.host.cpu_percent ?? 0].slice(-MAX_POINTS));
    setMemHist((d) => [...d, sample.host.mem_used_mb ?? 0].slice(-MAX_POINTS));
    setBoard((b) => ({
      ...(b || {}),
      latest: sample,
      telemetry: sample.telemetry ?? b?.telemetry ?? {},
    }));
  }, []);

  const applyHistory = useCallback((payload) => {
    setBoard(payload);
    const hist = payload.history || [];
    if (hist.length) {
      setTimeLabels(hist.map((s) => new Date(s.at).toLocaleTimeString()));
      setCpuHist(hist.map((s) => s.host?.cpu_percent ?? 0));
      setMemHist(hist.map((s) => s.host?.mem_used_mb ?? 0));
    }
  }, []);

  useEffect(() => {
    if (!me || panel !== 'overview') return;
    const es = new EventSource('/api/metrics/stream');
    es.addEventListener('history', (ev) => {
      try {
        applyHistory(JSON.parse(ev.data));
      } catch {
        /* ignore */
      }
    });
    es.addEventListener('tick', (ev) => {
      try {
        applyTick(JSON.parse(ev.data));
      } catch {
        /* ignore */
      }
    });
    es.onmessage = (ev) => {
      try {
        applyTick(JSON.parse(ev.data));
      } catch {
        /* ignore */
      }
    };
    return () => es.close();
  }, [me, panel, applyHistory, applyTick]);

  useEffect(() => {
    if (!me || panel !== 'apps') return;
    const poll = () => api('/api/metrics/latest').then(setBoard).catch(() => {});
    poll();
    const t = setInterval(poll, 4000);
    return () => clearInterval(t);
  }, [me, panel]);

  useEffect(() => {
    if (panel !== 'people' || !me) return;
    if (!me.is_root && !me.grants?.some((g) => g.permission === 'users.manage')) return;
    api('/api/users').then(setUsers).catch(() => setUsers([]));
    if (me.is_root) api('/api/audit').then(setAudit).catch(() => setAudit([]));
  }, [panel, me]);

  const appsList = useMemo(() => {
    const names = new Set();
    (board?.latest?.apps || []).forEach((a) => names.add(a.name));
    Object.keys(board?.deploy_meta || {}).forEach((n) => names.add(n));
    (board?.registry?.apps || []).forEach((a) => names.add(a.app_name));
    return [...names].filter((n) => n !== 'admin').sort();
  }, [board]);

  const appMeta = (name) => board?.deploy_meta?.[name] || {};
  const appMetrics = (name) => (board?.latest?.apps || []).find((a) => a.name === name) || {};
  const appTel = (name) => board?.telemetry?.[name] || {};

  const dbForApp = (name) => (board?.registry?.apps || []).find((a) => a.app_name === name);

  async function onLogin(e) {
    e.preventDefault();
    setLoginErr('');
    const fd = new FormData(e.target);
    try {
      await api('/api/login', {
        method: 'POST',
        body: JSON.stringify({ username: fd.get('username'), password: fd.get('password') }),
      });
      await loadMe();
    } catch (err) {
      setLoginErr(String(err.message || err));
    }
  }

  async function redeploy(name) {
    if (!confirm(`Redeploy ${name}?`)) return;
    await api('/api/redeploy', { method: 'POST', body: JSON.stringify({ app: name }) });
    alert('Redeploy triggered — check deploy history');
  }

  async function rollback(name) {
    if (!confirm(`Roll back ${name} to the previous backend release?`)) return;
    await api('/api/rollback', { method: 'POST', body: JSON.stringify({ app: name }) });
    alert('Rollback complete');
  }

  const deployEvents = board?.deploy_events || [];

  async function runSQL(name, write) {
    const db = dbForApp(name);
    if (!db) {
      setSqlError('No database registered for this app.');
      setSqlResult(null);
      return;
    }
    setSqlLoading(true);
    setSqlError('');
    try {
      setSqlResult(
        await api('/api/sql', {
          method: 'POST',
          body: JSON.stringify({ db_id: db.id, app: name, sql: sqlText, write }),
        }),
      );
    } catch (err) {
      setSqlResult(null);
      setSqlError(String(err.message || err));
    } finally {
      setSqlLoading(false);
    }
  }

  async function loadTables(name) {
    const db = dbForApp(name);
    if (!db) return;
    setTables(
      await api(`/api/sql/tables?db_id=${encodeURIComponent(db.id)}&app=${encodeURIComponent(name)}`),
    );
  }

  if (!me) {
    return (
      <div className="login-wrap">
        <form className="login-box" onSubmit={onLogin}>
          <h1>CoGS</h1>
          <p className="login-tagline">Mission control</p>
          {loginErr && <div className="err">{loginErr}</div>}
          <label>Username</label>
          <input name="username" autoComplete="username" required />
          <label>Password</label>
          <input name="password" type="password" autoComplete="current-password" required />
          <button type="submit" className="btn btn-primary">
            Sign in
          </button>
        </form>
      </div>
    );
  }

  const signOut = () => api('/api/logout', { method: 'POST' }).then(() => setMe(null));

  return (
    <div className="shell">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark" aria-hidden />
          <div className="brand-text">
            <span className="brand-name">CoGS</span>
            <span className="brand-sub">Mission control</span>
          </div>
        </div>
        <div className="topbar-user hide-mobile">
          <span>{me.username}</span>
          {me.is_root && <span className="role">Root</span>}
          <button type="button" className="btn btn-ghost" onClick={signOut}>
            Sign out
          </button>
        </div>
        <div className="topbar-user hide-desktop">
          {me.is_root && <span className="role">Root</span>}
          <button type="button" className="btn btn-ghost" onClick={signOut}>
            Out
          </button>
        </div>
      </header>
      <nav className="nav" aria-label="Main">
        <span className="nav-label hide-mobile">Navigate</span>
        <button type="button" className={panel === 'overview' ? 'active' : ''} onClick={() => go('overview')}>
          Overview
        </button>
        <button type="button" className={panel === 'apps' || panel === 'app' ? 'active' : ''} onClick={() => go('apps')}>
          Apps
        </button>
        {(me.is_root || me.grants?.some((g) => g.permission === 'users.manage')) && (
          <button type="button" className={panel === 'people' ? 'active' : ''} onClick={() => go('people')}>
            People
          </button>
        )}
        <div className="nav-footer hide-mobile">
          <div className="username">{me.username}</div>
          <button type="button" className="btn btn-ghost" onClick={signOut}>
            Sign out
          </button>
        </div>
      </nav>
      <main className="main">
        {panel === 'overview' && (
          <>
            <PageHeader title="Overview" subtitle="Live host telemetry and request activity" />
            <div className="grid2">
              <StreamingChart title="Host CPU %" labels={timeLabels} data={cpuHist} color="#f4f4f5" />
              <StreamingChart title="Memory used (MiB)" labels={timeLabels} data={memHist} color="#5eead4" />
            </div>
            <div className="card">
              <h3>Network (all apps)</h3>
              <div className="grid3">
                {appsList.map((name) => {
                  const t = appTel(name);
                  return (
                    <div key={name} className="mini-stat">
                      <strong>{name}</strong>
                      <span>{t.requests_total ?? 0} req</span>
                      <span>{t.in_flight ?? 0} in flight (peak {t.peak_in_flight ?? 0})</span>
                      <span>{t.unique_ips ?? 0} IPs this session</span>
                    </div>
                  );
                })}
              </div>
            </div>
          </>
        )}

        {panel === 'apps' && (
          <>
            <PageHeader title="Apps" subtitle="Workspace services and runtime health" />
            {me.is_root && <GitHubAndAddApp api={api} onAdded={() => api('/api/metrics/latest').then(setBoard)} />}
            <div className="app-grid">
              {appsList.map((name) => {
                const m = appMetrics(name);
                const meta = appMeta(name);
                const t = appTel(name);
                const live = !!m.running;
                return (
                  <button key={name} type="button" className="app-card" onClick={() => go(`app/${name}/hosting`)}>
                    <div className="app-card-head">
                      <h4>{name}</h4>
                      <span className={`status-pill ${live ? 'live' : 'down'}`}>
                        {live ? 'Live' : 'Offline'}
                      </span>
                    </div>
                    <div className="app-card-stat">
                      <span>
                        {live ? (m.pid ? `PID ${m.pid}` : 'Health check OK') : 'Not responding'}
                      </span>
                      <span>
                        CPU {m.cpu_percent?.toFixed?.(1) ?? '—'}% · RSS {m.rss_mb?.toFixed?.(1) ?? '—'} MiB
                      </span>
                      <span>
                        DB {fmtBytes(m.db_bytes)} · blobs {fmtBytes(m.blob_bytes)}
                      </span>
                      <span>
                        {t.requests_total ?? 0} requests · {t.in_flight ?? 0} active
                      </span>
                    </div>
                    {meta.public_url && <p className="url">{meta.public_url}</p>}
                    {meta.deploy_mode && <p className="muted">Mode {meta.deploy_mode}</p>}
                    {meta.last_status && <p className="muted">Deploy {meta.last_status}</p>}
                    {meta.last_error && <p className="err">{meta.last_error.slice(0, 80)}</p>}
                    {meta.deploy_count > 0 && meta.last_deploy_at && (
                      <p className="muted">Last deploy {new Date(meta.last_deploy_at).toLocaleString()}</p>
                    )}
                  </button>
                );
              })}
            </div>
          </>
        )}

        {panel === 'app' && selectedApp && (
          <AppDetailView
            app={selectedApp}
            tab={appTab || 'hosting'}
            go={go}
            appMeta={appMeta}
            appMetrics={appMetrics}
            api={api}
            me={me}
            redeploy={redeploy}
            rollback={rollback}
            deployEvents={deployEvents}
            sqlText={sqlText}
            setSqlText={setSqlText}
            tables={tables}
            loadTables={loadTables}
            runSQL={runSQL}
            sqlResult={sqlResult}
            sqlError={sqlError}
            sqlLoading={sqlLoading}
          />
        )}

        {panel === 'people' && (
          <>
            <PageHeader title="People" subtitle="Administrators, grants, and audit trail" />
            <div className="card">
              <h3>Users</h3>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>User</th>
                      <th>Root</th>
                      <th>Grants</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(users || []).map((u) => (
                      <tr key={u.id}>
                        <td>{u.username}</td>
                        <td>{u.is_root ? 'yes' : 'no'}</td>
                        <td>{(u.grants || []).map((g) => `${g.app_name}:${g.permission}`).join(', ') || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
            {me.is_root && (
              <div className="card">
                <h3>Audit</h3>
                <div className="table-wrap">
                  <table>
                    <tbody>
                      {(audit || []).map((a) => (
                        <tr key={a.id}>
                          <td>{a.created_at}</td>
                          <td>{a.username}</td>
                          <td>{a.action}</td>
                          <td>{a.detail}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
            <CreateUserForm
              apps={appsList}
              onCreated={() => api('/api/users').then(setUsers)}
            />
          </>
        )}
      </main>
    </div>
  );
}

function CreateUserForm({ onCreated, apps }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [grants, setGrants] = useState([]);
  const safeApps = apps || [];

  async function submit(e) {
    e.preventDefault();
    const { id } = await api('/api/users', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    if (grants.length) {
      await api(`/api/users/${id}/grants`, { method: 'PUT', body: JSON.stringify({ grants }) });
    }
    setUsername('');
    setPassword('');
    setGrants([]);
    onCreated();
  }

  function toggleGrant(app, perm) {
    setGrants((g) => {
      const has = g.some((x) => x.app_name === app && x.permission === perm);
      if (has) return g.filter((x) => !(x.app_name === app && x.permission === perm));
      return [...g, { app_name: app, permission: perm }];
    });
  }

  return (
    <form className="card" onSubmit={submit}>
      <h3>Create admin</h3>
      <input placeholder="Username" value={username} onChange={(e) => setUsername(e.target.value)} required />
      <input placeholder="Password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
      <div className="grant-grid">
        {safeApps.map((a) => (
          <div key={a}>
            <strong>{a}</strong>
            {PERMS.map((p) => {
              const on = grants.some((x) => x.app_name === a && x.permission === p);
              return (
                <button
                  key={p}
                  type="button"
                  className={`chip${on ? ' selected' : ''}`}
                  onClick={() => toggleGrant(a, p)}
                >
                  {p}
                </button>
              );
            })}
          </div>
        ))}
      </div>
      <button type="submit" className="btn btn-primary">
        Create
      </button>
    </form>
  );
}

function GitHubAndAddApp({ api, onAdded }) {
  const [gh, setGh] = useState(null);
  const [flow, setFlow] = useState(null);
  const [repos, setRepos] = useState([]);
  const [name, setName] = useState('');
  const [repo, setRepo] = useState('');
  const [mode, setMode] = useState('push');
  const [subdomain, setSubdomain] = useState('');
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/api/github/status')
      .then(setGh)
      .catch(() => setGh({ signed_in: false }));
  }, [api]);

  useEffect(() => {
    if (!gh?.signed_in) return;
    api('/api/github/repos')
      .then(setRepos)
      .catch(() => setRepos([]));
  }, [gh, api]);

  useEffect(() => {
    if (!flow?.device_code) return;
    const t = setInterval(async () => {
      try {
        const st = await api('/api/github/login/poll', {
          method: 'POST',
          body: JSON.stringify({ device_code: flow.device_code }),
        });
        if (st.signed_in) {
          setFlow(null);
          setGh(st);
        }
      } catch {
        /* still pending */
      }
    }, (flow.interval || 5) * 1000);
    return () => clearInterval(t);
  }, [flow, api]);

  async function startLogin() {
    setErr('');
    const start = await api('/api/github/login/start', { method: 'POST', body: '{}' });
    setFlow(start);
  }

  async function addApp(e) {
    e.preventDefault();
    setErr('');
    try {
      await api('/api/apps', {
        method: 'POST',
        body: JSON.stringify({
          name,
          repo,
          deploy_mode: mode,
          page_subdomain: subdomain || name,
        }),
      });
      onAdded?.();
      setName('');
      setRepo('');
    } catch (ex) {
      setErr(String(ex.message || ex));
    }
  }

  return (
    <div className="card stack">
      <h3>GitHub &amp; new app</h3>
      {err && <div className="err">{err}</div>}
      {!gh?.signed_in ? (
        <div>
          {flow ? (
            <p>
              Open{' '}
              <a href={flow.verification_uri} target="_blank" rel="noreferrer">
                {flow.verification_uri}
              </a>{' '}
              and enter code <strong>{flow.user_code}</strong>
            </p>
          ) : (
            <button type="button" className="btn btn-primary" onClick={startLogin}>
              Login with GitHub
            </button>
          )}
        </div>
      ) : (
        <p className="muted">Signed in as {gh.login || 'GitHub user'}</p>
      )}
      <form className="stack" onSubmit={addApp}>
        <label>
          App id
          <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label>
          Repository
          <select value={repo} onChange={(e) => setRepo(e.target.value)} required>
            <option value="">Select repo…</option>
            {repos.map((r) => (
              <option key={r.id} value={r.full_name}>
                {r.full_name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Deploy mode
          <select value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="push">push (CoGS builds UI)</option>
            <option value="pull">pull (Workers Builds UI)</option>
          </select>
        </label>
        <label>
          UI subdomain
          <input value={subdomain} onChange={(e) => setSubdomain(e.target.value)} placeholder="defaults to app id" />
        </label>
        <button type="submit" className="btn btn-primary" disabled={!gh?.signed_in}>
          Add app
        </button>
      </form>
    </div>
  );
}
