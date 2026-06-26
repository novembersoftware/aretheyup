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
| `STATUS_SNAPSHOT_API_READS_ENABLED` | no | When `true`, service list, search, and detail status reads use `service_statuses` instead of recalculating status from raw inputs. Default `false`. |
| `STATUS_SNAPSHOT_INCIDENT_READS_ENABLED` | no | When `true`, incident reconciliation reads current status from `service_statuses`. Default `false`. |
| `RAW_PROBE_RETENTION_CLEANUP_ENABLED` | no | When `true`, worker mode deletes expired raw probe rows. Default `false` so rollout backfills can validate against raw source data first. |

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

## Rollout gates

The status snapshot and raw-retention switches are intentionally closed by default.

Recommended order:

1. Deploy migrations and dual-write code with all three flags set to `false`.
2. Run the probe rollup and derived probe backfills.
3. Let `worker` refresh `service_statuses` and compare snapshot responses against legacy responses.
4. Enable `STATUS_SNAPSHOT_API_READS_ENABLED=true` after parity looks good.
5. Enable `STATUS_SNAPSHOT_INCIDENT_READS_ENABLED=true` after incident reconciliation parity looks good.
6. Enable `RAW_PROBE_RETENTION_CLEANUP_ENABLED=true` only after raw probe history is no longer needed for validation.

Rollback is flag-based: turn the API or incident snapshot flag back to `false` to return to legacy read calculation while the derived tables continue to be maintained.

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
