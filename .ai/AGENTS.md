# Balda — Agent Guide

> AI coding agent reference for the Balda project.  
> Last updated: 2026-04-14

---

## Project Overview

**Balda** is a multiplayer turn-based Russian word game server written in Go. Two players compete on a 5×5 letter grid, placing letters to form valid Russian words and score points. This is a personal "just for fun" project and is still a work in progress.

- **Module path:** `github.com/rustwizard/balda`
- **Go version:** 1.26
- **License:** Apache 2.0

---

## Technology Stack

| Layer | Technology |
|-------|------------|
| Language | Go 1.26 |
| REST API | [ogen](https://github.com/ogen-go/ogen) — code-generated from OpenAPI 3.0 spec |
| CLI | [cobra](https://github.com/spf13/cobra) + [pflag](https://github.com/spf13/pflag) |
| Database | PostgreSQL 16 (driver: `pgx/v5`) |
| Migrations | [tern](https://github.com/jackc/tern) — embedded SQL, runs automatically on server start |
| Session store | Redis 8 (driver: `go-redis/v9`) |
| Real-time events | Centrifugo v6 |
| Frontend | Svelte 5 (runes API) + Tailwind CSS, served via Nginx in Docker |
| Logging | `log/slog` (standard library) |
| IDs | UUID v4 for games, sessions, and player state |
| Testing | `testify`, `testcontainers-go` (PostgreSQL + Redis modules) |
| Linting | `golangci-lint` v2 config (`.golangci.yml`) |
| Runtime image | `debian:trixie-slim` |

---

## Code Organization

```
balda/
├── cmd/                              # CLI entry points
│   ├── root.go                       # cobra root command
│   └── server.go                     # "server" subcommand (wires all deps)
├── internal/
│   ├── game/                         # Core game logic
│   │   ├── game.go                   # Game struct, turn loop, word submission
│   │   ├── fsm.go                    # Finite-state machine (states & events)
│   │   ├── table.go                  # 5×5 LettersTable and placement rules
│   │   ├── dictionary.go             # Embedded Russian nouns dictionary
│   │   └── game_test/                # Unit tests for game mechanics
│   ├── lobby/                        # In-memory active game registry
│   │   └── lobby.go                  # Create, join, start, list, remove games
│   ├── matchmaking/                  # Rating-based matchmaking queue
│   │   └── matchmaking.go            # Exp-window pairing algorithm
│   ├── notifier/                     # Game event notifier abstraction
│   │   ├── notifier.go               # Notifier interface + Redis sender
│   │   └── redis.go                  # Redis Pub/Sub sender implementation
│   ├── server/
│   │   ├── ogen/                     # GENERATED CODE — do not edit manually
│   │   │   └── generate.go           # go:generate directive for ogen
│   │   └── restapi/handlers/         # HTTP request handlers (implement ogen interfaces)
│   │       ├── handlers.go           # Constructor + security handlers
│   │       ├── signup.go
│   │       ├── auth.go
│   │       ├── ping.go
│   │       ├── player_state.go
│   │       ├── create_game.go
│   │       ├── join_game.go
│   │       ├── move_game.go
│   │       ├── skip_game.go
│   │       └── list_games.go
│   ├── service/                      # Application service layer
│   │   └── balda_service.go          # Orchestrates lobby, matchmaking, storage, notifier
│   ├── session/                      # Redis-backed session management
│   │   ├── session.go
│   │   └── config.go
│   ├── storage/                      # Storage abstraction
│   │   ├── storage.go                # Thin wrapper over *pgxpool.Pool
│   │   └── config.go
│   ├── centrifugo/                   # Centrifugo real-time client
│   │   ├── client.go
│   │   └── events.go                 # Real-time event payload structs
│   ├── gamecoord/                    # Bridges game FSM events to Centrifugo
│   │   └── coord.go                  # Notifier implementation (turn_change, game_state, game_over)
│   ├── flname/                       # Auto-generated player nicknames
│   │   └── flname.go
│   └── rnd/                          # RNG utilities
│       └── rnd.go
├── api/openapi/                      # OpenAPI 3.0 specification
│   ├── http-api.yaml
│   └── spec.go                       # Embedded spec served at /docs/openapi.yaml
├── migrations/                       # SQL migration files
│   ├── 001_initial.up.sql
│   ├── 002_player_state.up.sql
│   └── migrations.go                 # Migrate() entry point using tern
├── tests/                            # Integration tests (HTTP handlers via testcontainers)
├── build/
│   └── Dockerfile                    # Multi-stage build
├── frontend/                         # Svelte 5 frontend (client-demo branch)
│   ├── src/
│   │   ├── App.svelte                # Root component; Centrifugo connection + event dispatch
│   │   ├── components/               # AuthForm, Lobby, WaitingScreen, GameScreen, Board, …
│   │   ├── stores/game.svelte.ts     # Reactive game state (phase, board, players, turn)
│   │   ├── lib/
│   │   │   ├── api.ts                # Typed fetch wrappers for all REST endpoints
│   │   │   └── centrifugo.ts         # CentrifugoClient singleton (connect/subscribe/events)
│   │   └── types.ts                  # Shared TypeScript interfaces matching API schemas
│   └── Dockerfile                    # Nginx-served production build
├── docker-compose.yml                # Postgres + Redis + Centrifugo + server + frontend
├── Makefile                          # build / code-gen / test / docker / restart
├── .golangci.yml
└── main.go
```

---

## Build and Test Commands

All commands are driven from the `Makefile`:

```bash
# Build the binary to ./bin/balda
make build

# Regenerate the ogen server/models from api/openapi/http-api.yaml and vendor
make code-gen

# Run all tests (excludes packages whose path contains "integration")
make test

# Build Docker image
make docker

# Restart Docker Compose (rebuild + restart all services)
make restart
```

### Running the server locally

Migrations run automatically on startup, but you **must** export the migration DSN first:

```bash
export MIGRATION_CONN_STRING="postgres://balda:password@localhost:5432/balda"

./bin/balda server \
  --server.addr 0.0.0.0 \
  --server.port 9666 \
  --server.x_api_token your-api-token \
  --pg.host localhost --pg.port 5432 \
  --pg.user balda --pg.database balda --pg.password password \
  --redis.addr localhost:6379
```

All flags can also be set via environment variables (e.g., `SERVER_ADDR`, `PG_HOST`, `REDIS_ADDR`).

### Docker Compose (recommended for local dev)

```bash
docker-compose up
```

Brings up PostgreSQL, Redis, Centrifugo, and the game server on port `9666`.

---

## Testing Strategy

The project uses a mix of **unit tests** and **integration tests**:

- **Unit tests** live next to the code they test (`*_test.go` inside the same package) or in a `game_test/` sub-package.
- **Integration tests** are in `tests/` and some files like `internal/lobby/matchmaking_integration_test.go`. They spin up real PostgreSQL and Redis containers via `testcontainers-go`.
- **Handler tests** (`tests/handlers_test.go`) wire the full HTTP stack (including ogen security middleware) against ephemeral databases.

> **Prerequisite for integration tests:** Docker must be running.

To run tests manually:

```bash
go test -v ./...
```

The Makefile and CI both pipe package lists through `grep -v integration`, but there is no `integration` package in the repo, so in practice this runs all tests.

---

## Code Style Guidelines

- **Linting:** `golangci-lint` v2 is configured in `.golangci.yml`.
- **Enabled linters:** `dogsled`, `errcheck`, `goconst`, `gocritic`, `revive`, `gosec`, `govet` (with shadow), `ineffassign`, `nakedret`, `staticcheck`, `unconvert`, `unparam`, `unused`, `whitespace`, `prealloc`, `misspell`.
- **Formatters:** `gofmt`, `goimports`.
- **Test exemptions:** several linters are disabled for `_test.go` files (see `.golangci.yml` exclusions).
- **Do not edit** anything under `internal/server/ogen/` — it is fully generated by `make code-gen`.

---

## API and Authentication

Base path: `/balda/api/v1`

- Authentication uses an `X-API-Key` header (or `api_key` query parameter) matched against `--server.x_api_token`.
- Session-sensitive endpoints also require `X-API-Session` (the session SID).
- Swagger UI is available at `/balda/api/v1/docs` when the server is running.
- The OpenAPI YAML is served from `/balda/api/v1/docs/openapi.yaml` (embedded from `api/openapi/http-api.yaml`).

Key endpoints:

| Method | Path | Description |
|--------|------|-------------|
| POST | `/signup` | Register a new user |
| POST | `/auth` | Authenticate and get a session |
| POST | `/session/ping` | Session keepalive |
| GET | `/player/state/{uid}` | Get player profile |
| GET | `/games` | List active games |
| POST | `/games` | Create a new waiting game |
| POST | `/games/{id}/join` | Join a waiting game |
| POST | `/games/{id}/move` | Submit a move (place letter + word) |
| POST | `/games/{id}/skip` | Skip the current turn |

---

## Database and Migrations

- **Schema:** Two main tables: `users` and `player_state`.
- **Password hashing:** Uses PostgreSQL `pgcrypto` (`crypt(..., gen_salt('bf', 8))`).
- **Migrations:** Located in `migrations/*.sql`. Applied automatically by `migrations.Migrate()` on server startup.
- **Requirement:** The environment variable `MIGRATION_CONN_STRING` must be set to a PostgreSQL DSN before starting the server.

---

## Runtime Architecture

The `server` command (`cmd/server.go`) wires the following dependencies:

1. Runs database migrations.
2. Creates a `pgxpool.Pool` for PostgreSQL.
3. Creates a Redis client for sessions and the notifier.
4. Constructs:
   - `session.Service` (Redis-backed)
   - `notifier.GameNotifier` (can use Redis or no-op sender)
   - `lobby.Lobby` (in-memory game registry)
   - `matchmaking.Queue` (rating-based pairing)
   - `storage.Balda` (DB access)
   - `service.Balda` (orchestrates the above)
   - `centrifugo.Client` (publishes real-time events)
   - `handlers.Handlers` (implements ogen handler and security interfaces)
5. Starts an `ogen` HTTP server with the custom mux for docs.

### Game lifecycle

- A game starts in `waiting` status when created via `POST /games`.
- When a second player joins via `POST /games/{id}/join`, the lobby transitions it to `in_progress` and launches `game.Run()` in a background goroutine.
- The game FSM drives turns with a 60-second timer per player. After 3 consecutive timeouts, the player is kicked and the game ends.
- When `game.Run()` exits, the lobby automatically removes the game.

### Centrifugo real-time events

Events are published by HTTP handlers (`internal/server/restapi/handlers/`) directly via `centrifugo.Client.Publish()`.

| Channel | Event type | When |
|---------|-----------|------|
| `lobby` | `game_created` | After `POST /games` |
| `lobby` + `game:{id}` | `game_started` | After `POST /games/{id}/join` |
| `game:{id}` | `game_state` | After game start and after each accepted move |
| `game:{id}` | `game_over` | When the game ends |

**`game_state` payload** (`internal/centrifugo/events.go`):
```json
{ "type": "game_state", "game_id": "…", "board": [["","","","",""],…],
  "current_turn_uid": "…", "players": [{"uid":"…","score":0,"words_count":0}],
  "status": "in_progress", "move_number": 0 }
```
Board is a 5×5 string array (empty string = empty cell; initial word is in row 2).

### Session keepalive

- Session TTL is 30 s by default. The frontend (`App.svelte`) calls `POST /session/ping` every 5 s after auth to keep the session alive.
- Clients must not rely on the Centrifugo `game_state` event for the initial board state after joining — the board is returned directly in `JoinGameResponse.board` to avoid the publish-before-subscribe race condition.

### Frontend phases (Svelte store `game.svelte.ts`)

`auth` → `lobby` → `waiting` (creator) or directly `playing` (joiner) → `finished`

- `game_started` Centrifugo event transitions the creator from `waiting` → `playing`; the joiner transitions synchronously in `join()` before subscribing.
- Centrifugo is connected once after auth (`centrifugoToken` effect) and never disconnected on phase changes.

---

## Security Considerations

- API key authentication is a simple static token check (`X-API-Key` header or `api_key` query param). It is **not** OAuth/JWT.
- Session IDs are stored in Redis with a configurable TTL (default 30s).
- Passwords are hashed with bcrypt (via PostgreSQL `pgcrypto`) before storage.
- Centrifugo connection and subscription tokens are HMAC-signed on the server side; the secret is passed via `--centrifugo.token_hmac_secret_key`.
- The `gosec` linter is enabled, but `G115` (integer conversion checks) is excluded because of frequent false positives.

---

## CI / CD

GitHub Actions workflow (`.github/workflows/ci.yml`):

1. Checks out the repo.
2. Sets up Go from `go.mod`.
3. Installs `go-swagger` (legacy step; the project actually uses `ogen`).
4. Runs `make code-gen`.
5. Builds the project: `go build ./...`
6. Runs tests: `go test -v $(go list ./... | grep -v integration)`.
7. Builds the Docker image.

---

## Quick Reference for Agents

- **Never edit** `internal/server/ogen/*_gen.go` files.
- **Always regenerate** after changing `api/openapi/http-api.yaml`:
  ```bash
  make code-gen
  ```
- When adding new handlers, implement the corresponding ogen-generated interface methods in `internal/server/restapi/handlers/`.
- Keep game logic inside `internal/game/`; keep HTTP concerns inside `internal/server/restapi/handlers/`.
- Use `log/slog` for logging.
- Integration tests that need real DB/Redis should go in `tests/` or use `testcontainers-go` directly.

---

## Agent Rules (DO NOT OVERRIDE)

- **NEVER commit, push, merge, rebase, or perform any git mutations** unless the user explicitly says "commit", "закоммить", or otherwise gives clear permission.
- If unsure whether the user wants a commit, ask first. Do not guess.
