# Deployment

## Production topology

The checked-in production deployment path is `docker-compose.prod.yml`.

It defines app and dependency services behind explicit Docker Compose profiles:

- `api` profile: starts the public HTTP server on `API_PORT`
- `probe` profile: starts the synthetic probe worker
- `worker` profile: starts recurring database jobs
- `app` profile: starts `api`, `probe`, and `worker`
- `deps` profile: starts PostgreSQL 16 and Redis 7

The `api`, `probe`, and `worker` services use the same image built from `Dockerfile`, but each has an explicit runtime command:

- `api` runs `command: ["api"]`
- `probe` runs `command: ["probe"]`
- `worker` runs `command: ["worker"]`

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

### Start only the database worker

Use this when PostgreSQL and Redis are already reachable, either from managed services or from a separately started `deps` profile. Keep a single worker active unless recurring jobs are protected with advisory locks.

```bash
docker compose -f docker-compose.prod.yml --profile worker up -d --build
```

### Start all app processes together

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

- `api` starts only the Gin server
- `probe` starts only the synthetic probe worker loop; due checks run on the global 5-minute service cadence
- `worker` starts baseline refresh, status snapshot refresh, incident reconciliation, and, when enabled, raw probe cleanup loops

Raw probe cleanup is disabled unless `RAW_PROBE_RETENTION_CLEANUP_ENABLED=true`. Incident reconciliation uses legacy status calculation unless `STATUS_SNAPSHOT_INCIDENT_READS_ENABLED=true`. API status reads use legacy calculation unless `STATUS_SNAPSHOT_API_READS_ENABLED=true`.

This behavior is implemented in `main.go`, `services/db.go`, `services/redis.go`, `storage/probes.go`, `storage/statuses.go`, `workers/baseline.go`, `workers/statuses.go`, `workers/incidents.go`, `workers/probe.go`, and `workers/cleanup.go`.

## Verify the deployment

After startup, verify:

- `GET /` returns the public index page from the `api` container
- `GET /robots.txt` and `GET /sitemap.xml` reflect the intended `SITE_BASE_URL`
- the `probe` container remains running when the `probe` or `app` profile is active
- exactly one `worker` container remains running when recurring database jobs are enabled
- enabled services begin accumulating fresh `probe_results`

Useful commands:

```bash
docker compose -f docker-compose.prod.yml logs api --tail=100
docker compose -f docker-compose.prod.yml logs probe --tail=100
docker compose -f docker-compose.prod.yml logs worker --tail=100
```

The probe worker should log work for due enabled probe configs. The worker should log baseline, status refresh, and incident startup. Probe-result cleanup startup appears only after `RAW_PROBE_RETENTION_CLEANUP_ENABLED=true`. If you intend to serve probe-backed status data, the API process should not be the only app container running.

## Derived-table rollout

Use this order when moving an existing deployment to the derived read model:

1. Deploy the new version with `STATUS_SNAPSHOT_API_READS_ENABLED=false`, `STATUS_SNAPSHOT_INCIDENT_READS_ENABLED=false`, and `RAW_PROBE_RETENTION_CLEANUP_ENABLED=false`.
2. Backfill hourly probe rollups in chunks:

```bash
aretheyup backfill-probe-rollups --start 2026-01-01T00:00:00Z --end 2026-02-01T00:00:00Z --chunk-duration 24h
```

Use UTC hour-aligned `--start` and `--end` values. Chunk durations must be whole-hour multiples so the command never splits an hourly rollup bucket across chunks.

3. Capture a fixed cutoff and backfill derived probe state/recent history:

```bash
aretheyup backfill-probe-derived --cutoff 2026-02-01T00:00:00Z --service-batch-size 500
```

4. Run `worker` so `service_statuses` refreshes, then compare legacy and snapshot responses for known healthy, degraded, and outage services.
5. Enable `STATUS_SNAPSHOT_API_READS_ENABLED=true`, watch response parity, slow queries, cache behavior, and `service_statuses.computed_at` freshness.
6. Enable `STATUS_SNAPSHOT_INCIDENT_READS_ENABLED=true`, then verify active incidents still open and resolve as expected.
7. Enable `RAW_PROBE_RETENTION_CLEANUP_ENABLED=true` only after raw source data is no longer needed for parity checks.

Rollback is to turn the API or incident snapshot flag back to `false`; derived writes and refreshes can keep running while reads return to the legacy path.

## Updates and shutdown

To roll out a new version:

```bash
docker compose -f docker-compose.prod.yml --profile deps --profile app up -d --build
```

To stop the stack:

```bash
docker compose -f docker-compose.prod.yml down
```

Use `down` without `-v` if you want to preserve the checked-in Postgres and Redis volumes. If you started only one profile, you can stop just that service with `docker compose -f docker-compose.prod.yml stop api`, `stop probe`, or `stop worker`.

Relevant files:

- `docker-compose.prod.yml`
- `Dockerfile`
- `main.go`
- `utils/parse-flags.go`
- `services/db.go`
- `services/redis.go`
- `storage/probes.go`
- `storage/probe_rollups.go`
- `storage/probe_derived_backfill.go`
- `storage/statuses.go`
- `workers/baseline.go`
- `workers/statuses.go`
- `workers/incidents.go`
- `workers/probe.go`
- `workers/cleanup.go`
- `api/routes/seo.go`
- `utils/utils.go`
