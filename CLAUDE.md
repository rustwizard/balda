# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build       # build binary → ./bin/balda
make code-gen    # regenerate ogen code from api/openapi/http-api.yaml + go mod vendor
make test        # run all tests (excludes paths containing "integration")
make docker      # build Docker image
make restart     # docker-compose rebuild + restart

go test -v ./internal/game/...          # run a specific package
go test -v ./tests -run TestCreateGame  # run a single integration test
```

Full stack for local dev: `docker-compose up` (Postgres, Redis, Centrifugo, server on `:9666`).

Running the binary directly requires `MIGRATION_CONN_STRING` env var (PostgreSQL DSN); migrations run automatically on startup.

## Code Generation

`internal/server/ogen/*_gen.go` is fully generated — **never edit it manually**. After any change to `api/openapi/http-api.yaml`, run:

```bash
make code-gen
```

Always run `go mod vendor` after `go get` (project uses a vendor directory).

## Architecture

**Request flow:** ogen HTTP server → security middleware (`X-API-Key` / `api_key`) → handler (`internal/server/restapi/handlers/`) → `service.Balda` → lobby / matchmaking / storage / notifier → real-time publish via `centrifugo.Client`.

**Key layers:**

| Package | Role |
|---------|------|
| `internal/game/` | Core FSM, 5×5 board, dictionary, word validation |
| `internal/lobby/` | In-memory active game registry |
| `internal/matchmaking/` | Rating-window pairing queue |
| `internal/service/` | Orchestrates all domain objects |
| `internal/server/restapi/handlers/` | HTTP handlers; implement ogen-generated interfaces |
| `internal/server/ogen/` | **Generated** — server router, encoders, security middleware |
| `internal/centrifugo/` | Publishes real-time events to Centrifugo |
| `internal/gamecoord/` | Bridges game FSM events → Centrifugo channels |
| `internal/session/` | Redis-backed session management (TTL 30 s) |
| `internal/storage/` | Thin pgxpool wrapper |
| `migrations/` | SQL migrations via tern; run automatically on start |

**Game lifecycle:** `POST /games` creates a `waiting` game → second player joins via `POST /games/{id}/join` → lobby launches `game.Run()` in a background goroutine → FSM drives 60-second turns → 3 consecutive timeouts kick the player → game ends and is removed from lobby.

**Real-time channels:**
- `lobby` — `game_created`, `game_started`
- `game:{id}` — `game_state` (after start and each accepted move), `game_over`

Joiner receives the initial board in `JoinGameResponse.board` directly (not via Centrifugo) to avoid a publish-before-subscribe race.

## Adding a New Endpoint

1. Add the path/operation to `api/openapi/http-api.yaml`.
2. Run `make code-gen`.
3. Implement the new interface method in `internal/server/restapi/handlers/` (one file per handler is the convention).
4. Keep HTTP concerns in handlers; keep game logic in `internal/game/`.

## Testing

- Unit tests: next to the code or in `game_test/` sub-package.
- Integration tests: `tests/` and `internal/lobby/matchmaking_integration_test.go` — spin up real Postgres + Redis via `testcontainers-go`. **Docker must be running.**
- Use `log/slog` for logging (no third-party logger).
- Linter: `golangci-lint` v2 with config in `.golangci.yml`.
