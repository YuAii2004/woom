# Production Deployment

This deployment runs WOOM, Redis, Live777, and Caddy on one Docker host. Caddy terminates HTTPS and forwards traffic to the WOOM service.

## Prerequisites

- A Linux cloud server with a public IPv4 address
- Docker Engine with Docker Compose
- A DNS A record pointing the meeting domain to the server
- Firewall access for TCP ports 80 and 443

## Configure

```bash
cp .env.production.example .env.production
```

Set `WOOM_DOMAIN` to the DNS name and replace `WOOM_SECRET` with a long random value. Keep `.env.production` private.

## Start

```bash
docker compose --env-file .env.production -f compose.production.yml up -d --build
```

Check service health:

```bash
curl https://meeting.example.com/healthz
curl https://meeting.example.com/readyz
```

The first request should report that the process is alive. The second should report that Redis and Live777 are ready.

## Operations

```bash
docker compose --env-file .env.production -f compose.production.yml ps
docker compose --env-file .env.production -f compose.production.yml logs -f woom caddy
```

Stop the deployment with:

```bash
docker compose --env-file .env.production -f compose.production.yml down
```
