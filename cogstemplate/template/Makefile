APP=app
MODULE={{.Module}}

.PHONY: build build-all test run sqlc migrate-new deploy rollback deps

build:
	@command -v cogs >/dev/null && cogs build || CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/$(APP) ./cmd/app

build-all:
	cogs build --target linux/amd64,linux/arm64,darwin/arm64,freebsd/amd64

test:
	go test ./...

run:
	go run ./cmd/app serve -env .env

sqlc:
	sqlc generate

migrate-new:
	@test -n "$(name)" || (echo "usage: make migrate-new name=add_foo"; exit 1)
	goose -dir internal/db/migrations create $(name) sql

deploy:
	cogs deploy $(HOST) --target $(TARGET)

rollback:
	cogs deploy $(HOST) --rollback

deps:
	cogs deps --target $(TARGET)
