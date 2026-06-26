# Troubleshooting

## App fails to start with database connection errors

Check `DB_DSN` and make sure Postgres from `docker-compose.dev.yml` is running on `localhost:5432`. The server exits during startup if `services.NewDB` or `services.MigrateDB` fails. See `main.go` and `services/db.go`.

## App fails to start with Redis connection errors

Check `REDIS_URL` and verify Redis is running on `localhost:6379`. The server exits during startup if `services.NewRedis` cannot connect. See `main.go` and `services/redis.go`.

## Requests return 403 on `/api/*`

This usually means the request did not satisfy the same-site policy.

Check:

- `Origin` or `Referer` matches `ALLOWED_PAGE_ORIGINS`
- `Sec-Fetch-Site` is not `cross-site`
- HTMX requests send `HX-Request: true` when required by the route

Implemented in `api/middleware/origin.go`.

## Report button appears to stop working

The report endpoint is rate limited in Redis. For HTMX requests that do not ask for JSON, the server can respond with `204 No Content` and an `HX-Trigger` payload instead of a visible JSON error. Check Redis availability and the configured report window in `REPORT_RATE_LIMIT_WINDOW_SECONDS`. Implemented in `api/middleware/rate-limit.go`.

## Probe data never updates

The API process does not execute probes. Run the separate worker with:

```bash
go run main.go probe
```

If you deploy with `docker-compose.prod.yml`, the checked-in Compose file already includes that worker as the `probe` service.
Start it with the `probe` profile, or use the `app` profile if you want both app containers together.

Then check:

- the service has an enabled probe config
- the probe URL points at the endpoint you actually want to check
- the expected status code and timeout are realistic for that endpoint

Existing services are backfilled with a default probe config from `HomepageURL` at startup, but that default may not be the best health-check target. Review or edit the config in manage mode if results look wrong. Implemented in `main.go`, `storage/probes.go`, `storage/storage.go`, and `workers/probe.go`.

## Snapshot status reads look stale or empty

Snapshot-backed reads require three things:

- `worker` is running and logging status refreshes
- `service_statuses.computed_at` is fresh for active services
- `STATUS_SNAPSHOT_API_READS_ENABLED=true` is enabled only after snapshot parity has been validated

Keep `STATUS_SNAPSHOT_API_READS_ENABLED=false` to use the legacy calculation while you backfill or troubleshoot. For existing production data, run:

```bash
go run main.go backfill-probe-rollups --start 2026-01-01T00:00:00Z --end 2026-02-01T00:00:00Z --chunk-duration 24h
go run main.go backfill-probe-derived --cutoff 2026-02-01T00:00:00Z --service-batch-size 500
```

Use a real fixed cutoff from the rollout window, not the example timestamp above. Implemented in `storage/statuses.go`, `workers/statuses.go`, `storage/probe_rollups.go`, and `storage/probe_derived_backfill.go`.

## Raw probe rows are not being deleted

That is expected until `RAW_PROBE_RETENTION_CLEANUP_ENABLED=true`. Cleanup is disabled by default so raw probe data remains available for derived-table backfill and parity validation. Once enabled, worker mode deletes expired raw successes after 24 hours and raw failures after 14 days in batches.

## Client IPs look wrong behind a proxy

If `TRUSTED_PROXIES` is empty or invalid, Gin will not trust forwarded IP headers. Also note that `utils.GetClientIP` prefers `CF-Connecting-IP` when present. Review `api/server.go`, `api/server_test.go`, and `utils/utils.go`.

## Canonical or sitemap URLs are wrong

Set `SITE_BASE_URL` explicitly. Without it, URL builders fall back to the inbound request host when a request context exists, and `robots.txt` omits the sitemap line entirely when the value is empty. See `utils/utils.go` and `api/routes/seo.go`.

## Seed mode does nothing in production

That is intentional. `services.SeedDB` returns early when `ENV=prod`. See `services/db.go`.

Relevant files:

- `main.go`
- `services/db.go`
- `services/redis.go`
- `api/middleware/origin.go`
- `api/middleware/rate-limit.go`
- `api/server.go`
- `api/server_test.go`
- `storage/probes.go`
- `storage/probe_rollups.go`
- `storage/probe_derived_backfill.go`
- `storage/statuses.go`
- `storage/storage.go`
- `utils/utils.go`
- `api/routes/seo.go`
- `workers/probe.go`
- `workers/statuses.go`
- `workers/cleanup.go`
