# Go Telegram Bot Template

A production-ready template for building Telegram bots in Go. Built on top of [gotgbot](https://github.com/PaulSonOfLars/gotgbot), it includes everything you need to ship a reliable, scalable bot out of the box.

## Features

- **Webhook-based updates** — no long polling; secure via secret token validation
- **Telegram WebApp support** — built-in Gin server with WebApp request validation
- **Rate limiting** — token bucket for private chats, sliding window for group chats
- **PostgreSQL integration** — pgxpool connection, Goose migrations, soft deletes
- **Redis integration** — ready-to-use client for caching and pub/sub
- **In-memory user cache** — lazy-loaded, auto-synced to DB, auto-cleanup
- **Localization** — YAML-based i18n with per-locale command descriptions
- **Structured logging** — Zap logger with slog bridge
- **Graceful shutdown** — grace period for in-flight requests
- **Docker** — multi-stage build producing a minimal scratch image; Docker Compose dev stack
- **ngrok integration** — config template for exposing webhook and WebApp locally

## Architecture Overview

```
main.go                  Entry point: wires everything together, manages lifecycle
├── configuration/       Viper-based config (file + MY_BOT_* env vars)
├── internals/
│   ├── bot/             Bot init, webhook server, DB pool, settings management
│   ├── commands/        Bot command definitions
│   ├── gin_server/      WebApp HTTP server (Gin)
│   ├── limiters/        Rate limiter pools (private + group chats)
│   ├── logger/          Zap + slog setup
│   └── users_cache/     In-memory user store with DB sync
├── handlers/
│   ├── contextual/      Middleware-style handlers: enrich update context
│   └── helpers/         Shared handler utilities
├── database/
│   ├── migrations/      Goose SQL migrations (PostgreSQL)
│   └── tables/          DB table models
├── locale/              en.yaml, en_commands.yaml (add more locales here)
└── static/              Static assets served by the WebApp server
```

**Handler execution order** (by group priority):

| Group | Handler | Purpose |
|-------|---------|---------|
| -2 | `MiscContextHandler` | Injects WebApp domain and locale |
| -1 | `UserContextHandler` | Loads user from cache / DB |
| 0 | `TermsAndConditionsHandler` | Enforces T&C acceptance before proceeding |
| 2+ | Command handlers | `start`, `my_id`, your custom handlers |

## Prerequisites

- Go 1.26+
- PostgreSQL 17+
- Redis 7+
- [ngrok](https://ngrok.com/download) (for local development)
- [goose](https://github.com/pressly/goose) (for database migrations)

## Getting Started

### 1. Create your bot

Talk to [@BotFather](https://t.me/BotFather) and create a new bot. Save the token you receive.

### 2. Set up ngrok (local development)

```shell
cp ngrok.dist.yaml ngrok.yaml
# Edit ngrok.yaml: set your authtoken
ngrok start --config=ngrok.yaml bot_webhook web_app
```

ngrok will expose two tunnels: one for the webhook (port 8080) and one for the WebApp (port 8081).

### 3. Start the dev stack

```shell
docker compose up -d
```

This starts PostgreSQL 17 on port 5432 and Redis 7 on port 6375.

### 4. Run database migrations

```shell
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir ./database/migrations/postgres postgres \
  "user=user password=pass dbname=my_db host=localhost port=5432 sslmode=disable" up
```

### 5. Configure the bot

```shell
cp config.dist.yaml config.yaml
# Edit config.yaml with your values (see Configuration section below)
```

### 6. Run the bot

```shell
go run .
```

Send `/start` to your bot. If it replies, you're good to go.

## Configuration

Configuration is loaded from `config.yaml` and can be overridden by environment variables with the `MY_BOT_` prefix (uppercased automatically). Environment variables take precedence over the config file.

| Config key | Env variable | Description |
|------------|-------------|-------------|
| `token` | `MY_BOT_TOKEN` | Telegram Bot API token |
| `webhook_domain` | `MY_BOT_WEBHOOK_DOMAIN` | Public HTTPS domain for the webhook |
| `webhook_port` | `MY_BOT_WEBHOOK_PORT` | Port to listen on (default: `8080`) |
| `webhook_secret` | `MY_BOT_WEBHOOK_SECRET` | Secret for validating webhook requests |
| `webapp_domain` | `MY_BOT_WEBAPP_DOMAIN` | Public HTTPS domain for the WebApp |
| `webapp_port` | `MY_BOT_WEBAPP_PORT` | Port for the WebApp server (default: `8081`) |
| `static_content_path` | `MY_BOT_STATIC_CONTENT_PATH` | Path to static assets directory |
| `db_dsn` | `MY_BOT_DB_DSN` | PostgreSQL connection string |
| `redis_addr` | `MY_BOT_REDIS_ADDR` | Redis address (e.g. `localhost:6375`) |
| `redis_user` | `MY_BOT_REDIS_USER` | Redis username |
| `redis_pass` | `MY_BOT_REDIS_PASS` | Redis password |
| `debug` | `MY_BOT_DEBUG` | Set to `true` for verbose JSON logging |

All fields are required at startup — the bot will refuse to start if any are missing.

## Database Migrations

Create a new migration:

```shell
goose -s -dir ./database/migrations/postgres create <migration_name> sql
```

Apply migrations:

```shell
goose -dir ./database/migrations/postgres postgres "<DSN>" up
```

Roll back the last migration:

```shell
goose -dir ./database/migrations/postgres postgres "<DSN>" down
```

## Docker

### Build the image

```shell
docker build -t my-telegram-bot .
```

The multi-stage build produces a minimal image based on `scratch` (~5 MB) containing only the compiled binary, CA certificates, locale files, and static assets.

## Adding Your Own Handlers

1. Create a new handler struct implementing `CheckUpdate()` and `HandleUpdate()`.
2. Register it in `main.go` with the appropriate priority group.

Command handlers go in group 2 or higher. Use groups -2 and -1 for context-enrichment middleware. The `TermsAndConditionsHandler` at group 0 acts as a gate — any handler in group 2+ can assume the user has accepted T&C.

## Adding Locales

1. Create `locale/<lang>.yaml` and `locale/<lang>_commands.yaml`.
2. Pass `--locale-path ./locale` (default) and set the locale in `MiscContextHandler`.

## Rate Limiting

The rate limiter middleware wraps the bot client and intercepts all outbound API calls:

- **Private chats**: 1 request/second (token bucket, burst 1)
- **Group chats**: 20 requests/minute (sliding window)

Limiter pools clean up inactive entries every 4 hours (stale threshold: 24 hours).
