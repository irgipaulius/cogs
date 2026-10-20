-- name: GetItem :one
SELECT id, val FROM items WHERE id = ?;

-- name: ListItems :many
SELECT id, val FROM items ORDER BY id LIMIT ?;
