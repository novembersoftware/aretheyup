# AreTheyUp Docs

Internal maintainer docs for this repository.

## What is here

- [Getting started](./getting-started.md): local setup, run modes, and seed workflow.
- [Architecture](./architecture.md): request flow, storage layout, worker split, and status calculation.
- [Status algorithm](./algorithm.md): thresholds, baseline generation, and incident-triggering logic.
- [Configuration](./configuration.md): environment variables, origin policy, trusted proxies, and container settings.
- [Development](./development.md): test and validation commands, probe operations, and code hotspots.
- [Troubleshooting](./troubleshooting.md): common startup and runtime failures.

## Project summary

`aretheyup` is a Go application that serves public status pages and private same-site API endpoints backed by PostgreSQL and Redis. It also has a separate synthetic probe worker that writes probe data back into Postgres. The binary can run as:

- the HTTP server (`api`, default)
- the Bubble Tea admin TUI (`manage`)
- the synthetic probe worker (`probe`)
- a development seeder (`seed`)

This behavior is wired in `main.go` and `utils/parse-flags.go`.

## Recent docs baseline

This docs baseline reflects the current `HEAD` state, including:

- same-site origin enforcement on `/api/*`
- request ID and security-header middleware
- trusted proxy configuration for client IP handling
- OG image and SEO route behavior
- per-service synthetic probe configs with startup backfill from service homepages
- a separate `probe` runtime mode that executes and retains raw probe results
- probe detail data in service-card responses and templates

Relevant files:

- `main.go`
- `api/server.go`
- `api/routes/router.go`
- `api/routes/services.go`
- `api/routes/seo.go`
- `api/middleware/origin.go`
- `api/middleware/request-id.go`
- `api/middleware/security-headers.go`
- `storage/probes.go`
- `workers/probe.go`
