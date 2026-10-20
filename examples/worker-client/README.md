# worker-client example

Worker bound to a **Workers VPC** service that calls your private CoGS origin.

```bash
cogs tunnel private --name cogs-demo --port 8080
# create vpc service + note service_id
# set vpc_services in wrangler.toml
npm install
npx wrangler deploy
```

The Worker sends `X-Cogs-Private-Token` matching `COGS_PRIVATE_TOKEN` on the server.
