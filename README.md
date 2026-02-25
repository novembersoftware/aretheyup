# Are they up?

Real-time service status pages powered by user outage reports and baseline-aware scoring.

Live site: [aretheyup.com](https://aretheyup.com)

## What this project does

`aretheyup` helps answer a simple question: is a service down for everyone, or just me?

It combines:

- Crowdsourced user reports ("I am having issues")
- Historical baselines by time bucket
- Optional probe-failure signal (when probe data exists)

The result is a clear status for each service:

- `Operational`
- `Issues Detected`

## Key features

- SEO-friendly status pages (`/`, `/:slug`)
- Fast, server-rendered UI with HTMX
- Report endpoint with abuse protection via rate limiting
- Background workers for baseline refresh and incident tracking
- Service management TUI for admins (`manage` mode)

## Tech stack

- Go 1.25+
- Gin + GORM
- PostgreSQL + Redis
- HTMX + Tailwind + DaisyUI

## Quick start (local development)

### 1) Prerequisites

- Go `1.25+`
- Docker + Docker Compose

### 2) Clone and configure

```bash
git clone https://github.com/novembersoftware/aretheyup.git
cd aretheyup
cp .env.example .env.local
```

Update values in `.env.local` as needed.

### 3) Start local dependencies

```bash
docker compose -f docker-compose.dev.yml up -d
```

### 4) Run the app

```bash
go run main.go
```

Server runs on `http://localhost:8080` by default.

## Run modes

The binary supports multiple modes:

```bash
# API server (default)
go run main.go
# equivalent:
go run main.go api

# Manage mode (service admin TUI)
go run main.go manage

# Seed mode
go run main.go seed --count 25 --clear
```

Seed flags:

- `--count` number of services to seed (default `10`)
- `--clear` clear existing data before seeding

## Environment variables

See `.env.example` for full list. Most important values:

- `ENV` (`dev` or `prod`)
- `API_PORT`
- `DB_DSN`
- `REDIS_URL`
- `SITE_BASE_URL`
- `ALLOWED_PAGE_ORIGINS` (comma-separated)
- `TRUSTED_PROXIES` (comma-separated CIDRs/IPs, optional)
- `REPORT_RATE_LIMIT_MAX_REQUESTS`
- `REPORT_RATE_LIMIT_WINDOW_SECONDS`

## Status algorithm (high level)

Status is calculated from a rolling 30-minute report window and compared to historical norms.

- Baselines are stored per service and hour-of-week bucket (`0..167`, UTC)
- Cold start behavior uses conservative hard thresholds
- Mature behavior uses z-score style anomaly checks
- Probe failures (if available) can independently trigger `Issues Detected`

Reference implementation:

- `algorithm/status.go`
- `workers/baseline.go`
- `workers/incidents.go`

## SEO behavior

- `GET /robots.txt` allows public crawling and disallows `/api/`
- `GET /sitemap.xml` includes `/` and active `/:slug` pages
- Absolute sitemap URLs are derived from `SITE_BASE_URL`

## Security notes

- Set `TRUSTED_PROXIES` to only your real edge/ingress proxy CIDRs
- Keep `SITE_BASE_URL` set in production
- `/api/*` routes require allowed website origin/referer checks

## Stopping local services

```bash
docker compose -f docker-compose.dev.yml down
```

## Contributing

Issues and PRs are welcome. Please open an issue before you start working on larger changes.

## License

MIT. See [`LICENSE`](./LICENSE).
