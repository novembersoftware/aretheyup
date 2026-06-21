# Development

## Core commands

```bash
go test ./...
go vet ./...
go run main.go
go run main.go probe
go run main.go manage
go run main.go seed --count 25 --clear
```

CI runs `go test ./...` and `go vet ./...` in `.github/workflows/ci.yml`.

## Local development workflow

1. Start Postgres and Redis with `docker-compose.dev.yml`.
2. Run `go run main.go`.
3. Run `go run main.go probe` in another terminal when you want live synthetic checks instead of only seeded probe history.
4. Use `go run main.go seed --count 25 --clear` when you need a realistic local dataset.
5. Use `go run main.go manage` to create, edit, or delete services and probe configuration.

## Admin TUI behavior

The manage mode is implemented in `manage/`.

Key flows in `manage/tui.go`:

- `n`: create a service
- `enter`: open detail view
- `e`: edit the selected service
- `d`: delete the selected service
- `q`: quit the list view

Manage mode reads and writes through the same storage layer used by the HTTP server.

Probe-specific behavior:

- new services are created with a default probe config based on `HomepageURL`
- the form can edit probe URL, method, timeout, expected status, and enabled state
- the list view shows whether the current service has probes enabled, disabled, or missing

## Files to check when behavior changes

- request admission or headers: `api/middleware/`
- route shape or response structure: `api/routes/`
- status behavior: `algorithm/status.go`, `utils/api-status.go`
- SQL queries and aggregation: `storage/`
- schema or migration effects: `structs/schema.go`, `services/db.go`
- startup and mode wiring: `main.go`, `utils/parse-flags.go`

## Testing focus

Existing tests already cover several security-sensitive paths:

- trusted proxy parsing and client IP handling in `api/server_test.go`
- same-site origin enforcement in `api/middleware/origin_test.go`
- request IDs, security headers, and rate limiting in the corresponding middleware test files
- algorithm behavior in `algorithm/status_test.go`
- probe execution, cleanup, and failure classification in `workers/probe_test.go` and `workers/incidents_test.go`
- probe storage and presentation helpers in `storage/probes_test.go` and `utils/probes_test.go`

When changing request policy, keep those tests aligned with the intended browser behavior.

Relevant files:

- `.github/workflows/ci.yml`
- `manage/tui.go`
- `manage/form.go`
- `api/server_test.go`
- `api/middleware/origin_test.go`
- `api/middleware/request-id_test.go`
- `api/middleware/security-headers_test.go`
- `api/middleware/rate-limit_test.go`
- `algorithm/status_test.go`
- `storage/probes_test.go`
- `utils/probes_test.go`
- `workers/probe_test.go`
- `workers/incidents_test.go`
