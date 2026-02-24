# aretheyup (coming soon)

A real-time outage detector that combines automated health checks with crowdsourced user reports to show you whether a service is actually down or if it's just your connection.

## Development

### Requirements

1. Go (v1.25+)
2. Docker

```bash
# run docker services
docker compose -f docker-compose.dev.yml up -d
```

- Set variables in .env.local (check [.env.example](./.env.example))

### Run in API mode

```bash
# run the app with air
air main.go
# OR, normally
go run main.go
```

### Run in Manage mode

Manage mode is a TUI for managing the services in the database.

```bash
go run main.go manage
```

### Run in Seed mode

Seed mode is used to seed the database with test data.

```bash
go run main.go seed
# supports `--count x` and `--clear` flags
# --count x will seed x services
# --clear will clear the database before seeding
```

### Cleanup

```bash
# shutdown the docker services
docker compose -f docker-compose.dev.yml down
```

## Stack

- Go
    - Gin
    - GORM
- HTMX
- Tailwind
- PostgreSQL
- Redis

## Status Algorithm

The app uses a baseline-driven algorithm to determine the status of a service.

### TL;DR

- Services are either `Operational` or `Issues Detected`
- We look at user reports in a rolling 30-minute window
- We compare that window to a historical baseline for the same day/hour bucket
- We also look at recent probe failures (if probe results exist)
- If either signal looks bad, status becomes `Issues Detected`

### Baseline model

Baselines are stored in `service_baselines` (GORM struct: `ServiceBaseline`).

Each row is keyed by:

- `service_id`
- `hour_of_week` (0..167, UTC)

For each bucket, we store:

- `mean_reports`
- `std_dev_reports`
- `sample_count` (how many distinct weeks contributed)
- `probe_failure_rate`
- `probe_failure_samples`

### How baselines are refreshed

- A background worker (`workers/baseline.go`) runs on API startup
- It refreshes immediately, then every hour
- It recomputes stats per active service
- It uses up to 6 months of history
- User-report baseline is built from 30-minute windows (including zero-report windows)

### User-report signal

Implemented in `algorithm/status.go`:

1. **Cold start path** (not enough history)
    - If `sample_count < 4`, we use a hard threshold
    - Requires at least 15 reports in the 30-minute window
2. **Mature baseline path**
    - Compute `z = (current - mean) / max(stdDev, 1)`
    - Trigger only when both are true:
        - `z >= 3.0`
        - `current_reports >= 3`

This setup is designed to avoid the "baseline=0, one report => incident" problem.

### Probe signal

Also in `algorithm/status.go`:

- Look at the most recent 5 probe results
- If fewer than 3 results exist, ignore probe signal
- If probe baseline is immature (`probe_failure_samples < 20`):
    - trigger when failure rate >= 0.8
- If probe baseline is mature:
    - trigger when failure rate is meaningfully above normal

Note: this repo currently consumes probe _results_ for scoring, but does not yet include a probe runner that generates those results.

### API integration

- List/search/detail endpoints all use the same status decision flow
- Baselines and probe stats are fetched in batches for list/search to avoid N+1 issues
- Templates now show a single issue state label: `Issues Detected`

### Incident tracking

Incident records are managed by a background worker (`workers/incidents.go`).

- Runs on API startup, then every minute
- Recomputes current status for each active service using the same algorithm signals
- Opens an incident when status flips to `Issues Detected` and no active incident exists
- Resolves the incident when status returns to `Operational`

This keeps incident history aligned with the status algorithm without relying on page requests.

## Idea

- Real-time SEO-friendly status pages for different websites
- Minimal design, as little JS as possible (heavy HTMX + Go)
- Get data from
    - user reports (/{service} will have a button)
    - automated ping bots to supplement
- 100% anonymous
- Free for the public page, paid API (?)

## SEO Crawling

- `GET /robots.txt` allows all page crawling and disallows `/api/`
- `GET /sitemap.xml` includes `/` and all active service pages (`/:slug`)
- Sitemap links are generated from `SITE_BASE_URL`, so set it to your production domain

## Security configuration

- Set `TRUSTED_PROXIES` to the exact CIDRs/IPs for your edge and ingress proxies.
- Leave `TRUSTED_PROXIES` empty to trust none (safest default).
- For Cloudflare deployments, include Cloudflare edge CIDRs and keep them updated.
- Keep `SITE_BASE_URL` set in production so generated absolute URLs do not depend on request headers.
- API routes are website-gated: `/api/*` requests must include an allowed website `Origin` or `Referer`.

## License

MIT License, see [LICENSE](./LICENSE) for details.
