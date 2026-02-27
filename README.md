# Balda

A multiplayer turn-based word game server written in Go. Players compete on a 5×5 letter grid, placing letters to form valid Russian words and score points.

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
| REST API | [go-swagger](https://github.com/go-swagger/go-swagger) (code-generated from OpenAPI spec) |
| CLI | [cobra](https://github.com/spf13/cobra) |
| Database | PostgreSQL 15 (pgx/v5 driver) |
| Runtime image | Debian 13 (trixie-slim) |
| Session store | Redis 6.2 |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Logging | [zerolog](https://github.com/rs/zerolog) |
| IDs | ULID (games), UUID (sessions) |

## Project Structure

```
balda/
├── cmd/                  # CLI entry points (server, migrate)
├── internal/
│   ├── game/             # Core game logic, FSM, letter table, dictionary
│   ├── server/
│   │   ├── restapi/      # go-swagger generated infrastructure
│   │   ├── handlers/     # HTTP request handlers
│   │   └── models/       # Data transfer objects
│   ├── session/          # Redis-backed session management
│   ├── storage/          # Storage abstraction
│   ├── flname/           # Auto-generated player nicknames
│   └── rnd/              # RNG utilities
├── api/swagger/          # OpenAPI specification
├── migrations/           # SQL migration files
├── main.go
├── Makefile
└── docker-compose.yml
```

## API

Base path: `/balda/api/v1`

Authentication uses an `X-API-Key` header (or `api_key` query parameter).

| Method | Path | Description |
|--------|------|-------------|
| POST | `/signup` | Register a new user account |
| POST | `/auth` | Authenticate and get a session |
| GET | `/users/state/{uid}` | Get user profile and state |

### POST /signup

```json
// Request
{
  "firstname": "Ivan",
  "lastname": "Petrov",
  "email": "ivan@example.com",
  "password": "secret"
}

// Response
{
  "uid": 1,
  "firstname": "Ivan",
  "lastname": "Petrov",
  "sid": "<session-uuid>",
  "key": "<api-key-uuid>"
}
```

### POST /auth

```json
// Request
{
  "email": "ivan@example.com",
  "password": "secret"
}

// Response: same User object as signup
```

### GET /users/state/{uid}

```json
// Response
{
  "uid": 1,
  "nickname": "BlackShearCougar",
  "exp": 0,
  "flags": 0,
  "lives": 0,
  "game_id": 0
}
```

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

### Placement Rules

- New letters must be placed adjacent (horizontally or vertically) to an existing letter.
- Rows 0–1: the cell directly below must already contain a letter.
- Rows 3–4: the cell directly above must already contain a letter.
- Row 2 (center): no additional adjacency constraint.

### Turn

- Each player has **60 seconds** per turn.
- After **3 consecutive timeouts**, the player is kicked and the game ends.
- A player can skip a turn voluntarily.

### Word Validation

Submitted words must:
- Be **3 or more letters** long
- Include the newly placed letter
- Consist of letters traceable on the board
- Exist in the embedded Russian nouns dictionary
- Not have been submitted before in this game

### State Machine

```
PAUSED → STARTED → WaitTurn ↔ NextTurn → PlaceKick (game over)
```

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

## Getting Started

### Prerequisites

- Go 1.26+
- Docker and Docker Compose

### Run with Docker Compose

```bash
docker-compose up
```

This starts PostgreSQL, Redis, and the game server on port `9666`.

### Build and Run Manually

```bash
# Build
make build

# Run migrations
./bin/balda migrate up \
  --pg.host localhost --pg.port 5432 \
  --pg.user balda --pg.database balda --pg.password password

# Start server
./bin/balda server \
  --server.addr 0.0.0.0 \
  --server.port 9666 \
  --server.x_api_token your-api-token \
  --pg.host localhost --pg.port 5432 \
  --pg.user balda --pg.database balda --pg.password password \
  --redis.addr localhost:6379
```

All flags can also be set via environment variables (e.g., `SERVER_ADDR`, `PG_HOST`, `REDIS_ADDR`).

### Regenerate API Code

```bash
make code-gen
```

This regenerates Go server stubs from [api/swagger/http-api.yaml](api/swagger/http-api.yaml).

### Run Tests

```bash
make test
```

## Configuration Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--server.addr` | `127.0.0.1` | Bind address |
| `--server.port` | `9666` | HTTP port |
| `--server.x_api_token` | | API key for requests |
| `--pg.host` | `localhost` | PostgreSQL host |
| `--pg.port` | `5432` | PostgreSQL port |
| `--pg.user` | | PostgreSQL user |
| `--pg.database` | | PostgreSQL database |
| `--pg.password` | | PostgreSQL password |
| `--redis.addr` | `127.0.0.1:6379` | Redis address |

## License

[Apache 2.0](LICENSE)
