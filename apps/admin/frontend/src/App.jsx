import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import * as echarts from 'echarts';

const PERMS = [
  'metrics.read',
  'logs.read',
  'sql.read',
  'sql.write',
  'redeploy',
  'users.manage',
];

function panelFromHash() {
  const h = (window.location.hash || '#overview').slice(1);
  return h || 'overview';
}

async function api(path, opts = {}) {
  const r = await fetch(path, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  });
  if (r.status === 401) throw new Error('unauthorized');
  if (!r.ok) {
    const t = await r.text();
    throw new Error(t || r.statusText);
  }
  const ct = r.headers.get('content-type') || '';
  if (ct.includes('json')) return r.json();
  return r.text();
}

function Chart({ title, series, labels }) {
  const ref = useRef(null);
  useEffect(() => {
    if (!ref.current) return;
    const ch = echarts.init(ref.current, 'dark');
    ch.setOption({
      backgroundColor: 'transparent',
      grid: { left: 40, right: 12, top: 24, bottom: 28 },
      xAxis: { type: 'category', data: labels, axisLabel: { show: false } },
      yAxis: { type: 'value', splitLine: { lineStyle: { color: '#21262d' } } },
      series: series.map((s) => ({ type: 'line', smooth: true, showSymbol: false, ...s })),
      tooltip: { trigger: 'axis' },
    });
    const ro = new ResizeObserver(() => ch.resize());
    ro.observe(ref.current);
    return () => {
      ro.disconnect();
      ch.dispose();
    };
  }, [series, labels]);
  return (
    <div className="card">
      <h3>{title}</h3>
      <div className="chart" ref={ref} />
    </div>
  );
}

export default function App() {
  const [panel, setPanel] = useState(panelFromHash);
  const [me, setMe] = useState(null);
  const [loginErr, setLoginErr] = useState('');
  const [metricsHist, setMetricsHist] = useState([]);
  const [registry, setRegistry] = useState(null);
  const [users, setUsers] = useState([]);
  const [audit, setAudit] = useState([]);
  const [sqlDb, setSqlDb] = useState('');
  const [sqlApp, setSqlApp] = useState('');
  const [sqlText, setSqlText] = useState('SELECT 1');
  const [sqlResult, setSqlResult] = useState(null);
  const [tables, setTables] = useState([]);

  useEffect(() => {
    const onHash = () => setPanel(panelFromHash());
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  const go = (p) => {
    window.location.hash = p;
    setPanel(p);
  };

  const loadMe = useCallback(async () => {
    try {
      const u = await api('/api/me');
      setMe(u);
    } catch {
      setMe(null);
    }
  }, []);

  useEffect(() => {
    loadMe();
  }, [loadMe]);

  useEffect(() => {
    if (!me) return;
    let es;
    try {
      es = new EventSource('/api/metrics/stream');
      es.onmessage = (ev) => {
        try {
          const pt = JSON.parse(ev.data);
          setMetricsHist((h) => [...h.slice(-899), pt]);
        } catch {
          /* ignore */
        }
      };
    } catch {
      /* SSE unsupported */
    }
    return () => es?.close();
  }, [me]);

  useEffect(() => {
    if (!me) return;
    api('/api/registry').then(setRegistry).catch(() => {});
  }, [me]);

  useEffect(() => {
    if (panel !== 'people') return;
    if (!me?.is_root && !me.grants?.some((g) => g.permission === 'users.manage')) return;
    api('/api/users').then(setUsers).catch(() => {});
    if (me.is_root) api('/api/audit').then(setAudit).catch(() => {});
  }, [panel, me]);

  const chartLabels = useMemo(
    () => metricsHist.map((_, i) => String(i)),
    [metricsHist],
  );

  const cpuSeries = useMemo(
    () => [{ name: 'CPU %', data: metricsHist.map((m) => m.host?.cpu_percent ?? 0) }],
    [metricsHist],
  );

  const memSeries = useMemo(
    () => [{ name: 'Mem used MB', data: metricsHist.map((m) => m.host?.mem_used_mb ?? 0) }],
    [metricsHist],
  );

  const latest = metricsHist[metricsHist.length - 1];

  async function onLogin(e) {
    e.preventDefault();
    setLoginErr('');
    const fd = new FormData(e.target);
    try {
      await api('/api/login', {
        method: 'POST',
        body: JSON.stringify({
          username: fd.get('username'),
          password: fd.get('password'),
        }),
      });
      await loadMe();
    } catch (err) {
      setLoginErr(String(err.message || err));
    }
  }

  async function redeploy(app) {
    if (!confirm(`Redeploy ${app}? (git pull --ff-only + redeploy)`)) return;
    await api('/api/redeploy', { method: 'POST', body: JSON.stringify({ app }) });
    alert('Redeploy started/completed — check audit log');
  }

  async function runSQL(write) {
    const res = await api('/api/sql', {
      method: 'POST',
      body: JSON.stringify({ db_id: sqlDb, app: sqlApp, sql: sqlText, write }),
    });
    setSqlResult(res);
  }

  async function loadTables() {
    const t = await api(`/api/sql/tables?db_id=${encodeURIComponent(sqlDb)}&app=${encodeURIComponent(sqlApp)}`);
    setTables(t);
  }

  if (!me) {
    return (
      <div className="login-wrap">
        <form className="login-box" onSubmit={onLogin}>
          <h1>CoGS Admin</h1>
          {loginErr && <div className="err">{loginErr}</div>}
          <label>Username</label>
          <input name="username" autoComplete="username" required />
          <label>Password</label>
          <input name="password" type="password" autoComplete="current-password" required />
          <button type="submit">Sign in</button>
        </form>
      </div>
    );
  }

  return (
    <div className="shell">
      <nav className="nav">
        <button type="button" className={panel === 'overview' ? 'active' : ''} onClick={() => go('overview')}>
          Overview
        </button>
        <button type="button" className={panel === 'apps' ? 'active' : ''} onClick={() => go('apps')}>
          Apps
        </button>
        <button type="button" className={panel === 'sql' ? 'active' : ''} onClick={() => go('sql')}>
          SQL
        </button>
        {(me.is_root || me.grants?.some((g) => g.permission === 'users.manage')) && (
          <button type="button" className={panel === 'people' ? 'active' : ''} onClick={() => go('people')}>
            People
          </button>
        )}
        <div className="user">
          {me.username}
          {me.is_root ? ' · root' : ''}
          <br />
          <button
            type="button"
            style={{ marginTop: 8, fontSize: 12 }}
            onClick={() => api('/api/logout', { method: 'POST' }).then(() => setMe(null))}
          >
            Sign out
          </button>
        </div>
      </nav>
      <main className="main">
        {panel === 'overview' && (
          <>
            <div className="grid2">
              <Chart title="Host CPU" series={cpuSeries} labels={chartLabels} />
              <Chart title="Memory" series={memSeries} labels={chartLabels} />
            </div>
            <div className="card" style={{ marginTop: '1rem' }}>
              <h3>Snapshot</h3>
              <pre style={{ margin: 0, fontSize: 12 }}>
                {JSON.stringify(latest || {}, null, 2)}
              </pre>
            </div>
          </>
        )}
        {panel === 'apps' && (
          <div className="card">
            <h3>Workspace apps</h3>
            <table>
              <thead>
                <tr>
                  <th>App</th>
                  <th>Running</th>
                  <th>PID</th>
                  <th>RSS MB</th>
                  <th>DB bytes</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {(latest?.apps || []).map((a) => (
                  <tr key={a.name}>
                    <td>{a.name}</td>
                    <td>{a.running ? 'yes' : 'no'}</td>
                    <td>{a.pid || '—'}</td>
                    <td>{a.rss_mb?.toFixed?.(1) ?? '—'}</td>
                    <td>{a.db_bytes ?? '—'}</td>
                    <td className="actions">
                      {(me.is_root || me.grants?.some((g) => g.permission === 'redeploy' && (g.app_name === a.name || g.app_name === '*'))) && (
                        <button type="button" onClick={() => redeploy(a.name)}>
                          Redeploy
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {panel === 'sql' && (
          <div className="card sql-area">
            <h3>SQL console</h3>
            <p style={{ fontSize: 13, color: '#8b949e' }}>
              Registered databases only — paths never exposed to the browser.
            </p>
            <select
              value={sqlDb}
              onChange={(e) => {
                setSqlDb(e.target.value);
                const row = registry?.apps?.find((x) => x.id === e.target.value);
                setSqlApp(row?.app_name || e.target.value);
              }}
            >
              <option value="">Select database</option>
              {(registry?.apps || []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.app_name} ({d.path})
                </option>
              ))}
            </select>
            <button type="button" style={{ marginLeft: 8 }} onClick={loadTables}>
              List tables
            </button>
            {tables.length > 0 && (
              <p style={{ fontSize: 12 }}>{tables.join(', ')}</p>
            )}
            <textarea value={sqlText} onChange={(e) => setSqlText(e.target.value)} />
            <div className="actions" style={{ marginTop: 8 }}>
              <button type="button" onClick={() => runSQL(false)}>
                Run read
              </button>
              <button type="button" onClick={() => runSQL(true)}>
                Run write
              </button>
            </div>
            {sqlResult && (
              <pre style={{ fontSize: 12, marginTop: 12, overflow: 'auto' }}>
                {JSON.stringify(sqlResult, null, 2)}
              </pre>
            )}
          </div>
        )}
        {panel === 'people' && me.is_root && (
          <>
            <div className="card">
              <h3>Users</h3>
              <table>
                <thead>
                  <tr>
                    <th>User</th>
                    <th>Root</th>
                    <th>Grants</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => (
                    <tr key={u.id}>
                      <td>{u.username}</td>
                      <td>{u.is_root ? 'yes' : 'no'}</td>
                      <td>
                        {(u.grants || []).map((g) => `${g.app_name}:${g.permission}`).join(', ') || '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="card" style={{ marginTop: '1rem' }}>
              <h3>Audit</h3>
              <table>
                <thead>
                  <tr>
                    <th>When</th>
                    <th>Who</th>
                    <th>Action</th>
                    <th>Detail</th>
                  </tr>
                </thead>
                <tbody>
                  {audit.map((a) => (
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
            <CreateUserForm onCreated={() => api('/api/users').then(setUsers)} perms={PERMS} apps={registry?.apps || []} />
          </>
        )}
      </main>
    </div>
  );
}

function CreateUserForm({ onCreated, perms, apps }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [grants, setGrants] = useState([]);

  async function submit(e) {
    e.preventDefault();
    const { id } = await api('/api/users', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    if (grants.length) {
      await api(`/api/users/${id}/grants`, {
        method: 'PUT',
        body: JSON.stringify({ grants }),
      });
    }
    setUsername('');
    setPassword('');
    setGrants([]);
    onCreated();
  }

  function toggleGrant(app, perm) {
    const key = `${app}:${perm}`;
    setGrants((g) => {
      const has = g.some((x) => x.app_name === app && x.permission === perm);
      if (has) return g.filter((x) => !(x.app_name === app && x.permission === perm));
      return [...g, { app_name: app, permission: perm }];
    });
  }

  return (
    <form className="card" style={{ marginTop: '1rem' }} onSubmit={submit}>
      <h3>Create admin</h3>
      <input placeholder="Username" value={username} onChange={(e) => setUsername(e.target.value)} required />
      <input
        placeholder="Password (10+ chars)"
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        style={{ marginTop: 8, width: '100%' }}
      />
      <div style={{ marginTop: 12, fontSize: 12 }}>
        Grants (click to toggle):
        {apps.map((a) => (
          <div key={a.id}>
            <strong>{a.app_name}</strong>
            {perms.map((p) => (
              <button
                key={p}
                type="button"
                style={{ margin: 4, fontSize: 11 }}
                onClick={() => toggleGrant(a.app_name, p)}
              >
                {p}
              </button>
            ))}
          </div>
        ))}
      </div>
      <button type="submit" style={{ marginTop: 12 }}>
        Create
      </button>
    </form>
  );
}
