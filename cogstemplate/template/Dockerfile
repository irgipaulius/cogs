# Optional: build static binary first (cogs build), then:
#   docker build -t myapp -f Dockerfile .
# For a full multi-stage example see github.com/irgipaulius/cogs/apps/todo/Dockerfile
FROM scratch
COPY dist/app /app
ENTRYPOINT ["/app", "serve", "-env", "/env/.env"]
