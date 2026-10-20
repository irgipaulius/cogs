-- +goose Up
INSERT INTO todos (id, client_id, title, done, image_r2_key, updated_at, deleted) VALUES
('seed-001', 'cogs-bootstrap', 'Finish onboarding your CoGS todo stack', 0, NULL, '2026-01-01T00:00:00Z', 0),
('seed-002', 'cogs-bootstrap', 'Attach a photo (stored in R2 when configured)', 0, NULL, '2026-01-01T00:00:01Z', 0),
('seed-003', 'cogs-bootstrap', 'Toggle this item — syncs to SQLite on your Mac', 1, NULL, '2026-01-01T00:00:02Z', 0);

-- +goose Down
DELETE FROM todos WHERE id IN ('seed-001', 'seed-002', 'seed-003');
