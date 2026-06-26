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

## Database load or table growth is unclear

Worker mode logs hourly table estimates using Postgres catalog statistics. To check the same signal manually without scanning full tables:

```sql
SELECT
  relname AS table_name,
  GREATEST(COALESCE(reltuples, 0), 0)::bigint AS estimated_rows,
  pg_size_pretty(pg_total_relation_size(oid)) AS total_size,
  pg_size_pretty(pg_relation_size(oid)) AS heap_size,
  pg_size_pretty(pg_indexes_size(oid)) AS index_size
FROM pg_class
WHERE relkind IN ('r', 'p')
  AND relnamespace = 'public'::regnamespace
  AND relname IN (
    'services',
    'service_submissions',
    'user_reports',
    'probe_results',
    'probe_configs',
    'service_baselines',
    'incidents'
  )
ORDER BY pg_total_relation_size(oid) DESC;
```

Optional derived tables should be visible when they exist, but missing optional tables should not be treated as an error:

```sql
SELECT
  requested.table_name,
  pg_class.oid IS NOT NULL AS exists,
  CASE
    WHEN pg_class.oid IS NULL THEN NULL
    ELSE GREATEST(COALESCE(pg_class.reltuples, 0), 0)::bigint
  END AS estimated_rows,
  CASE
    WHEN pg_class.oid IS NULL THEN NULL
    ELSE pg_size_pretty(pg_total_relation_size(pg_class.oid))
  END AS total_size
FROM (VALUES
  ('service_baselines'),
  ('service_probe_states'),
  ('probe_recent_results'),
  ('probe_hourly_rollups'),
  ('service_statuses')
) AS requested(table_name)
LEFT JOIN pg_class
  ON pg_class.relname = requested.table_name
  AND pg_class.relkind IN ('r', 'p')
  AND pg_class.relnamespace = 'public'::regnamespace
ORDER BY requested.table_name;
```

Use exact counts only when you intentionally need precision, because `COUNT(*)` can scan large tables:

```sql
SELECT COUNT(*) FROM probe_results;
SELECT COUNT(*) FROM user_reports;
```

For runtime attribution, check structured logs by `mode` and `job`: API mode emits `list_cache_summary`, probe mode emits `probe_sweep`, and worker mode emits `baseline_refresh`, `incident_reconciliation`, `probe_result_cleanup`, and `table_stats`.

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
- `storage/storage.go`
- `utils/utils.go`
- `api/routes/seo.go`
- `workers/probe.go`
