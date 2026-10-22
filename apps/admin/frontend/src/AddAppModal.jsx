import React, { useEffect, useRef, useState } from 'react';
import { CogsMark } from './CogsMark.jsx';

const REPO_PATTERN = /^[a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+$/;

export function AddAppModal({ open, onClose, api, onAdded, authStatus }) {
  const [repos, setRepos] = useState([]);
  const [reposLoading, setReposLoading] = useState(false);
  const [repoQuery, setRepoQuery] = useState('');
  const [name, setName] = useState('');
  const [repo, setRepo] = useState('');
  const [mode, setMode] = useState('push');
  const [subdomain, setSubdomain] = useState('');
  const [zone, setZone] = useState('');
  const [zones, setZones] = useState([]);
  const [hosts, setHosts] = useState({});
  const [preview, setPreview] = useState(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [advanced, setAdvanced] = useState(false);
  const [err, setErr] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);
  const touched = useRef({ name: false, mode: false, subdomain: false, zone: false });

  const gh = authStatus?.github;
  const cf = authStatus?.cloudflare;
  const ghReady = !!gh?.logged_in;
  const cfReady = !!cf?.logged_in;

  const canInstall = preview?.state === 'ready' && REPO_PATTERN.test(repo.trim());

  useEffect(() => {
    if (!open) return;
    setErr('');
    setDone(false);
    setRepoQuery('');
    setPreview(null);
    touched.current = { name: false, mode: false, subdomain: false, zone: false };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    if (!ghReady) return;
    setReposLoading(true);
    api('/api/repos')
      .then(setRepos)
      .catch(() => setRepos([]))
      .finally(() => setReposLoading(false));
  }, [open, ghReady, api]);

  useEffect(() => {
    if (!open) return;
    const q = repo.trim();
    if (!REPO_PATTERN.test(q)) {
      setPreview(null);
      setZones([]);
      setHosts({});
      return;
    }
    const t = setTimeout(() => {
      setPreviewLoading(true);
      const zq = zone.trim() ? `&zone=${encodeURIComponent(zone.trim())}` : '';
      api(`/api/repos/preview?q=${encodeURIComponent(q)}${zq}`)
        .then((data) => {
          setPreview(data.preview);
          setZones(data.zones || []);
          setHosts(data.hosts || {});
          const app = data.preview?.app;
          if (app && data.preview.state === 'ready') {
            if (!touched.current.name && app.name) setName(app.name);
            if (!touched.current.mode && app.deploy_mode) setMode(app.deploy_mode);
            if (!touched.current.subdomain && app.page_subdomain) setSubdomain(app.page_subdomain);
            if (!touched.current.zone && app.zone) setZone(app.zone);
          }
        })
        .catch((ex) => {
          setPreview({ state: 'error', message: String(ex.message || ex) });
        })
        .finally(() => setPreviewLoading(false));
    }, 350);
    return () => clearTimeout(t);
  }, [open, repo, zone, api]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e) => {
      if (e.key === 'Escape') onClose();
    };
    document.body.classList.add('modal-open');
    window.addEventListener('keydown', onKey);
    return () => {
      document.body.classList.remove('modal-open');
      window.removeEventListener('keydown', onKey);
    };
  }, [open, onClose]);

  const filteredRepos = ghReady
    ? repos.filter((r) => {
        const q = repoQuery.trim().toLowerCase();
        return !q || r.full_name?.toLowerCase().includes(q);
      })
    : repos;

  async function submit(e) {
    e.preventDefault();
    const repoFull = repo.trim();
    if (!REPO_PATTERN.test(repoFull)) {
      setErr('Enter repository as owner/name (e.g. irgipaulius/rampocalypse).');
      return;
    }
    if (!canInstall) {
      setErr(preview?.message || 'Fix repository issues before installing.');
      return;
    }
    setErr('');
    setSubmitting(true);
    try {
      await api('/api/apps', {
        method: 'POST',
        body: JSON.stringify({
          name: name.trim(),
          repo: repoFull,
          deploy_mode: mode,
          page_subdomain: (subdomain.trim() || name.trim()).toLowerCase(),
          zone: zone.trim(),
        }),
      });
      setDone(true);
      onAdded?.();
      setName('');
      setRepo('');
      setSubdomain('');
      setZone('');
      setMode('push');
      setPreview(null);
      setTimeout(() => {
        setDone(false);
        onClose();
      }, 2200);
    } catch (ex) {
      setErr(String(ex.message || ex));
    } finally {
      setSubmitting(false);
    }
  }

  function previewBanner() {
    if (previewLoading) {
      return (
        <div className="repo-preview repo-preview--loading">
          <CogsMark size={28} spin />
          <span>Reading cogs.app.toml…</span>
        </div>
      );
    }
    if (!preview || !REPO_PATTERN.test(repo.trim())) return null;
    const cls = `repo-preview repo-preview--${preview.state || 'unknown'}`;
    return (
      <div className={cls}>
        <p className="repo-preview-msg">{preview.message}</p>
        {preview.state === 'ready' && preview.app && (
          <ul className="repo-preview-facts">
            <li>
              App <strong>{preview.app.name}</strong>
              {preview.config_path ? ` · ${preview.config_path}` : ''}
            </li>
            <li>
              Deploy <strong>{preview.app.deploy_mode || mode}</strong>
              {preview.app.kind ? ` · ${preview.app.kind}` : ''}
            </li>
            {(hosts.page || preview.app.page_host) && (
              <li>
                UI{' '}
                <strong>{hosts.page || `https://${preview.app.page_host}`}</strong>
              </li>
            )}
            {(hosts.api || preview.app.api_host) && (
              <li>
                API{' '}
                <strong>{hosts.api || `https://${preview.app.api_host}`}</strong>
              </li>
            )}
          </ul>
        )}
        {preview.state === 'need_gh' && (
          <p className="footnote muted">
            SSH to the CoGS host and run <code>gh auth login</code> and{' '}
            <code>gh auth setup-git</code>, then refresh this page.
          </p>
        )}
      </div>
    );
  }

  if (!open) return null;

  return (
    <div className="modal-root" role="presentation" onClick={onClose}>
      <div
        className="modal-panel modal-panel--add-app"
        role="dialog"
        aria-modal="true"
        aria-labelledby="add-app-title"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="modal-header">
          <div>
            <p className="modal-eyebrow">Workspace</p>
            <h2 id="add-app-title">Add application</h2>
            <p className="modal-lead">
              Paste <strong>owner/repo</strong> — we fetch <code>cogs.app.toml</code> and prefill the
              rest. Public repos need no <code>gh</code>.
            </p>
          </div>
          <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>

        <div className="host-auth-strip">
          <span className={ghReady ? 'host-auth-ok' : 'host-auth-warn'}>
            GitHub: {ghReady ? gh.login || 'connected' : 'optional (public repos)'}
          </span>
          <span className={cfReady ? 'host-auth-ok' : 'host-auth-warn'}>
            Cloudflare: {cfReady ? cf.email || cf.account_name || 'connected' : 'needed at launch'}
          </span>
        </div>

        {err && <div className="err modal-err">{err}</div>}
        {done && (
          <div className="modal-success">
            App registered — <code>cogs launch</code> is running in the background on the server.
          </div>
        )}

        <form className="modal-form" onSubmit={submit}>
          <div className="form-field">
            <label htmlFor="add-app-repo-text">Repository</label>
            <input
              id="add-app-repo-text"
              value={repo}
              onChange={(e) => setRepo(e.target.value.trim())}
              placeholder="owner/my-cogs-app"
              pattern="[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+"
              required
              autoComplete="off"
              autoFocus
            />
            {ghReady && (
              <>
                <input
                  id="add-app-repo-search"
                  type="search"
                  placeholder="Filter your repositories…"
                  value={repoQuery}
                  onChange={(e) => setRepoQuery(e.target.value)}
                  autoComplete="off"
                />
                <select
                  id="add-app-repo"
                  value={repo}
                  onChange={(e) => setRepo(e.target.value)}
                  disabled={reposLoading}
                  size={Math.min(6, Math.max(2, filteredRepos.length || 2))}
                >
                  <option value="">{reposLoading ? 'Loading…' : 'Or pick from your repos'}</option>
                  {filteredRepos.map((r) => (
                    <option key={r.id || r.full_name} value={r.full_name}>
                      {r.full_name}
                      {r.private ? ' · private' : ''}
                      {r.has_cogs_config ? ` · ${r.cogs_hint || 'cogs'}` : ''}
                    </option>
                  ))}
                </select>
              </>
            )}
          </div>

          <div className="modal-pad">{previewBanner()}</div>

          {canInstall && (
            <>
              <div className="form-field">
                <label htmlFor="add-app-zone">Cloudflare zone</label>
                <span className="field-hint">
                  Each app can use a different apex domain — set in <code>cogs.app.toml</code> or pick
                  here (written on install).
                </span>
                {zones.length > 0 ? (
                  <select
                    id="add-app-zone"
                    value={zone}
                    onChange={(e) => {
                      touched.current.zone = true;
                      setZone(e.target.value);
                    }}
                  >
                    <option value="">From repo config</option>
                    {zones.map((z) => (
                      <option key={z} value={z}>
                        {z}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    id="add-app-zone"
                    value={zone}
                    onChange={(e) => {
                      touched.current.zone = true;
                      setZone(e.target.value.trim());
                    }}
                    placeholder={preview?.app?.zone || 'example.com'}
                    autoComplete="off"
                  />
                )}
                {!cfReady && (
                  <p className="footnote muted">
                    Run <code>wrangler login</code> on the host to list zones here; you can still type
                    the apex domain manually.
                  </p>
                )}
              </div>

              <button
                type="button"
                className="btn btn-ghost btn-sm advanced-toggle"
                onClick={() => setAdvanced((v) => !v)}
              >
                {advanced ? 'Hide' : 'Show'} advanced options
              </button>

              {advanced && (
                <>
                  <div className="form-field">
                    <label htmlFor="add-app-id">App id</label>
                    <input
                      id="add-app-id"
                      value={name}
                      onChange={(e) => {
                        touched.current.name = true;
                        setName(e.target.value);
                      }}
                      pattern="[a-z0-9][a-z0-9-]*"
                      autoComplete="off"
                    />
                  </div>
                  <fieldset className="form-field mode-picker">
                    <legend>Deploy mode</legend>
                    <div className="mode-options">
                      <label className={`mode-card${mode === 'push' ? ' selected' : ''}`}>
                        <input
                          type="radio"
                          name="deploy_mode"
                          value="push"
                          checked={mode === 'push'}
                          onChange={() => {
                            touched.current.mode = true;
                            setMode('push');
                          }}
                        />
                        <span className="mode-title">Push</span>
                        <span className="mode-desc">Host builds UI and deploys</span>
                      </label>
                      <label className={`mode-card${mode === 'pull' ? ' selected' : ''}`}>
                        <input
                          type="radio"
                          name="deploy_mode"
                          value="pull"
                          checked={mode === 'pull'}
                          onChange={() => {
                            touched.current.mode = true;
                            setMode('pull');
                          }}
                        />
                        <span className="mode-title">Pull</span>
                        <span className="mode-desc">Workers Builds ships UI</span>
                      </label>
                    </div>
                  </fieldset>
                  <div className="form-field">
                    <label htmlFor="add-app-sub">UI subdomain</label>
                    <input
                      id="add-app-sub"
                      value={subdomain}
                      onChange={(e) => {
                        touched.current.subdomain = true;
                        setSubdomain(e.target.value);
                      }}
                      placeholder={preview?.app?.page_subdomain || name}
                      autoComplete="off"
                    />
                  </div>
                </>
              )}
            </>
          )}

          <footer className="modal-footer">
            <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || done || !canInstall}
            >
              {submitting ? 'Installing…' : 'Install application'}
            </button>
          </footer>
        </form>
      </div>
    </div>
  );
}

export function AddAppTrigger({ onClick }) {
  return (
    <div className="apps-toolbar">
      <p className="muted apps-toolbar-hint">
        Install from GitHub — paste owner/repo; public repos work without <code>gh</code>.
      </p>
      <button type="button" className="btn btn-primary" onClick={onClick}>
        Add application
      </button>
    </div>
  );
}
