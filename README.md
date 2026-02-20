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

## Idea

- Real-time SEO-friendly status pages for different websites
- Minimal design, as little JS as possible (heavy HTMX + Go)
- Get data from
    - user reports (/{service} will have a button)
    - automated ping bots to supplement
- 100% anonymous
- Free for the public page, paid API (?)

## License

MIT License, see [LICENSE](./LICENSE) for details.
