PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS apps (
  id TEXT PRIMARY KEY,
  desired_present INTEGER NOT NULL DEFAULT 0,
  repo TEXT NOT NULL DEFAULT '',
  branch TEXT NOT NULL DEFAULT 'main',
  deploy_mode TEXT NOT NULL DEFAULT 'push',
  zone TEXT NOT NULL DEFAULT '',
  page_subdomain TEXT NOT NULL DEFAULT '',
  api_subdomain TEXT NOT NULL DEFAULT '',
  app_path TEXT NOT NULL DEFAULT '',
  git_root TEXT NOT NULL DEFAULT '',
  converged INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_owned (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  external_id TEXT NOT NULL DEFAULT '',
  meta_json TEXT NOT NULL DEFAULT '{}',
  UNIQUE(app_id, kind, external_id)
);

CREATE TABLE IF NOT EXISTS app_releases (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  commit_sha TEXT NOT NULL,
  built_at TEXT NOT NULL,
  binary_path TEXT NOT NULL DEFAULT '',
  health_ok INTEGER NOT NULL DEFAULT 0,
  UNIQUE(app_id, commit_sha)
);

CREATE TABLE IF NOT EXISTS app_release_pointers (
  app_id TEXT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
  current_release_id INTEGER REFERENCES app_releases(id),
  last_good_release_id INTEGER REFERENCES app_releases(id)
);

CREATE TABLE IF NOT EXISTS reconcile_runs (
  run_id TEXT PRIMARY KEY,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  current_stage TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  ended_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  log_path TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS reconcile_stages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id TEXT NOT NULL REFERENCES reconcile_runs(run_id) ON DELETE CASCADE,
  stage_name TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  ended_at TEXT NOT NULL DEFAULT '',
  output_tail TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS hostnames (
  hostname TEXT PRIMARY KEY,
  app_id TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  kind TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workspace_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  zone TEXT NOT NULL DEFAULT '',
  purpose TEXT NOT NULL DEFAULT 'dns',
  token TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reconcile_runs_app ON reconcile_runs(app_id, started_at DESC);
