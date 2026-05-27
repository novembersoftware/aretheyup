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

Then check:

- the service has an enabled probe config
- the probe URL points at the endpoint you actually want to check
- the expected status code and timeout are realistic for that endpoint

Existing services are backfilled with a default probe config from `HomepageURL` at startup, but that default may not be the best health-check target. Review or edit the config in manage mode if results look wrong. Implemented in `main.go`, `storage/probes.go`, `storage/storage.go`, and `workers/probe.go`.

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
