import React, { useEffect, useMemo, useState } from 'react';

export function AddAppModal({ open, onClose, api, onAdded, authStatus }) {
  const [repos, setRepos] = useState([]);
  const [reposLoading, setReposLoading] = useState(false);
  const [repoQuery, setRepoQuery] = useState('');
  const [name, setName] = useState('');
  const [repo, setRepo] = useState('');
  const [mode, setMode] = useState('push');
  const [subdomain, setSubdomain] = useState('');
  const [err, setErr] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  const gh = authStatus?.github;
  const cf = authStatus?.cloudflare;
  const ghReady = !!gh?.logged_in;
  const cfReady = !!cf?.logged_in;

  useEffect(() => {
    if (!open) return;
    setErr('');
    setDone(false);
    setRepoQuery('');
  }, [open]);

  useEffect(() => {
    if (!open || !ghReady) {
      setRepos([]);
      return;
    }
    setReposLoading(true);
    api('/api/repos')
      .then(setRepos)
      .catch(() => setRepos([]))
      .finally(() => setReposLoading(false));
  }, [open, ghReady, api]);

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

  const filteredRepos = useMemo(() => {
    const q = repoQuery.trim().toLowerCase();
    if (!q) return repos;
    return repos.filter((r) => r.full_name?.toLowerCase().includes(q));
  }, [repos, repoQuery]);

  async function submit(e) {
    e.preventDefault();
    if (!ghReady) return;
    setErr('');
    setSubmitting(true);
    try {
      await api('/api/apps', {
        method: 'POST',
        body: JSON.stringify({
          name: name.trim(),
          repo,
          deploy_mode: mode,
          page_subdomain: (subdomain.trim() || name.trim()).toLowerCase(),
        }),
      });
      setDone(true);
      onAdded?.();
      setName('');
      setRepo('');
      setSubdomain('');
      setMode('push');
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

  if (!open) return null;

  return (
    <div className="modal-root" role="presentation" onClick={onClose}>
      <div
        className="modal-panel"
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
              Pick a GitHub repo with <code>cogs.app.toml</code>, register it, and run launch on the server.
            </p>
          </div>
          <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>

        <div className="host-auth-strip">
          <span className={ghReady ? 'host-auth-ok' : 'host-auth-warn'}>
            GitHub: {ghReady ? gh.login || 'connected' : gh?.installed ? 'not logged in' : 'gh not installed'}
          </span>
          <span className={cfReady ? 'host-auth-ok' : 'host-auth-warn'}>
            Cloudflare: {cfReady ? cf.email || cf.account_name || 'connected' : cf?.installed ? 'not logged in' : 'wrangler missing'}
          </span>
        </div>

        {err && <div className="err modal-err">{err}</div>}
        {done && (
          <div className="modal-success">
            App registered — <code>cogs launch</code> is running in the background on the server.
          </div>
        )}

        {!ghReady ? (
          <section className="modal-section gh-gate">
            <h3>Host GitHub login required</h3>
            <p className="muted">
              The admin dashboard uses the server&apos;s <code>gh</code> login (no OAuth app in the browser). SSH to
              the CoGS host and run:
            </p>
            <pre className="code-block">gh auth login{'\n'}gh auth setup-git</pre>
            <p className="footnote muted">Or run <code>scripts/bootstrap-host.sh</code> once on the server.</p>
          </section>
        ) : (
          <>
            <form className="modal-form" onSubmit={submit}>
              <div className="form-field">
                <label htmlFor="add-app-repo">Repository</label>
                <input
                  id="add-app-repo-search"
                  type="search"
                  placeholder="Filter repositories…"
                  value={repoQuery}
                  onChange={(e) => setRepoQuery(e.target.value)}
                  autoComplete="off"
                />
                <select
                  id="add-app-repo"
                  value={repo}
                  onChange={(e) => setRepo(e.target.value)}
                  required
                  disabled={reposLoading}
                  size={Math.min(8, Math.max(3, filteredRepos.length))}
                >
                  <option value="">{reposLoading ? 'Loading…' : 'Choose a repository'}</option>
                  {filteredRepos.map((r) => (
                    <option key={r.id} value={r.full_name}>
                      {r.full_name}
                      {r.has_cogs_config ? ` · ${r.cogs_hint || 'cogs'}` : ''}
                    </option>
                  ))}
                </select>
              </div>

              <div className="form-field">
                <label htmlFor="add-app-id">App id (optional)</label>
                <span className="field-hint">Defaults to name in cogs.app.toml</span>
                <input
                  id="add-app-id"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="from repo config"
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
                      onChange={() => setMode('push')}
                    />
                    <span className="mode-title">Push</span>
                    <span className="mode-desc">CoGS builds and deploys the UI from this server</span>
                  </label>
                  <label className={`mode-card${mode === 'pull' ? ' selected' : ''}`}>
                    <input
                      type="radio"
                      name="deploy_mode"
                      value="pull"
                      checked={mode === 'pull'}
                      onChange={() => setMode('pull')}
                    />
                    <span className="mode-title">Pull</span>
                    <span className="mode-desc">Workers Builds ships the UI; host deploys API on success</span>
                  </label>
                </div>
              </fieldset>

              {!cfReady && mode === 'pull' && (
                <p className="footnote muted">
                  Pull mode needs <code>wrangler login</code> on the host for Workers Builds setup during launch.
                </p>
              )}

              <div className="form-field">
                <label htmlFor="add-app-sub">UI subdomain</label>
                <span className="field-hint">Only when scaffolding; ignored if repo already has config</span>
                <input
                  id="add-app-sub"
                  value={subdomain}
                  onChange={(e) => setSubdomain(e.target.value)}
                  placeholder={name.trim() || 'from config'}
                  autoComplete="off"
                />
              </div>

              <footer className="modal-footer">
                <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={submitting || done || !repo}>
                  {submitting ? 'Installing…' : 'Install application'}
                </button>
              </footer>
            </form>
          </>
        )}
      </div>
    </div>
  );
}

export function AddAppTrigger({ onClick, authStatus }) {
  const ghReady = !!authStatus?.github?.logged_in;
  return (
    <div className="apps-toolbar">
      <p className="muted apps-toolbar-hint">
        {ghReady
          ? 'Install an app from a GitHub repository (host gh login).'
          : 'Run gh auth login on the server to enable repo install.'}
      </p>
      <button type="button" className="btn btn-primary" onClick={onClick}>
        Add application
      </button>
    </div>
  );
}
