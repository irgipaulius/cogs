import React, { useEffect, useMemo, useState } from 'react';
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
    if (!open) return;
    if (ghReady) {
      setReposLoading(true);
      api('/api/repos')
        .then(setRepos)
        .catch(() => setRepos([]))
        .finally(() => setReposLoading(false));
      return;
    }
    const q = repo.trim();
    if (!REPO_PATTERN.test(q)) {
      setRepos([]);
      return;
    }
    const t = setTimeout(() => {
      setReposLoading(true);
      api(`/api/repos?q=${encodeURIComponent(q)}`)
        .then(setRepos)
        .catch(() => setRepos([]))
        .finally(() => setReposLoading(false));
    }, 400);
    return () => clearTimeout(t);
  }, [open, ghReady, api, repo]);

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
    if (!ghReady) return repos;
    const q = repoQuery.trim().toLowerCase();
    if (!q) return repos;
    return repos.filter((r) => r.full_name?.toLowerCase().includes(q));
  }, [repos, repoQuery, ghReady]);

  async function submit(e) {
    e.preventDefault();
    const repoFull = repo.trim();
    if (!REPO_PATTERN.test(repoFull)) {
      setErr('Enter repository as owner/name (e.g. irgipaulius/cogs).');
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
              Public GitHub repos install without <code>gh</code>. Private repos need{' '}
              <code>gh auth login</code> on the server.
            </p>
          </div>
          <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>

        <div className="host-auth-strip">
          <span className={ghReady ? 'host-auth-ok' : 'host-auth-warn'}>
            GitHub:{' '}
            {ghReady
              ? gh.login || 'connected (private + public)'
              : 'optional — public repos only'}
          </span>
          <span className={cfReady ? 'host-auth-ok' : 'host-auth-warn'}>
            Cloudflare: {cfReady ? cf.email || cf.account_name || 'connected' : cf?.installed ? 'needed at launch' : 'wrangler missing'}
          </span>
        </div>

        {!ghReady && (
          <p className="footnote muted modal-pad">
            Enter <strong>owner/repo</strong> below. For private repositories, SSH to the host and run{' '}
            <code>gh auth login</code> — then refresh this page to browse your repos.
          </p>
        )}

        {err && <div className="err modal-err">{err}</div>}
        {done && (
          <div className="modal-success">
            App registered — <code>cogs launch</code> is running in the background on the server.
          </div>
        )}

        {reposLoading && ghReady ? (
          <div className="modal-pad session-loading">
            <CogsMark size={40} spin label="Loading repositories…" />
          </div>
        ) : null}

        <form className="modal-form" onSubmit={submit}>
          <div className="form-field">
            <label htmlFor="add-app-repo-text">Repository</label>
            <span className="field-hint">GitHub owner/name — public repos work without gh login</span>
            <input
              id="add-app-repo-text"
              value={repo}
              onChange={(e) => setRepo(e.target.value.trim())}
              placeholder="irgipaulius/cogs"
              pattern="[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+"
              required
              autoComplete="off"
            />
            {ghReady ? (
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
                  size={Math.min(8, Math.max(3, filteredRepos.length || 3))}
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
            ) : null}
            {!ghReady && reposLoading && REPO_PATTERN.test(repo.trim()) ? (
              <p className="footnote muted">Checking public repository…</p>
            ) : null}
            {!ghReady && !reposLoading && REPO_PATTERN.test(repo.trim()) && repos.length === 1 ? (
              <p className="footnote muted">Public repo found — ready to install.</p>
            ) : null}
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
              Pull mode needs <code>wrangler login</code> on the host during launch (and <code>gh</code> for private
              repo builds).
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
            <button type="submit" className="btn btn-primary" disabled={submitting || done || !repo.trim()}>
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
        Install from a public GitHub repo (owner/name) or sign in with gh on the server for private repos.
      </p>
      <button type="button" className="btn btn-primary" onClick={onClick}>
        Add application
      </button>
    </div>
  );
}
