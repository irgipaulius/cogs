-- +goose Up
CREATE TABLE todos (
    id TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    title TEXT NOT NULL,
    done INTEGER NOT NULL DEFAULT 0,
    image_r2_key TEXT,
    updated_at TEXT NOT NULL,
    deleted INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_todos_client ON todos(client_id);
CREATE INDEX idx_todos_updated ON todos(updated_at);

-- +goose Down
DROP TABLE IF EXISTS todos;
