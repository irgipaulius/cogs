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
import { AddAppModal, AddAppTrigger } from './AddAppModal.jsx';
import { CogsMark, CogsLogo } from './CogsMark.jsx';

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
  const ct = r.headers.get('content-type') || '';
  const isJSON = ct.includes('json');
  const body = isJSON ? await r.json().catch(() => null) : await r.text();
  if (r.status === 401) throw new Error('Session expired — sign in again.');
  if (!r.ok) {
    if (body && typeof body === 'object' && body.error) {
      throw new Error(body.error);
    }
    if (typeof body === 'string' && body.includes('<!DOCTYPE html>')) {
      throw new Error(
        'Admin API unreachable (502). Check cogs-admin is running and the tunnel is healthy.',
      );
    }
    const msg = typeof body === 'string' ? body.trim().slice(0, 400) : JSON.stringify(body);
    throw new Error(msg || `Request failed (${r.status})`);
  }
  return body ?? {};
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
  const [sessionBooting, setSessionBooting] = useState(true);
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
  const [addAppOpen, setAddAppOpen] = useState(false);
  const [authStatus, setAuthStatus] = useState(null);

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
    } finally {
      setSessionBooting(false);
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
    if (!me?.is_root) return;
    api('/api/auth/status')
      .then(setAuthStatus)
      .catch(() => setAuthStatus(null));
  }, [me, addAppOpen]);

  useEffect(() => {
    if (panel !== 'people' || !me) return;
    if (!me.is_root && !me.grants?.some((g) => g.permission === 'users.manage')) return;
    api('/api/users')
      .then((data) => setUsers(Array.isArray(data) ? data : []))
      .catch(() => setUsers([]));
    if (me.is_root) {
      api('/api/audit')
        .then((data) => setAudit(Array.isArray(data) ? data : []))
        .catch(() => setAudit([]));
    }
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

  if (sessionBooting) {
    return (
      <div className="login-wrap session-loading">
        <CogsMark size={56} spin label="Loading session…" />
      </div>
    );
  }

  if (!me) {
    return (
      <div className="login-wrap">
        <form className="login-box" onSubmit={onLogin}>
          <div className="login-brand">
            <CogsLogo className="login-logo" />
          </div>
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
      <div className="brand-cell hide-mobile">
        <CogsLogo className="brand-logo" />
      </div>
      <header className="topbar">
        <CogsLogo className="brand-logo brand-logo--mobile hide-desktop" />
        <div className="topbar-spacer hide-mobile" aria-hidden="true" />
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
            {me.is_root && (
              <>
                <AddAppTrigger onClick={() => setAddAppOpen(true)} />
                <AddAppModal
                  open={addAppOpen}
                  onClose={() => setAddAppOpen(false)}
                  api={api}
                  authStatus={authStatus}
                  onAdded={() => {
                    api('/api/auth/status').then(setAuthStatus);
                    api('/api/metrics/latest').then(setBoard);
                  }}
                />
              </>
            )}
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
                    {(Array.isArray(users) ? users : []).map((u) => (
                      <tr key={u.id}>
                        <td>{u.username}</td>
                        <td>{u.is_root ? 'yes' : 'no'}</td>
                        <td>
                          {(Array.isArray(u.grants) ? u.grants : [])
                            .map((g) => `${g.app_name}:${g.permission}`)
                            .join(', ') || '—'}
                        </td>
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
                      {(Array.isArray(audit) ? audit : []).map((a) => (
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
              onCreated={() =>
                api('/api/users').then((data) => setUsers(Array.isArray(data) ? data : []))
              }
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

