# Architecture — C4 Model

## Level 1 — System Context

```mermaid
C4Context
  title System Context — Balda

  Person(player, "Player", "Plays Balda word game via web browser")

  System(balda, "Balda", "Word game server: manages sessions, matchmaking, and real-time gameplay")

  Rel(player, balda, "Signs up, authenticates, creates/joins games, makes moves", "HTTPS / WebSocket")
```

---

## Level 2 — Container

```mermaid
C4Container
  title Container Diagram — Balda

  Person(player, "Player", "Web browser")

  System_Boundary(balda, "Balda") {
    Container(frontend, "Frontend", "HTML/JS/Protobuf", "Static single-page app served from web-assets/")
    Container(api, "Go HTTP Server", "Go, ogen", "Handles REST API: signup, auth, game lifecycle, moves")
    ContainerDb(postgres, "PostgreSQL", "PostgreSQL 15", "Stores users, player_state; migrations via tern")
    ContainerDb(redis, "Redis", "Redis 7", "Stores sessions (TTL 30s)")
    Container(centrifugo, "Centrifugo", "Centrifugo 5", "Real-time pub/sub: lobby and game channels")
  }

  Rel(player, frontend, "Opens", "HTTPS")
  Rel(frontend, api, "REST calls", "HTTPS / JSON")
  Rel(frontend, centrifugo, "Subscribes to channels", "WebSocket")
  Rel(api, postgres, "Reads/writes user and player data", "pgx/v5")
  Rel(api, redis, "Stores/reads sessions", "go-redis/v9")
  Rel(api, centrifugo, "Publishes game events", "HTTP API")
```

---

## Level 3 — Component

```mermaid
C4Component
  title Component Diagram — Go HTTP Server

  Person(player, "Player")
  ContainerDb(postgres, "PostgreSQL")
  ContainerDb(redis, "Redis")
  Container(centrifugo, "Centrifugo")

  Container_Boundary(api, "Go HTTP Server") {
    Component(ogen, "ogen Router", "internal/server/ogen", "Generated HTTP router and security middleware (X-API-Key / api_key)")
    Component(handlers, "HTTP Handlers", "internal/server/restapi/handlers", "signup, auth, create/join/list game, move, skip, ping")
    Component(svc, "Balda Service", "internal/service", "Orchestrates lobby, matchmaking, storage, and real-time publishing")
    Component(lobby, "Lobby", "internal/lobby", "In-memory registry of active games; starts game.Run goroutine on join")
    Component(mm, "Matchmaking Queue", "internal/matchmaking", "Rating-window pairing; expands window every 10s")
    Component(sess, "Session Service", "internal/session", "Creates and validates Redis-backed sessions (TTL 30s)")
    Component(storage, "Storage", "internal/storage", "Thin pgxpool wrapper for PostgreSQL queries")
    Component(gamecoord, "Game Coordinator", "internal/gamecoord", "Implements game.Notifier; bridges FSM events to Centrifugo channels")
    Component(cfclient, "Centrifugo Client", "internal/centrifugo", "HTTP publish client; generates JWT tokens for connection/subscription")
  }

  Rel(player, ogen, "HTTP requests", "HTTPS / JSON")
  Rel(ogen, handlers, "Routes to")
  Rel(handlers, svc, "Delegates domain logic")
  Rel(handlers, sess, "Validates session")
  Rel(handlers, cfclient, "Publishes lobby_update")
  Rel(svc, lobby, "Creates/joins/queries games")
  Rel(svc, mm, "Enqueues players")
  Rel(svc, storage, "Reads player UUIDs")
  Rel(lobby, gamecoord, "Passes as game.Notifier")
  Rel(gamecoord, cfclient, "Publishes turn_change, game_state, game_over, skip_warn")
  Rel(storage, postgres, "SQL queries", "pgx/v5")
  Rel(sess, redis, "GET/SET session keys", "go-redis/v9")
  Rel(cfclient, centrifugo, "POST /publish", "HTTP + apikey")
```

---

## Level 4 — Code

Key structs and interfaces inside the `internal/game` package.

```mermaid
classDiagram
  class Game {
    -players []*Player
    -board *LettersTable
    -state GameState
    -current int
    -turn *Turn
    -eventCh chan TurnEvent
    -notifier Notifier
    +Run(ctx)
    +SubmitWord(playerID, newLetter, word) error
    +Skip(playerID) error
    +AckTimeout()
    +Kick()
    +Board() *LettersTable
    +PlayerScores() []PlayerState
    +CurrentPlayerID() string
    +Done() chan struct{}
  }

  class Player {
    +ID string
    +Exp int
    +Score int
    +Words []string
    +ConsecutiveTimeouts int
    +ConsecutiveSkips int
    +Kicked bool
  }

  class LettersTable {
    +Table [5][5]*Letter
    +NewLettersTable(word) (*LettersTable, error)
    +PutLetterOnTable(l) error
    +IsFull() bool
    +AsStrings() [5][5]string
    +InitialWord() string
  }

  class Letter {
    +RowID uint8
    +ColID uint8
    +Char string
  }

  class Notifier {
    <<interface>>
    +NotifyTurnStart(playerID)
    +NotifyTimeout(playerID, consecutive, willKick)
    +NotifySkip(playerID, consecutive, willEnd)
    +NotifyKick(playerID)
    +NotifyBoardFull()
  }

  class GameState {
    <<enumeration>>
    StateWaitingForMove
    StatePlayerTimedOut
    StateGameOver
  }

  class TurnEvent {
    <<enumeration>>
    EventMoveSubmitted
    EventTurnSkipped
    EventTurnTimeout
    EventAckTimeout
    EventKick
    EventBoardFull
  }

  class Dictionary {
    +Definition map[string]string
    +RandomFiveLetterWord() string
  }

  Game "1" --> "2" Player : manages
  Game "1" --> "1" LettersTable : owns
  Game ..> Notifier : uses
  Game --> GameState : tracks
  Game ..> TurnEvent : dispatches
  LettersTable "1" --> "0..25" Letter : contains
  Game ..> Dictionary : validates words
```
