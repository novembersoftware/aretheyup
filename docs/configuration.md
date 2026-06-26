# Configuration

## Environment variables

The config struct is defined in `structs/config.go` and loaded by `config/config.go`.

| Variable | Required | Purpose |
| --- | --- | --- |
| `ENV` | no | Runtime mode. `prod` enables Gin release mode and production logging. |
| `API_PORT` | no | HTTP port for Gin. |
| `DB_DSN` | yes | PostgreSQL connection string used by GORM. |
| `REDIS_URL` | yes | Redis connection URL used for cache and rate limiting. |
| `SITE_BASE_URL` | strongly recommended | Base URL for canonical URLs, Open Graph metadata, and `robots.txt` sitemap output. |
| `ALLOWED_PAGE_ORIGINS` | yes in deployed setups | Comma-separated allowed origins for page HTMX requests and all `/api/*` requests. |
| `TRUSTED_PROXIES` | optional | Comma-separated proxy CIDRs or IPs trusted for forwarded client IP handling. |
| `REPORT_RATE_LIMIT_MAX_REQUESTS` | no | Max allowed report submissions per key within the rate-limit window. |
| `REPORT_RATE_LIMIT_WINDOW_SECONDS` | no | Report rate-limit window length in seconds. |
| `STATUS_SNAPSHOT_API_READS_ENABLED` | no | Reserved rollout flag loaded for compatibility. Current beta reads service status from `service_statuses`. |
| `STATUS_SNAPSHOT_INCIDENT_READS_ENABLED` | no | Reserved rollout flag loaded for compatibility. Current beta reconciles incidents from `service_statuses`. |
| `RAW_PROBE_RETENTION_CLEANUP_ENABLED` | no | Reserved rollout flag loaded for compatibility. Current beta worker mode deletes expired raw probe rows. |

Defaults and sample values live in `.env.example`.

## Origin and request policy

`ALLOWED_PAGE_ORIGINS` is more than a CORS-style hint. It drives request admission:

- page HTMX requests must originate from an allowed origin or referer
- `/api/*` requests must be same-site and match an allowed origin or referer
- cross-site `Sec-Fetch-Site` values are rejected for the API
- `POST /api/service/:slug/report` can return an HTMX trigger payload instead of JSON when rate limited

Implemented in `api/middleware/origin.go` and `api/middleware/rate-limit.go`.

## Trusted proxies

`TRUSTED_PROXIES` is passed into Gin `SetTrustedProxies` after CSV normalization in `api/server.go`. If it is empty, Gin does not trust forwarded proxy headers.

Client IP behavior:

- `utils.GetClientIP` prefers `CF-Connecting-IP`
- otherwise it falls back to Gin `ClientIP()`
- untrusted proxies cannot spoof `X-Forwarded-For`

This behavior is covered in `api/server_test.go`.

## Security headers

`api/middleware/security-headers.go` adds:

- `X-Content-Type-Options`
- `X-Frame-Options`
- `Referrer-Policy`
- `Permissions-Policy`
- `Cross-Origin-Opener-Policy`
- `Cross-Origin-Resource-Policy`
- `Content-Security-Policy-Report-Only`
- `Strict-Transport-Security` when the request is HTTPS or forwarded as HTTPS

The CSP is currently report-only, not enforcing.

## Derived Data Rollout

Current beta reads from the derived status tables and runs raw probe cleanup in worker mode. For an existing deployment, run backfills before starting the new worker against historical raw probe data.

Recommended order:

1. Run the probe rollup and derived probe backfills.
2. Let `worker` refresh `service_statuses`.
3. Compare response shape, slow queries, cache behavior, `service_statuses.computed_at` freshness, and incident transitions.
4. Confirm raw cleanup is deleting only rows older than the retention windows.

Rollback is deployment-based: redeploy the previous version while the derived tables continue to be maintained.

## Container configuration

### Development

`docker-compose.dev.yml` provides local Postgres and Redis only. The app still runs with `go run main.go`.

### Production-style compose

`docker-compose.prod.yml` runs:

- an `api` profile that starts the HTTP server
- a `probe` profile that starts the synthetic probe worker
- a `worker` profile that starts recurring database jobs
- an `app` profile that starts all app containers together
- a `deps` profile that starts PostgreSQL and Redis

The app services are built from `Dockerfile`. `api` runs `command: ["api"]`, `probe` runs `command: ["probe"]`, and `worker` runs `command: ["worker"]`. The Compose file sets `ENV`, `API_PORT`, `DB_DSN`, `REDIS_URL`, `ALLOWED_PAGE_ORIGINS`, and the rollout gate defaults. Set `SITE_BASE_URL` explicitly in real deployments if you need stable canonical and sitemap URLs independent of the inbound host. `robots.txt` omits the sitemap line when `SITE_BASE_URL` is empty. That behavior is implemented in `api/routes/seo.go` and `utils/utils.go`.

Relevant files:

- `.env.example`
- `structs/config.go`
- `config/config.go`
- `api/server.go`
- `api/server_test.go`
- `api/middleware/origin.go`
- `api/middleware/rate-limit.go`
- `api/middleware/security-headers.go`
- `api/routes/seo.go`
- `docker-compose.dev.yml`
- `docker-compose.prod.yml`
- `Dockerfile`
