# aretheyup

A real-time outage detector that combines automated health checks with crowdsourced user reports to show you whether a service is actually down or if it's just your connection.

## Stack

- Go
    - Gin
    - GORM
- HTMX
- Tailwind
- PostgreSQL
- Redis

## Idea

- Real-time SEO-friendly status pages for different websites
- Minimal design, as little JS as possible (heavy HTMX + Go)
- Get data from
    - user reports (/{service} will have a button)
    - automated ping bots to supplement
- 100% anonymous
- Free for the public page, paid API (in the future maybe, starting with the public thing)

## Development

1. Start the services:

```bash
docker compose -f docker-compose.dev.yml up -d
```

2. Run the app:

```bash
go run main.go
# or
air main.go # automatic restarts
```
