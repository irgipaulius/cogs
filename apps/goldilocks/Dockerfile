FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
COPY pkg ./pkg
COPY apps/goldilocks/go.mod apps/goldilocks/go.sum ./apps/goldilocks/

WORKDIR /src/apps/goldilocks
RUN go mod download
COPY apps/goldilocks/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/app ./cmd/app

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget python3 py3-pip py3-numpy netcdf-dev hdf5-dev g++ python3-dev \
	&& pip3 install --no-cache-dir --break-system-packages netCDF4 \
	&& apk del g++ python3-dev py3-pip \
	&& adduser -D -H -u 10001 cogs
WORKDIR /app
COPY --from=builder /out/app /usr/local/bin/cogs-goldilocks
COPY apps/goldilocks/docker/entrypoint.sh /entrypoint.sh
COPY apps/goldilocks/public /app/public
COPY apps/goldilocks/scripts /app/scripts
RUN chmod +x /entrypoint.sh \
	&& mkdir -p /data /config \
	&& chown -R cogs:cogs /data /config /app

USER cogs
ENV COGS_LISTEN=0.0.0.0:18788 \
	COGS_ALLOW_PUBLIC_BIND=1 \
	COGS_DB_PATH=/data/goldilocks.db \
	COGS_OBS=off \
	COGS_PUBLIC_DIR=/app/public \
	COGS_SCRAPER_SCRIPT=/app/scripts/scrape_chunk.py \
	COGS_WORKDIR=/app

EXPOSE 18788
VOLUME ["/data"]

ENTRYPOINT ["/entrypoint.sh"]
