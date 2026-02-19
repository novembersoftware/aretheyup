# aretheyup

A real-time outage detector that combines automated health checks with crowdsourced user reports to show you whether a service is actually down or if it's just your connection.

## Development

### Requirements

1. Go (v1.25+)
2. Docker

### Run the app

- Set variables in .env.local (check [.env.example](./.env.example))

```bash
# run docker services
docker compose -f docker-compose.dev.yml up -d
# run the app with air
air main.go
# OR, normally
go run main.go
```

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
