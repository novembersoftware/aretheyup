# Architecture

## Overview

The application is a server-rendered status site with a same-site API, PostgreSQL as the source of truth, and Redis for cache and rate-limit state.

Core runtime flow:

1. `main.go` loads config and logging, then connects to Postgres and Redis.
2. `services/MigrateDB` applies schema changes with GORM `AutoMigrate`.
3. `storage.BackfillMissingProbeConfigs` ensures every existing service has a default probe config derived from its `HomepageURL`.
4. API mode boots Gin for HTML pages, HTMX fragments, and JSON responses.
5. Worker mode starts recurring database loops:
   - baseline refresh every hour
   - status snapshot refresh every 30 seconds
   - incident reconciliation every minute
   - policy-aware raw probe result cleanup every hour
   - table-stat logging every hour
6. Probe mode runs a separate synthetic worker loop that claims due probe configs and writes `probe_results`.

## HTTP surface

### Page routes

Defined in `api/routes/router.go`:

- `GET /`
- `GET /robots.txt`
- `GET /sitemap.xml`
- `GET /:slug`

These routes are wrapped by `RequireAllowedPageOrigin`. Direct navigation with no `Origin` header is allowed; HTMX requests must match an allowed origin or referer. Implemented in `api/middleware/origin.go`.

### API routes

Defined in `api/routes/router.go` under `/api`:

- `GET /api/services`
- `GET /api/services/search`
- `GET /api/service/:slug`
- `POST /api/service/:slug/report`
- `GET /api/services/count`

All `/api` routes require a same-site origin or referer. The report endpoint also applies Redis-backed rate limiting. Implemented in `api/middleware/origin.go` and `api/middleware/rate-limit.go`.

## Request pipeline

The Gin server is configured in `api/server.go` with this middleware order:

1. `gin.Recovery()`
2. request IDs
3. security headers
4. request logging

Templates are parsed from `templates/*.html` and `templates/components/*.html`. Static assets are served from `static/`, with dedicated routes for `/favicon.ico` and `/og-image.png`. Implemented in `api/server.go`.

## Status calculation

The status algorithm is defined in `algorithm/status.go` and documented in [algorithm.md](./algorithm.md).

`service_statuses` stores precomputed decisions used by API list/search/detail reads and by incident reconciliation. Detail responses also include recent probe history, latency summaries, and last-success / last-failure labels assembled in `api/routes/services.go`, `storage/probes.go`, and `utils/probes.go`.

## Workers

### Baseline refresher

`workers/baseline.go` runs only in `worker` mode. It refreshes all service baselines immediately at startup and then every hour. This keeps the hour-of-week baseline table warm for request handlers.

### Incident tracker

`workers/incidents.go` runs only in `worker` mode. It reconciles current service state once per minute from `service_statuses` and opens or resolves incidents based on transitions into and out of `Outage`. Probe-only `Degraded` states are visible in the UI but do not create incident records.

### Status refresher

`workers/statuses.go` runs only in `worker` mode. It refreshes `service_statuses` immediately at startup and then every 30 seconds. Refreshes batch report counts, baselines, and derived probe state before upserting snapshot rows.

### Probe result cleaner

`workers/cleanup.go` runs only in `worker` mode. It deletes expired raw probe rows immediately at startup and then once per hour, retaining raw successes for 24 hours and raw failures for 14 days. Cleanup deletes successes and failures in separate small-batch scans and only runs `VACUUM (ANALYZE) probe_results` after a large purge. Run any historical rollup backfill before starting worker mode on existing production data, because raw history is incomplete after success rows expire.

### Synthetic probe worker

`workers/probe.go` runs only in `probe` mode. It:

- wakes every 5 seconds to claim due probe configs for active services
- schedules each service on the global 5-minute cadence stored in `next_run_at`
- leases configs in batches of 16 using `FOR UPDATE SKIP LOCKED`
- executes HTTP requests with normalized method, timeout, and expected status handling
- records typed failure reasons such as `timeout`, `dns`, `connect`, `tls`, and `http_status`

Probe configs are stored per service in `probe_configs`. New services get a default config from `HomepageURL`, and startup backfill applies the same default to older rows that predate the probe feature. New and backfilled configs get deterministic initial jitter across the 5-minute cadence window, and recurring schedules advance from the stored `next_run_at` phase so probe worker restarts do not collapse every service onto the same due time. The legacy `interval_seconds` column is retained for compatibility but is not used for scheduling.

## Storage and data model

Schema models live in `structs/schema.go`:

- `Service`: primary service record
- `UserReport`: user-submitted outage report
- `ProbeResult`: external probe outcome
- `ProbeConfig`: probe definition per service
- `ServiceProbeState`: current derived probe state per service
- `ProbeRecentResult`: capped recent probe history per service
- `ProbeHourlyRollup`: compact hourly probe aggregates for baselines
- `ServiceBaseline`: hour-of-week report and probe baseline statistics
- `ServiceStatus`: precomputed current status snapshot
- `Incident`: tracked outage window

The storage layer in `storage/` owns SQL queries, Redis-backed list caching, baseline lookups, incident lookups, probe leasing/history queries, and manage-mode CRUD.

Notable storage behavior:

- service lists are cached in Redis for 10 seconds when the request matches the first page shape (`storage/storage.go`)
- detail pages aggregate histogram, regional counts, uptime days, incident history, and recent probe presentation data (`storage/service-detail.go`, `storage/probes.go`, `api/routes/services.go`, `utils/probes.go`)
- sitemap generation uses active services ordered by slug (`storage/storage.go`, `api/routes/seo.go`)

Relevant files:

- `main.go`
- `api/server.go`
- `api/routes/router.go`
- `api/routes/services.go`
- `workers/baseline.go`
- `workers/statuses.go`
- `workers/incidents.go`
- `workers/probe.go`
- `workers/cleanup.go`
- `storage/storage.go`
- `storage/probes.go`
- `storage/statuses.go`
- `storage/service-detail.go`
- `structs/schema.go`
