import React, { useCallback, useEffect, useState } from 'react';

export function AddAppModal({ open, onClose, api, onAdded }) {
  const [gh, setGh] = useState(null);
  const [ghLoading, setGhLoading] = useState(true);
  const [flow, setFlow] = useState(null);
  const [repos, setRepos] = useState([]);
  const [reposLoading, setReposLoading] = useState(false);
  const [name, setName] = useState('');
  const [repo, setRepo] = useState('');
  const [mode, setMode] = useState('push');
  const [subdomain, setSubdomain] = useState('');
  const [err, setErr] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [done, setDone] = useState(false);

  const refreshGh = useCallback(() => {
    setGhLoading(true);
    return api('/api/github/status')
      .then((st) => setGh(st))
      .catch(() => setGh({ signed_in: false }))
      .finally(() => setGhLoading(false));
  }, [api]);

  useEffect(() => {
    if (!open) return;
    setErr('');
    setDone(false);
    setFlow(null);
    refreshGh();
  }, [open, refreshGh]);

  useEffect(() => {
    if (!open || !gh?.signed_in) {
      setRepos([]);
      return;
    }
    setReposLoading(true);
    api('/api/github/repos')
      .then(setRepos)
      .catch(() => setRepos([]))
      .finally(() => setReposLoading(false));
  }, [open, gh?.signed_in, api]);

  useEffect(() => {
    if (!open || !flow?.device_code) return;
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
        /* pending */
      }
    }, (flow.interval || 5) * 1000);
    return () => clearInterval(t);
  }, [open, flow, api]);

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

  async function startLogin() {
    setErr('');
    try {
      const start = await api('/api/github/login/start', { method: 'POST', body: '{}' });
      setFlow(start);
    } catch (ex) {
      setErr(String(ex.message || ex));
    }
  }

  async function submit(e) {
    e.preventDefault();
    if (!gh?.signed_in) return;
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
      }, 1200);
    } catch (ex) {
      setErr(String(ex.message || ex));
    } finally {
      setSubmitting(false);
    }
  }

  if (!open) return null;

  const signedIn = !!gh?.signed_in;

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
              Scaffold a new app from GitHub, register it in the workspace, and run launch.
            </p>
          </div>
          <button type="button" className="modal-close" onClick={onClose} aria-label="Close">
            ×
          </button>
        </header>

        {err && <div className="err modal-err">{err}</div>}
        {done && <div className="modal-success">App added — launch is running in the background.</div>}

        {ghLoading ? (
          <p className="muted modal-pad">Checking GitHub connection…</p>
        ) : !signedIn ? (
          <section className="modal-section gh-gate">
            <div className="gh-gate-icon" aria-hidden>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
              </svg>
            </div>
            <h3>Connect GitHub first</h3>
            <p className="muted">
              CoGS needs repository access to scaffold <code>cogs.app.toml</code>, update{' '}
              <code>go.work</code>, and configure deploy mode. Sign in once — then you can add apps.
            </p>
            {flow ? (
              <div className="device-flow">
                <p className="device-flow-label">Enter this code on GitHub</p>
                <p className="device-code">{flow.user_code}</p>
                <a
                  className="btn btn-primary"
                  href={flow.verification_uri}
                  target="_blank"
                  rel="noreferrer"
                >
                  Open github.com/login/device
                </a>
                <p className="footnote muted">Waiting for authorization…</p>
              </div>
            ) : (
              <button type="button" className="btn btn-primary btn-wide" onClick={startLogin}>
                Sign in with GitHub
              </button>
            )}
          </section>
        ) : (
          <>
            <div className="gh-connected">
              <span className="gh-connected-dot" aria-hidden />
              Connected as <strong>{gh.login || 'GitHub user'}</strong>
            </div>
            <form className="modal-form" onSubmit={submit}>
              <div className="form-field">
                <label htmlFor="add-app-id">App id</label>
                <span className="field-hint">Workspace name — lowercase, no spaces</span>
                <input
                  id="add-app-id"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="my-app"
                  pattern="[a-z0-9][a-z0-9-]*"
                  required
                  autoComplete="off"
                />
              </div>

              <div className="form-field">
                <label htmlFor="add-app-repo">Repository</label>
                <span className="field-hint">Must exist in your GitHub account</span>
                <select
                  id="add-app-repo"
                  value={repo}
                  onChange={(e) => setRepo(e.target.value)}
                  required
                  disabled={reposLoading}
                >
                  <option value="">{reposLoading ? 'Loading repositories…' : 'Choose a repository'}</option>
                  {repos.map((r) => (
                    <option key={r.id} value={r.full_name}>
                      {r.full_name}
                    </option>
                  ))}
                </select>
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
                    <span className="mode-desc">Cloudflare Workers Builds ships the UI; host deploys API on success</span>
                  </label>
                </div>
              </fieldset>

              <div className="form-field">
                <label htmlFor="add-app-sub">UI subdomain</label>
                <span className="field-hint">Host prefix on your zone — defaults to app id</span>
                <input
                  id="add-app-sub"
                  value={subdomain}
                  onChange={(e) => setSubdomain(e.target.value)}
                  placeholder={name.trim() || 'same as app id'}
                  autoComplete="off"
                />
              </div>

              <footer className="modal-footer">
                <button type="button" className="btn btn-ghost" onClick={onClose} disabled={submitting}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={submitting || done}>
                  {submitting ? 'Adding…' : 'Add application'}
                </button>
              </footer>
            </form>
          </>
        )}
      </div>
    </div>
  );
}

export function AddAppTrigger({ onClick, githubSignedIn }) {
  return (
    <div className="apps-toolbar">
      <p className="muted apps-toolbar-hint">
        {githubSignedIn
          ? 'Register a new app from a GitHub repository.'
          : 'Connect GitHub to register new apps from a repository.'}
      </p>
      <button type="button" className="btn btn-primary" onClick={onClick}>
        Add application
      </button>
    </div>
  );
}
