# Balda

A multiplayer turn-based word game server written in Go, with a real-time Svelte 5 frontend. Players compete on a 5×5 letter grid, placing letters to form valid Russian words and score points.

> Work in progress — personal "just for fun" project.

## What is Balda?

Balda is a classic Russian word game. Two players share a 5×5 grid. The game starts with a random 5-letter Russian word placed in the center row. On each turn, a player must:

1. Place exactly one new letter on the board (adjacent to an existing letter)
2. Trace a word using letters already on the board (including the new one)
3. The word must exist in the Russian dictionary and must not have been used already

The player with the most words when the game ends wins.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.26 |
| REST API | [ogen](https://github.com/ogen-go/ogen) (code-generated from OpenAPI 3.0 spec) |
| CLI | [cobra](https://github.com/spf13/cobra) |
| Database | PostgreSQL 16 (pgx/v5 driver) |
| Session store | Redis 8 |
| Real-time events | [Centrifugo v6](https://centrifugal.dev) (WebSocket pub/sub) |
| Frontend | Svelte 5 (runes API) + Tailwind CSS, served via Nginx |
| Migrations | [tern](https://github.com/jackc/tern) (embedded SQL, runs on server start) |
| Logging | `log/slog` (standard library) |
| Runtime image | Debian trixie-slim |

## Project Structure

```
balda/
├── cmd/                    # CLI entry points (server)
├── internal/
│   ├── game/               # Core game logic, FSM, letter table, dictionary
│   ├── gamecoord/          # Coordinator: bridges game events → Centrifugo
│   ├── lobby/              # In-memory active game registry
│   ├── matchmaking/        # Rating-based matchmaking queue
│   ├── centrifugo/         # Centrifugo HTTP API client + event types
│   ├── notifier/           # Notifier abstraction (Redis sender)
│   ├── server/
│   │   ├── ogen/           # ogen-generated server code (do not edit)
│   │   └── restapi/
│   │       └── handlers/   # HTTP request handlers (move_game.go, skip_game.go, etc.)
│   ├── session/            # Redis-backed session management
│   ├── service/            # Application service layer
│   ├── storage/            # PostgreSQL access
│   ├── flname/             # Auto-generated player nicknames
│   └── rnd/                # RNG utilities
├── frontend/               # Svelte 5 frontend
│   └── src/
│       ├── App.svelte      # Root: Centrifugo connection + event dispatch
│       ├── components/     # AuthForm, Lobby, GameScreen, Board, Alphabet, …
│       ├── stores/         # Reactive game state (game.svelte.ts)
│       ├── lib/            # api.ts, centrifugo.ts
│       └── types.ts        # TypeScript interfaces
├── api/openapi/            # OpenAPI 3.0 specification
├── migrations/             # SQL migration files
├── tests/                  # Integration tests (testcontainers)
├── Makefile
└── docker-compose.yml
```

## Getting Started

### Prerequisites

- Docker and Docker Compose

### Run with Docker Compose

```bash
docker compose up
```

Starts PostgreSQL, Redis, Centrifugo, the game server on port `9666`, and the frontend on port `8080`.

Open `http://localhost:8080` to play. The frontend is also reachable from other devices on your local network via `http://<host-ip>:8080` (e.g. `http://192.168.1.42:8080`).

### Frontend development

```bash
cd frontend
npm install
npm run dev
```

The Vite dev server listens on all interfaces (`0.0.0.0:5173`), so you can open the game from a phone or another computer on the same Wi-Fi network at `http://<host-ip>:5173`.

Proxy targets for the backend and Centrifugo can be configured via environment variables (see `frontend/.env.example`):

```bash
BALDA_API_PROXY_URL=http://127.0.0.1:9666 \
BALDA_CENTRIFUGO_PROXY_URL=http://127.0.0.1:8000 \
npm run dev
```

### Rebuild and restart

```bash
make restart
```

### Build the server binary manually

```bash
make build
```

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

> **Note:** `MIGRATION_CONN_STRING` must be set before starting the server. Migrations are applied automatically at startup.

### Regenerate API Code

```bash
make code-gen
```

Regenerates the typed Go server code from [api/openapi/http-api.yaml](api/openapi/http-api.yaml) using [ogen](https://github.com/ogen-go/ogen) and vendors the result.

### Run Tests

```bash
make test
```

Integration tests in `tests/` spin up ephemeral PostgreSQL and Redis containers via [testcontainers-go](https://golang.testcontainers.org/) — Docker must be running.

---

## API

Base path: `/balda/api/v1`

Authentication uses an `X-API-Key` header (or `api_key` query parameter). Session-sensitive endpoints also require `X-API-Session`.

Swagger UI is available at `/balda/api/v1/docs` when the server is running.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/signup` | Register a new user account |
| POST | `/auth` | Authenticate and get a session |
| POST | `/session/ping` | Keepalive — refreshes session TTL |
| GET | `/player/state/{uid}` | Get player profile and state |
| GET | `/games` | List all currently active games |
| POST | `/games` | Create a new waiting game |
| POST | `/games/{id}/join` | Join an existing waiting game |
| POST | `/games/{id}/move` | Submit a move (place letter + word) |
| POST | `/games/{id}/skip` | Skip the current turn |

### POST /signup

```json
// Request
{ "firstname": "Ivan", "lastname": "Petrov", "email": "ivan@example.com", "password": "secret" }

// Response
{
  "user": { "uid": "…", "firstname": "Ivan", "lastname": "Petrov", "sid": "…", "key": "…" },
  "centrifugo_token": "…",
  "lobby_token": "…"
}
```

### POST /auth

```json
// Request
{ "email": "ivan@example.com", "password": "secret" }

// Response
{
  "player": { "uid": "…", "firstname": "Ivan", "lastname": "Petrov", "sid": "…", "key": "…" },
  "centrifugo_token": "…",
  "lobby_token": "…"
}
```

### POST /games

Creates a new game in `waiting` status. Returns a `game_token` for subscribing to the game's Centrifugo channel.

```json
// Response
{
  "game": { "id": "…", "player_ids": ["<creator>"], "status": "waiting", "started_at": 1712600000000 },
  "game_token": "…"
}
```

### POST /games/{id}/join

Joins a waiting game. When the second player joins, the game starts immediately. Returns the initial board state to avoid a publish-before-subscribe race with Centrifugo.

```json
// Response
{
  "game": { "id": "…", "player_ids": ["<creator>", "<joiner>"], "status": "in_progress", "started_at": 1712600000000 },
  "game_token": "…",
  "board": [["","","","",""],["","","","",""],["с","л","о","в","о"],["","","","",""],["","","","",""]],
  "current_turn_uid": "<creator-uid>"
}
```

### POST /games/{id}/move

Submits a move: places one new letter on the board and specifies the word path.

```json
// Request
{
  "new_letter": { "row": 3, "col": 3, "char": "е" },
  "word_path": [
    { "row": 2, "col": 0 },
    { "row": 2, "col": 1 },
    { "row": 2, "col": 2 },
    { "row": 2, "col": 3 },
    { "row": 3, "col": 3 }
  ]
}

// Response
{
  "board": [["","","","",""],…],
  "current_turn_uid": "…",
  "players": [{"uid":"…","score":5,"words_count":1}],
  "status": "in_progress",
  "move_number": 1
}
```

### POST /games/{id}/skip

Skips the current turn. Returns `204 No Content` on success.

---

## Real-time Events (Centrifugo)

After auth, the client connects to Centrifugo using `centrifugo_token`. Events flow over channels:

| Channel | Event type | When |
|---------|-----------|------|
| `lobby` | `game_created` | After `POST /games` |
| `lobby` + `game:{id}` | `game_started` | After `POST /games/{id}/join` |
| `game:{id}` | `game_state` | On turn start and after each accepted move |
| `game:{id}` | `turn_change` | On every turn change (any reason) |
| `game:{id}` | `game_over` | When the game ends |

### `game_state`

Full board snapshot — sent after game start and after each move.

```json
{ "type": "game_state", "game_id": "…", "board": [["","…"]],
  "current_turn_uid": "…", "players": [{"uid":"…","score":0,"words_count":0}],
  "status": "in_progress", "move_number": 0 }
```

### `turn_change`

General turn change notification — sent on every turn start. The `reason` field identifies why the turn changed.

```json
{ "type": "turn_change", "game_id": "…", "current_turn_uid": "…",
  "reason": "game_start" }
```

Possible `reason` values: `game_start`, `move`, `skip`, `timeout`.

### `game_over`

```json
{ "type": "game_over", "game_id": "…", "winner_uid": "…",
  "players": [{"uid":"…","score":5,"words_count":2}] }
```

Sent when the game ends — either because the board became full or a player was kicked. `winner_uid` is absent on a draw.

---

## Game Mechanics

### Board

A 5×5 grid. The starting word occupies the center row (row index 2). Coordinates are `(RowID, ColID)` from `(0,0)` to `(4,4)`.

```
[ ][ ][ ][ ][ ]   row 0
[ ][ ][ ][ ][ ]   row 1
[С][л][о][в][о]   row 2  ← initial word
[ ][ ][ ][ ][ ]   row 3
[ ][ ][ ][ ][ ]   row 4
```

### Turn

- Each player has **60 seconds** per turn.
- On timeout the turn passes to the other player automatically; no action from either client is needed.
- After **3 consecutive timeouts**, the player is kicked and the game ends.
- A player can skip a turn voluntarily via `POST /games/{id}/skip`.
- The game also ends automatically when the board is full (all 25 cells are filled).

### Word Validation

Submitted words must:
- Be **3 or more letters** long
- Include the newly placed letter
- Consist of letters traceable on the board (adjacent cells only)
- Exist in the embedded Russian nouns dictionary
- Not have been submitted before in this game
- Not be identical to the initial board word

> **Note:** `е` and `ё` are treated as the same letter for dictionary lookup, word reuse checks, and board display. For example, a word spelled with `ё` will match a dictionary entry with `е`, and vice versa.

### State Machine

Each game runs an FSM loop (`Game.Run`) driven by `TurnEvent` values sent over an internal channel.

```
┌─────────────────────┬────────────────────┬─────────────────────┐
│ State               │ Event              │ Next State          │
├─────────────────────┼────────────────────┼─────────────────────┤
│ WaitingForMove      │ MoveSubmitted      │ WaitingForMove      │
│ WaitingForMove      │ TurnSkipped        │ WaitingForMove      │
│ WaitingForMove      │ TurnTimeout        │ PlayerTimedOut      │
│ WaitingForMove      │ BoardFull          │ GameOver            │
├─────────────────────┼────────────────────┼─────────────────────┤
│ PlayerTimedOut      │ AckTimeout         │ WaitingForMove      │
│ PlayerTimedOut      │ Kick               │ GameOver            │
└─────────────────────┴────────────────────┴─────────────────────┘
```

- A 60-second timer fires `TurnTimeout` automatically. The `Coordinator` (`internal/gamecoord/`) acknowledges it via `AckTimeout`, advancing to the next player.
- `MoveSubmitted` and `TurnSkipped` reset the consecutive-timeout counter.
- On the third consecutive timeout the game auto-queues `Kick` → `GameOver`.

---

## Database Schema

**users**

| Column | Type | Notes |
|--------|------|-------|
| user_id | bigserial | PK |
| first_name | text | |
| last_name | text | |
| email | text | unique |
| hash_password | text | bcrypt via pgcrypto |
| api_key | uuid | |
| confirmed | boolean | default false |
| created_at | timestamp | |
| updated_at | timestamp | |

**user_state**

| Column | Type | Notes |
|--------|------|-------|
| user_id | bigint | PK, FK → users |
| nickname | text | auto-generated |
| exp | bigint | experience points |
| flags | bigint | feature flags |
| lives | bigint | |
| created_at | timestamp | |
| updated_at | timestamp | |

---

## Configuration Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--server.addr` | `127.0.0.1` | Bind address |
| `--server.port` | `9666` | HTTP port |
| `--server.x_api_token` | | API key for requests |
| `--pg.host` | `127.0.0.1` | PostgreSQL host |
| `--pg.port` | `5432` | PostgreSQL port |
| `--pg.user` | | PostgreSQL user |
| `--pg.database` | | PostgreSQL database |
| `--pg.password` | | PostgreSQL password |
| `--pg.max_pool_size` | `10` | Max connection pool size |
| `--pg.ssl` | `disable` | PostgreSQL SSL mode |
| `--redis.addr` | `127.0.0.1:6379` | Redis address |
| `--redis.username` | | Redis username |
| `--redis.password` | | Redis password |
| `--redis.db_num` | `0` | Redis database number |
| `--redis.expiration` | `30s` | Session expiration duration |
| `--centrifugo.api_url` | | Centrifugo HTTP API URL |
| `--centrifugo.api_key` | | Centrifugo API key |
| `--centrifugo.token_hmac_secret_key` | | Secret for signing Centrifugo tokens |
| `MIGRATION_CONN_STRING` | | PostgreSQL DSN for migrations (env var) |

## License

[Apache 2.0](LICENSE)
