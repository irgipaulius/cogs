# Admin dashboard + cogs supervisor (workspace control plane).
# Build from repo root: docker build -f apps/admin/Dockerfile -t cogs-admin-stack .

FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
COPY pkg ./pkg
COPY internal ./internal
COPY cogstemplate ./cogstemplate
COPY cmd/cogs ./cmd/cogs

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/cogs ./cmd/cogs

COPY apps/admin/go.mod apps/admin/go.sum ./apps/admin/
WORKDIR /src/apps/admin
RUN go mod download
COPY apps/admin/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/cogs-admin ./cmd/app

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
	&& adduser -D -H -u 10001 cogs
WORKDIR /app
COPY --from=builder /out/cogs /usr/local/bin/cogs
COPY --from=builder /out/cogs-admin /usr/local/bin/cogs-admin
COPY apps/admin/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh \
	&& mkdir -p /workspace /config \
	&& chown -R cogs:cogs /workspace /config /app

USER cogs
ENV COGS_LISTEN=0.0.0.0:18786 \
	COGS_ALLOW_PUBLIC_BIND=1 \
	COGS_WORKSPACE_ROOT=/workspace \
	COGS_ADMIN_DB_PATH=/workspace/apps/admin/data/admin.db \
	COGS_ADMIN_DEV=1 \
	COGS_PUBLIC_PAGE_ORIGIN=http://127.0.0.1:18786 \
	COGS_PUBLIC_API_URL=http://127.0.0.1:18786

EXPOSE 18786
VOLUME ["/workspace"]

ENTRYPOINT ["/entrypoint.sh"]
