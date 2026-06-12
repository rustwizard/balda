# Боты для Balda

> Статус: MVP реализовано. Бот живёт внутри процесса сервера, ходит через `game.Game`, а фронт видит его как обычного соперника.

## Варианты архитектуры

| Подход | Суть | Плюсы | Минусы |
|--------|------|-------|--------|
| **A. Server-side BotPlayer** | Бот живёт внутри процесса сервера, ходит напрямую через `rec.Game.SubmitWord()` | Мгновенные ходы, нет HTTP оверхеда, нет нужды в сессиях | Нужно менять `Lobby` и `cmd/server.go` |
| **B. HTTP Bot Client** | Отдельный процесс/горутина ходит через REST API как обычный игрок | Не трогает лобби, бот = полноценный клиент | Нужна сессия в Redis, player_state в БД, задержки сети |
| **C. Гибрид: BotFacade** | В лобби бот выглядит как `game.Player`, но ходит через внутренний сервис, а для фронта — через Centrifugo | Чистый API, фронт не замечает разницы | Средняя сложность |

**Рекомендация:** Вариант **A (Server-side BotPlayer)** для MVP, потому что:
- Игра Балда — пошаговая, боту нужен доступ к доске и словарю
- HTTP-бот понадобится отдельный процесс, сложнее деплоя
- `Game` engine уже thread-safe и публичен

---

## Детальный план: Вариант A

### 1. Добавить признак бота в доменную модель

**`internal/game/player.go`** (новый файл):
```go
type PlayerType int

const (
    PlayerTypeHuman PlayerType = iota
    PlayerTypeBot
)

type Player struct {
    ID                  string
    Exp                 int
    Score               int
    Words               []string
    ConsecutiveTimeouts int
    ConsecutiveSkips    int
    Kicked              bool
    Type                PlayerType // новое
}
```

### 2. Создать `internal/game/bot/` — движок бота

**Интерфейс:**
```go
type Strategy interface {
    // Выбирает ход: букву и путь до слова
    // Возвращает ошибку если ход невозможен
    MakeMove(board [5][5]string, usedWords []string) (*Letter, []Letter, error)
}
```

**Реализации стратегий:**
- `RandomValidStrategy` — находит первое подходящее слово из словаря (MVP)
- `GreedyStrategy` — выбирает слово с максимальным количеством букв
- `MinimaxStrategy` — оценивает позицию (будущее)

**`internal/game/bot/engine.go`:**
```go
type Engine struct {
    dict     *game.Dictionary
    strategy Strategy
}
```

Алгоритм `MakeMove` для `RandomValidStrategy`:
1. Получить все пустые клетки доски
2. Для каждой пустой клетки попробовать каждую букву алфавита
3. Для каждой буквы найти все пути длиной ≥ 3, начинающиеся/проходящие через эту клетку (DFS/BFS по соседним клеткам)
4. Проверить, есть ли слово в `game.Dict` и не использовано ли ранее
5. Вернуть первое найденное

> **Оптимизация:** Предвычислить префиксное дерево (trie) из словаря, чтобы отсекать невалидные пути на лету. Но для MVP можно brute-force с таймаутом.

### 3. Создать `internal/game/bot/notifier.go`

```go
type BotNotifier struct {
    engine *bot.Engine
    game   *game.Game
    // канал для передачи ходов обратно в игру
}

func (b *BotNotifier) NotifyTurnStart(playerID string) {
    if playerID != b.botPlayerID {
        return // не наш ход
    }
    // Делаем ход с небольшой задержкой (500-1500 мс), чтобы выглядело естественно
    go func() {
        time.Sleep(b.thinkingTime)
        letter, path, err := b.engine.MakeMove(b.game.BoardSnapshot(), b.game.UsedWords())
        if err != nil {
            // Нет ходов — скипаем
            b.game.Skip(b.botPlayerID)
            return
        }
        b.game.SubmitWord(b.botPlayerID, letter, path)
    }()
}
```

### 4. Модифицировать `Lobby` для поддержки ботов

Добавить метод `CreateWithBot(humanPlayer *game.Player, bot *game.Player) (*GameRecord, error)` или расширить `Create` параметром.

Либо лучше — добавить `AddBot(gameID string, bot *game.Player) error`:
- Проверяет, что игра в статусе `waiting`
- Добавляет бота в `rec.Players`
- Сразу запускает игру (вызывает фабрику)
- Возвращает `GameRecord`

**В `cmd/server.go` фабрика** должна уметь создавать `gamecoord` для человека и `bot.Notifier` для бота. Но сейчас фабрика принимает один `Notifier` на игру.

Варианты:
- **A1.** Сделать `Notifier` composite — `MultiNotifier []Notifier`, который рассылает всем подписчикам
- **A2.** Сделать `BotNotifier` обёрткой вокруг `gamecoord.Coordinator`, которая перехватывает `NotifyTurnStart` для бота, а для человека проксирует дальше

```go
type CompositeNotifier struct {
    human *gamecoord.Coordinator
    bot   *bot.Notifier
}

func (c *CompositeNotifier) NotifyTurnStart(playerID string) {
    c.human.NotifyTurnStart(playerID)
    c.bot.NotifyTurnStart(playerID)
}
// ... остальные методы аналогично
```

### 5. API для создания игры с ботом

**Вариант 1: Автоподмена** — если игра ждёт соперника > 30 секунд, автоматически добавлять бота. Нужен фоновый worker.

**Вариант 2: Явный режим** — `POST /games/with-bot` создаёт игру сразу с ботом.

Рекомендую Вариант 2 для MVP.

**`api/openapi/http-api.yaml`:**
```yaml
/games/with-bot:
  post:
    summary: Create a game against a bot
    operationId: createGameWithBot
    # security, responses аналогично CreateGame
```

**Handler (`internal/server/restapi/handlers/create_game_with_bot.go`):**
```go
func (h *Handlers) CreateGameWithBot(ctx context.Context, params baldaapi.CreateGameWithBotParams) (baldaapi.CreateGameWithBotRes, error) {
    uid, err := h.sess.GetUID(params.XAPISession)
    // ...
    rec, err := h.svc.CreateGameWithBot(ctx, uid)
    // возвращаем board snapshot + game_token (как в JoinGame)
}
```

**Service (`internal/service/balda_service.go`):**
```go
func (b *Balda) CreateGameWithBot(ctx context.Context, userID int64) (*lobby.GameRecord, error) {
    pg, err := b.strg.GetPlayerByUID(ctx, userID)
    // ...
    human := &game.Player{ID: pg.PlayerID.String(), Exp: pg.Exp}
    botPlayer := &game.Player{
        ID:   uuid.New().String(),
        Exp:  0,
        Type: game.PlayerTypeBot,
    }
    rec, err := b.lobby.CreateWithBot(human, botPlayer)
    // ...
}
```

### 6. Доработка `Lobby.CreateWithBot`

```go
func (l *Lobby) CreateWithBot(human, bot *game.Player) (*GameRecord, error) {
    // 1. Создаём GameRecord с обоими игроками
    // 2. Сразу вызываем factory и запускаем g.Run()
    // 3. Не ждём Join
}
```

### 7. Словарь и board для бота

Боту нужен доступ к `game.Dict`. Он уже глобальный (`var Dict Dictionary`), так что бот может читать напрямую.

Нужен метод `Game.UsedWords()` — сейчас такого нет. Можно добавить:
```go
func (g *Game) UsedWords() []string {
    g.mu.Lock()
    defer g.mu.Unlock()
    var out []string
    for _, p := range g.players {
        out = append(out, p.Words...)
    }
    return out
}
```

### 8. Фронтенд

Нужно добавить кнопку «Играть с ботом» в лобби.

**`frontend/src/components/LobbyScreen.svelte`:**
```svelte
<button onclick={handleCreateGameWithBot}>
  🤖 Играть с ботом
</button>
```

API-вызов аналогичен `createGame`, но endpoint другой. После создания фронт получит `game_started` (или сразу `game_state`), так как игра запустится мгновенно.

### 9. Таймауты и graceful degradation бота

- Бот не должен блокировать `g.Run()` — все вычисления в отдельной горутине
- Если бот не может найти ход за 5 секунд — делает `Skip`
- Если бот проиграл/выйграл — `onDone` лобби удаляет игру как обычно

---

## Этапы реализации (приоритеты)

### Этап 1: MVP (1-2 дня)
1. Добавить `PlayerType` в `game.Player`
2. Создать `internal/game/bot/` с `RandomValidStrategy` и `BotNotifier`
3. Добавить `Game.UsedWords()`
4. Сделать `CompositeNotifier`
5. Добавить `Lobby.CreateWithBot` и фабрику
6. Добавить `POST /games/with-bot` (без OpenAPI code-gen, ручной handler)
7. Кнопка на фронте

### Этап 2: Умный бот (2-3 дня)
1. Построить trie из `Dict` для быстрого поиска
2. DFS с отсечением по префиксам
3. `GreedyStrategy` — выбирать слово максимальной длины
4. Добавить `thinkingTime` (500-1500 мс) с вариацией

### Этап 3: Инфраструктура (1 день)
1. Добавить в OpenAPI схему `POST /games/with-bot`
2. `make code-gen`
3. Тесты: `bot_test.go`, интеграционный тест «человек vs бот»
4. README: описать режим игры с ботом

---

## Вопросы для обсуждения перед началом

1. **Какой уровень сложности бота нужен?** Случайный валидный ход (MVP) или стратегия с оценкой позиции?
2. **Какой способ запуска?** Явная кнопка «Играть с ботом» или автозамена отсутствующего соперника?
3. **Бот как постоянный player_state?** Нужна ли статистика игр с ботом (запись в `game_results`)?
