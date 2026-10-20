# Optional runtime image for the todo API (SQLite + Go).
# Build from repo root: docker build -f apps/todo/Dockerfile -t cogs-todo .
# Default deploy remains: ./cogs launch todo / cogs deploy

FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
COPY pkg ./pkg
COPY apps/todo/go.mod apps/todo/go.sum ./apps/todo/

WORKDIR /src/apps/todo
RUN go mod download
COPY apps/todo/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/app

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
	&& adduser -D -H -u 10001 cogs
WORKDIR /app
COPY --from=builder /out/app /usr/local/bin/cogs-todo
COPY apps/todo/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh \
	&& mkdir -p /data /config \
	&& chown -R cogs:cogs /data /config /app

USER cogs
ENV COGS_LISTEN=0.0.0.0:18787 \
	COGS_ALLOW_PUBLIC_BIND=1 \
	COGS_DB_PATH=/data/todo.db \
	COGS_OBS=off

EXPOSE 18787
VOLUME ["/data"]

ENTRYPOINT ["/entrypoint.sh"]
