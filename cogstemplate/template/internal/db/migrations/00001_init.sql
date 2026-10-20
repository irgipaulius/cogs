-- +goose Up
CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    val TEXT NOT NULL,
    blob BLOB,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS items;
