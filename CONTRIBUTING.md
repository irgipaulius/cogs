# Contributing

1. Fork and branch from `main`.
2. `go test ./...` and `go vet ./...` must pass.
3. Keep SQL in goose migrations and sqlc query files; avoid ad-hoc raw SQL in handlers.
4. Document Cloudflare steps in `docs/` when behavior changes.
5. Open a PR with a short description and test plan.
