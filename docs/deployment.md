# Deployment

## Production topology

The checked-in production deployment path is `docker-compose.prod.yml`.

It defines four services behind explicit Docker Compose profiles:

- `api` profile: starts the public HTTP server on `API_PORT`
- `probe` profile: starts the synthetic probe worker
- `app` profile: starts both `api` and `probe`
- `deps` profile: starts PostgreSQL 16 and Redis 7

The `api` and `probe` services use the same image built from `Dockerfile`, but each has an explicit runtime command:

- `api` runs `command: ["api"]`
- `probe` runs `command: ["probe"]`

That split is implemented in `main.go`, `utils/parse-flags.go`, and `docker-compose.prod.yml`.

## Required configuration

Set these values in the deployment environment before starting app services:

- `ENV=prod`
- `DB_DSN`
- `REDIS_URL`
- `ALLOWED_PAGE_ORIGINS`
- `SITE_BASE_URL`

Recommended additions:

- `TRUSTED_PROXIES` when the app sits behind a real ingress or CDN
- `REPORT_RATE_LIMIT_MAX_REQUESTS` and `REPORT_RATE_LIMIT_WINDOW_SECONDS` if the defaults are not appropriate

`docker-compose.prod.yml` provides default DSNs that point at the bundled `postgres` and `redis` services from the `deps` profile. Override them if production uses managed services instead.

## Bring the stack up

Use the profiles independently depending on which pieces you want to run.

### Start dependencies only

```bash
docker compose -f docker-compose.prod.yml --profile deps up -d
```

### Start only the HTTP server

Use this when PostgreSQL and Redis are already reachable, either from managed services or from a separately started `deps` profile.

```bash
docker compose -f docker-compose.prod.yml --profile api up -d --build
```

### Start only the probe worker

Use this when PostgreSQL and Redis are already reachable, either from managed services or from a separately started `deps` profile.

```bash
docker compose -f docker-compose.prod.yml --profile probe up -d --build
```

### Start both app processes together

```bash
docker compose -f docker-compose.prod.yml --profile app up -d --build
```

### Start the full local production-style stack

```bash
docker compose -f docker-compose.prod.yml --profile deps --profile app up -d --build
```

Inspect the current state with:

```bash
docker compose -f docker-compose.prod.yml ps
```

If only the API container is up, probe-backed status data will never refresh. If you are using bundled Postgres and Redis, include the `deps` profile too.

## What happens at startup

Each app container, whether started alone or through `app`:

- loads configuration
- opens PostgreSQL and Redis
- runs GORM migrations
- backfills any missing default probe configs from service `HomepageURL` with jittered initial probe times

After that, the runtimes diverge:

- `api` starts the baseline refresher, incident tracker, and Gin server
- `probe` starts the synthetic probe worker loop and raw probe cleanup loop; due checks run on the global 5-minute service cadence

This behavior is implemented in `main.go`, `services/db.go`, `services/redis.go`, `storage/probes.go`, `workers/baseline.go`, `workers/incidents.go`, and `workers/probe.go`.

## Verify the deployment

After startup, verify:

- `GET /` returns the public index page from the `api` container
- `GET /robots.txt` and `GET /sitemap.xml` reflect the intended `SITE_BASE_URL`
- the `probe` container remains running when the `probe` or `app` profile is active
- enabled services begin accumulating fresh `probe_results`

Useful commands:

```bash
docker compose -f docker-compose.prod.yml logs api --tail=100
docker compose -f docker-compose.prod.yml logs probe --tail=100
```

The probe worker should log work for due enabled probe configs. If you intend to serve probe-backed status data, the API process should not be the only app container running.

## Updates and shutdown

To roll out a new version:

```bash
docker compose -f docker-compose.prod.yml --profile deps --profile app up -d --build
```

To stop the stack:

```bash
docker compose -f docker-compose.prod.yml down
```

Use `down` without `-v` if you want to preserve the checked-in Postgres and Redis volumes. If you started only one profile, you can stop just that service with `docker compose -f docker-compose.prod.yml stop api` or `stop probe`.

Relevant files:

- `docker-compose.prod.yml`
- `Dockerfile`
- `main.go`
- `utils/parse-flags.go`
- `services/db.go`
- `services/redis.go`
- `storage/probes.go`
- `workers/baseline.go`
- `workers/incidents.go`
- `workers/probe.go`
- `api/routes/seo.go`
- `utils/utils.go`
