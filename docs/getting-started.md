# Getting Started

## Prerequisites

- Go `1.25.4` or newer, matching `go.mod`
- Docker with Compose support for local Postgres and Redis

The module and toolchain version are defined in `go.mod`.

## Local setup

1. Copy the environment template:

```bash
cp .env.example .env.local
```

2. Start local dependencies:

```bash
docker compose -f docker-compose.dev.yml up -d
```

3. Start the API server:

```bash
go run main.go
```

The process loads `.env.local`, opens PostgreSQL and Redis, runs GORM migrations, backfills any missing default probe configs from existing service homepages, starts the baseline and incident workers, then serves HTTP on `API_PORT`. This startup path is implemented in `main.go`, `services/db.go`, `services/redis.go`, `storage/probes.go`, and `api/server.go`.

4. In a second terminal, start the probe worker if you want live synthetic probe data:

```bash
go run main.go probe
```

The probe worker is a separate process. API mode reads probe results but does not execute probes itself.

## Run modes

### API server

```bash
go run main.go
go run main.go api
```

Starts the public pages and the `/api` routes. Background workers are started only in this mode. Implemented in `main.go`.

### Admin TUI

```bash
go run main.go manage
```

Opens the Bubble Tea service-management UI. Implemented in `manage/tui.go`.

### Probe worker

```bash
go run main.go probe
```

Runs the synthetic probe loop that claims due probe configs, executes HTTP checks, writes `probe_results`, and cleans up raw probe history older than 30 days. Implemented in `main.go`, `workers/probe.go`, and `storage/probes.go`.

### Seeder

```bash
go run main.go seed
go run main.go seed --count 25
go run main.go seed --count 25 --clear
```

The seed mode inserts example services, reports, probe configs/results, and incidents for development and is disabled in production. Implemented in `utils/parse-flags.go` and `services/db.go`.

## Local services

`docker-compose.dev.yml` starts:

- PostgreSQL 16 on `localhost:5432`
- Redis 7 on `localhost:6379`

The default `.env.example` values point at those containers.

## First verification

After startup, verify:

- `GET /` renders the index page.
- `GET /api/services` succeeds only when the request carries an allowed same-site `Origin` or `Referer`.
- `X-Request-ID` is present on responses.
- `go run main.go probe` begins writing probe results for enabled services.

Relevant files:

- `main.go`
- `utils/parse-flags.go`
- `services/db.go`
- `services/redis.go`
- `storage/probes.go`
- `docker-compose.dev.yml`
- `api/server.go`
- `api/middleware/origin.go`
- `api/middleware/request-id.go`
- `workers/probe.go`
