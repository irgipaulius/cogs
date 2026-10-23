import React, { useEffect, useMemo, useState } from 'react';
import { GeoMap } from './GeoMap.jsx';

const APP_TABS = [
  { id: 'hosting', label: 'Hosting' },
  { id: 'traffic', label: 'Traffic' },
  { id: 'logs', label: 'Logs' },
  { id: 'sql', label: 'SQL' },
  { id: 'geo', label: 'Geo' },
];

export function filterStreamLines(lines, filter, hideInternal, methodFilter) {
  const q = (filter || '').trim().toLowerCase();
  const mf = (methodFilter || '').trim().toUpperCase();
  return (lines || []).filter((line) => {
    if (hideInternal && line.internal) return false;
    if (mf && typeof line === 'object' && (line.method || '').toUpperCase() !== mf) return false;
    if (!q) return true;
    const hay = typeof line === 'string' ? line : JSON.stringify(line);
    return hay.toLowerCase().includes(q);
  });
}

export function StreamControls({ paused, onTogglePause, filter, onFilter, hideInternal, onToggleInternal, extra }) {
  return (
    <div className="stream-controls">
      <input
        className="stream-filter"
        type="search"
        placeholder="Filter…"
        value={filter}
        onChange={(e) => onFilter(e.target.value)}
      />
      <button type="button" className={`btn${paused ? ' btn-primary' : ''}`} onClick={onTogglePause}>
        {paused ? 'Resume live' : 'Pause'}
      </button>
      {onToggleInternal && (
        <button type="button" className={`btn chip${hideInternal ? ' selected' : ''}`} onClick={onToggleInternal}>
          Hide internal
        </button>
      )}
      {extra}
    </div>
  );
}

export function SqlResults({ result, error }) {
  if (error) {
    return <div className="err">{error}</div>;
  }
  if (!result) {
    return <p className="muted">Run a query to see results.</p>;
  }
  if (result.rows_affected != null && !result.columns?.length) {
    return <p className="muted">OK — {result.rows_affected} row(s) affected.</p>;
  }
  const cols = result.columns || [];
  const rows = result.rows || [];
  if (!cols.length) {
    return <p className="muted">Query returned no columns.</p>;
  }
  return (
    <div className="table-wrap sql-results">
      <table>
        <thead>
          <tr>
            {cols.map((c) => (
              <th key={c}>{c}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 && (
            <tr>
              <td colSpan={cols.length} className="muted">
                0 rows
              </td>
            </tr>
          )}
          {rows.map((row, i) => (
            <tr key={i}>
              {row.map((cell, j) => (
                <td key={j}>{cell == null ? 'NULL' : String(cell)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatRequestLine(line) {
  const t = line.at ? new Date(line.at).toLocaleTimeString() : '';
  const cc = line.country ? ` [${line.country}]` : '';
  const internal = line.internal ? ' (internal)' : '';
  const body = line.body ? ` | ${line.body}` : '';
  return `${t} ${line.ip}${cc}${internal} ${line.method} ${line.path} → ${line.status} (${line.ms}ms)${body}`;
}

export function AppDetailView({
  app,
  tab,
  go,
  appMeta,
  appMetrics,
  api,
  me,
  redeploy,
  rollback,
  deployEvents,
  provision,
  sqlText,
  setSqlText,
  tables,
  loadTables,
  runSQL,
  sqlResult,
  sqlError,
  sqlLoading,
}) {
  const [traffic, setTraffic] = useState(null);
  const [trafficPaused, setTrafficPaused] = useState(false);
  const [trafficFilter, setTrafficFilter] = useState('');
  const [hideInternal, setHideInternal] = useState(true);
  const [logs, setLogs] = useState([]);
  const [logsPaused, setLogsPaused] = useState(false);
  const [logsFilter, setLogsFilter] = useState('');
  const [logsLoading, setLogsLoading] = useState(false);
  const [logKind, setLogKind] = useState('access');
  const [trafficMethod, setTrafficMethod] = useState('');
  const [geoMode, setGeoMode] = useState('all');

  useEffect(() => {
    if (tab !== 'traffic' && tab !== 'geo') return;
    let cancelled = false;
    const load = () => {
      if (trafficPaused) return;
      api(`/api/apps/${encodeURIComponent(app)}/telemetry`)
        .then((snap) => {
          if (!cancelled) setTraffic(snap);
        })
        .catch(() => {});
    };
    load();
    const t = setInterval(load, 2000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [app, tab, trafficPaused, api]);

  useEffect(() => {
    if (tab !== 'logs') return;
    let cancelled = false;
    const load = () => {
      if (logsPaused) return;
      setLogsLoading(true);
      api(`/api/apps/${encodeURIComponent(app)}/logs?kind=${encodeURIComponent(logKind)}`)
        .then((lines) => {
          if (!cancelled) setLogs(lines);
        })
        .catch(() => {
          if (!cancelled) setLogs([]);
        })
        .finally(() => {
          if (!cancelled) setLogsLoading(false);
        });
    };
    load();
    const t = setInterval(load, 2000);
    return () => {
      cancelled = true;
      clearInterval(t);
    };
  }, [app, tab, logsPaused, logKind, api]);

  const trafficLines = useMemo(() => {
    const recent = traffic?.recent || [];
    return filterStreamLines(recent, trafficFilter, hideInternal, trafficMethod);
  }, [traffic, trafficFilter, hideInternal, trafficMethod]);

  const logLines = useMemo(() => filterStreamLines(logs, logsFilter, false), [logs, logsFilter]);

  const meta = appMeta(app);
  const metrics = appMetrics(app);
  const canRedeploy =
    me.is_root ||
    me.grants?.some((g) => g.permission === 'redeploy' && (g.app_name === app || g.app_name === '*'));

  return (
    <div className="app-detail stack">
      <div className="page-head app-detail-head">
        <button type="button" className="back" onClick={() => go('apps')}>
          ← Apps
        </button>
        <h2>{app}</h2>
      </div>
      <div className="subnav" role="tablist">
        {APP_TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            className={tab === t.id ? 'active' : ''}
            onClick={() => go(`app/${app}/${t.id}`)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'hosting' && (
        <div className="card">
          <h3>Hosting</h3>
          <dl className="deploy-dl">
            <dt>Deploy mode</dt>
            <dd>{meta.deploy_mode || 'push'}</dd>
            <dt>Commit</dt>
            <dd>{meta.git_commit || '—'}</dd>
            <dt>Last status</dt>
            <dd>{meta.last_status || '—'}</dd>
            {meta.last_error && (
              <>
                <dt>Last error</dt>
                <dd className="err">{meta.last_error}</dd>
              </>
            )}
          </dl>
          {meta.deploy_mode === 'pull' && (
            <p className="muted">
              Pull mode: Cloudflare builds the UI first; keep API changes backward compatible until the backend deploy finishes.
            </p>
          )}
          <pre className="meta">
            {JSON.stringify({ ...meta, runtime: metrics }, null, 2)}
          </pre>
          {(deployEvents || [])
            .filter((e) => e.app === app)
            .slice(0, 8)
            .map((e, i) => (
              <div key={i} className="deploy-line muted">
                {e.at} · {e.action} · {e.status || '—'} · {e.git_commit || ''}{' '}
                {e.detail ? `— ${e.detail.slice(0, 120)}` : ''}
              </div>
            ))}
          {provision && !provision.ready && (
            <div className="modal-err">
              <strong>Not provisioned</strong>
              <ul>
                {(provision.reasons || []).map((r) => (
                  <li key={r}>{r}</li>
                ))}
              </ul>
            </div>
          )}
          {me?.is_root && (
            <div className="btn-row">
              {provision && !provision.ready && (
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={() =>
                    api(`/api/apps/${encodeURIComponent(app)}/reconcile`, { method: 'POST' }).catch((ex) =>
                      alert(ex.message),
                    )
                  }
                >
                  Reconcile
                </button>
              )}
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => {
                  if (!window.confirm(`Remove ${app} from CoGS?`)) return;
                  api(`/api/apps/${encodeURIComponent(app)}/remove`, { method: 'POST' })
                    .then(() => go('apps'))
                    .catch((ex) => alert(ex.message));
                }}
              >
                Remove app
              </button>
            </div>
          )}
          {canRedeploy && (
            <div className="btn-row">
              <button type="button" className="btn btn-primary" onClick={() => redeploy(app)}>
                Redeploy
              </button>
              <button type="button" className="btn" onClick={() => rollback(app)}>
                Rollback
              </button>
            </div>
          )}
        </div>
      )}

      {tab === 'traffic' && (
        <div className="card">
          <h3>Live traffic</h3>
          <StreamControls
            paused={trafficPaused}
            onTogglePause={() => setTrafficPaused((p) => !p)}
            filter={trafficFilter}
            onFilter={setTrafficFilter}
            hideInternal={hideInternal}
            onToggleInternal={() => setHideInternal((h) => !h)}
            extra={
              <>
                {['', 'GET', 'POST', 'PATCH', 'DELETE'].map((m) => (
                  <button
                    key={m || 'all'}
                    type="button"
                    className={`btn chip${trafficMethod === m ? ' selected' : ''}`}
                    onClick={() => setTrafficMethod(m)}
                  >
                    {m || 'All methods'}
                  </button>
                ))}
                {trafficPaused ? (
                  <span className="muted live-badge paused">Paused</span>
                ) : (
                  <span className="muted live-badge">Live · 2s</span>
                )}
              </>
            }
          />
          <pre className="meta meta-compact">
            {JSON.stringify(
              {
                requests_total: traffic?.requests_total,
                in_flight: traffic?.in_flight,
                peak_in_flight: traffic?.peak_in_flight,
                unique_ips: traffic?.unique_ips_session,
                updated_at: traffic?.updated_at,
              },
              null,
              2,
            )}
          </pre>
          <h4 className="subhead">Recent requests</h4>
          <div className="log-box tall">
            {trafficLines.length === 0 && <div className="log-line muted">No matching requests.</div>}
            {trafficLines.map((line, i) => (
              <div key={i} className={`log-line${line.internal ? ' internal' : ''}`}>
                {formatRequestLine(line)}
              </div>
            ))}
          </div>
          <p className="muted footnote">
            Public clients use Cloudflare headers (CF-Connecting-IP, CF-IPCountry). Health checks from Docker show as
            internal.
          </p>
        </div>
      )}

      {tab === 'logs' && (
        <div className="card">
          <h3>Logs</h3>
          <div className="stream-controls">
            <button
              type="button"
              className={`btn chip${logKind === 'access' ? ' selected' : ''}`}
              onClick={() => setLogKind('access')}
            >
              HTTP access
            </button>
            <button
              type="button"
              className={`btn chip${logKind === 'process' ? ' selected' : ''}`}
              onClick={() => setLogKind('process')}
            >
              Process
            </button>
          </div>
          <StreamControls
            paused={logsPaused}
            onTogglePause={() => setLogsPaused((p) => !p)}
            filter={logsFilter}
            onFilter={setLogsFilter}
            extra={
              logsPaused ? (
                <span className="muted live-badge paused">Paused</span>
              ) : (
                <span className="muted live-badge">Live · 2s</span>
              )
            }
          />
          {logKind === 'access' && (
            <p className="muted footnote">Every API request (except health checks). Search for POST, /api/todos, etc.</p>
          )}
          <div className="log-box tall">
            {logsLoading && logLines.length === 0 && <div className="log-line muted">Loading…</div>}
            {!logsLoading && logLines.length === 0 && <div className="log-line muted">No matching log lines.</div>}
            {logLines.map((line, i) => (
              <div key={i} className="log-line">
                {line}
              </div>
            ))}
          </div>
        </div>
      )}

      {tab === 'sql' && (
        <div className="card sql-area">
          <h3>SQL</h3>
          <button type="button" className="btn" onClick={() => loadTables(app)}>
            List tables
          </button>
          {tables.length > 0 && <p className="muted">{tables.join(', ')}</p>}
          <textarea value={sqlText} onChange={(e) => setSqlText(e.target.value)} />
          <div className="actions">
            <button type="button" className="btn" disabled={sqlLoading} onClick={() => runSQL(app, false)}>
              Run read
            </button>
            <button type="button" className="btn" disabled={sqlLoading} onClick={() => runSQL(app, true)}>
              Run write
            </button>
          </div>
          <SqlResults result={sqlResult} error={sqlError} />
        </div>
      )}

      {tab === 'geo' && (
        <div className="card">
          <h3>Geography</h3>
          <p className="muted">
            Country = ISO alpha-2 from Cloudflare <code>CF-IPCountry</code> (same codes as the map and
            table). Reads vs writes split by HTTP method.
          </p>
          <div className="stream-controls">
            {[
              { id: 'read', label: 'Incoming (reads)' },
              { id: 'write', label: 'Outgoing (writes)' },
              { id: 'all', label: 'All' },
            ].map((m) => (
              <button
                key={m.id}
                type="button"
                className={`btn chip${geoMode === m.id ? ' selected' : ''}`}
                onClick={() => setGeoMode(m.id)}
              >
                {m.label}
              </button>
            ))}
            <button type="button" className={`btn${trafficPaused ? ' btn-primary' : ''}`} onClick={() => setTrafficPaused((p) => !p)}>
              {trafficPaused ? 'Resume' : 'Pause'}
            </button>
          </div>
          <GeoMap
            statsRead={traffic?.by_country_read}
            statsWrite={traffic?.by_country_write}
            mode={geoMode}
          />
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Country</th>
                  <th>Reads</th>
                  <th>Writes</th>
                </tr>
              </thead>
              <tbody>
                {[
                  ...new Set([
                    ...Object.keys(traffic?.by_country_read || {}),
                    ...Object.keys(traffic?.by_country_write || {}),
                  ]),
                ]
                  .sort()
                  .map((cc) => (
                    <tr key={cc}>
                      <td>{cc}</td>
                      <td>{traffic?.by_country_read?.[cc] ?? 0}</td>
                      <td>{traffic?.by_country_write?.[cc] ?? 0}</td>
                    </tr>
                  ))}
                {!Object.keys(traffic?.by_country || {}).length && (
                  <tr>
                    <td colSpan={3} className="muted">
                      No country data yet — use the public URL through Cloudflare.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
