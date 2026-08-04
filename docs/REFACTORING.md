# План рефакторинга по итогам стайл-ревью

Документ собран по результатам ревью Go-кодовой базы (5 параллельных независимых
областей: control flow, function design, объявления, строки/длины, организация кода).
Охват: `cmd/`, `internal/`, `tests/` (кроме `vendor/` и сгенерированного `internal/server/ogen/`).
Ничего не исправлено — только находки.

**Общий вердикт:** база в хорошем состоянии — ранние возвраты, `%w` в ошибках,
нет `reflect`/`[]any`/dot imports, `range` вместо index-циклов, единообразный
маппинг ошибок в хендлерах. Проблемы концентрируются в нескольких «толстых»
местах и мелком стиле.

---

## 🔴 Системные (cross-cutting)

### R1. Жирные сигнатуры → options struct
- `internal/server/restapi/handlers/handlers.go:29` — `New(...)` 7 параметров.
- `internal/storage/user.go:158` — `CreateUser(ctx, firstname, lastname, email, password, nickname)` — 6 параметров (5 строк подряд), сигнатура дублируется в `service/balda_service.go`.
- `internal/storage/token.go:25` — `SaveRefreshToken` 6 параметров (тип `RefreshToken` уже существует, но как параметр не используется).
- `internal/server/restapi/handlers/handlers.go:45` — `issueTokens` 6 параметров, зовётся с заглушками `""`.

### R2. Толстые функции
- `cmd/server.go:111` — `RunE` ~210 строк: миграции, пулы, фабрика игр, matchmaking, HTTP-сервер, graceful shutdown в одном замыкании.
- `internal/storage/game_result.go:81` — `SaveGameResultWithAchievements` ~150 строк (INSERT + ELO + счётчики + достижения); ELO-блок просится в хелпер (заодно снимет вложенность 4 уровня, см. C1).
- `internal/server/restapi/handlers/join_game.go:18` — `JoinGame` ~123 строки.
- `internal/matchmaking/matchmaking.go:168` — `Tick` ~90 строк (expiry-фаза + matching-фаза в одном).
- `internal/game/game.go:382` — `SubmitWord` ~75 строк с ~10 ручными `g.mu.Unlock()`.
- `internal/game/bot/engine.go:141` — `findWordFrom` ~72 строки с рекурсивным `dfs`-замыканием.

### R3. Дублирование
- Сборка `EvGameOver` — трижды в `internal/gamecoord/coord.go` (166/319/354).
- `join_game.go` и `create_game_with_bot.go` — почти дословные копии; инлайнят конвертацию доски, хотя есть `boardToSlice` (`move_game.go:120`).
- Проверка статуса в `lobby.Join` повторена дважды (`internal/lobby/lobby.go:112` и `:149`).

### R4. Nil-слайсы → `null` в JSON
- `internal/storage/leaderboard.go:52` — пустая выборка nil'ом; `internal/leaderboard/service.go:85` кладёт `null` в Redis-кэш (хендлер страхует, но кэш хранит `null` вместо `[]`).
- Тот же паттерн (внутренние потребители, некритично): `internal/storage/achievements.go:25`, `internal/storage/game_result.go:145`.

---

## 🟡 Точечные

### C1. Control flow
- Счастливый путь вложен: `internal/storage/game_result.go:110`, `internal/gamecoord/coord.go:179` (`if !accepted { return }` выровняет ~30 строк), `internal/server/restapi/handlers/auth_telegram.go:42`.
- Вложенность 4 уровня: `internal/game/bot/notifier.go:111`.
- Условие 4 операнда без именованных булевых: `internal/game/game.go:342`.
- else-if вместо switch: `internal/gamecoord/coord.go:64`, `internal/server/restapi/handlers/auth.go:110`, `internal/lobby/lobby.go:112`.
- else после guard: `internal/matchmaking/matchmaking.go:236`, `internal/service/balda_service.go:261`.
- `internal/game/dictionary.go:68` — ошибка открытия `custom_words.json` молча проглатывается (стоит `slog.Warn` при не-NotFound).

### C2. Организация файлов
- stdlib-импорты в отдельной группе: `cmd/server.go:35`, `internal/session/session.go:13`.
- `internal/game/fsm.go` — константы `State*`/`Event*` после методов `String()`.
- `internal/game/game.go` — 8 типов в файле; `Player` просится в `player.go`, `NoopNotifier` в `notifier.go`; конструктор и `PlayerState` посреди файла.
- `ErrPlayerInGame` объявлен посреди `internal/service/balda_service.go` (ошибки должны быть в одном месте).
- Хелперы вклинены между методами: `internal/gamecoord/coord.go:36,146,224`.

### C3. Объявления переменных
- `:=` для zero-value: `internal/server/restapi/handlers/join_game.go:103`, `internal/storage/player_stats.go:97-98`, `internal/gamecoord/coord.go:225`, `internal/game/game.go:406,562`.
- `internal/game/game.go:39` — `var OfflineGraceDuration` там, где достаточно `const`.

### C4. Values
- `game.Letter` (~24 байта) повсюду по указателю: `internal/game/table.go:90`, `internal/game/game.go:382`, `internal/game/bot/engine.go:48`; доска хранит `[5][5]*Letter`. Переезд на value semantics — большой, затрагивает много сигнатур.

### C5. Мёртвый код
- `internal/achievements/achievements.go:55` — тип `Unlock` не используется.
- `internal/achievements/achievements.go:122` — `Reload` — неиспользуемая обёртка над `Load`.
- `internal/achievements/achievements.go:200` — `WordLength` экспортирован, но используется только в своём тесте.
- `internal/session/` — в проде используется только `session.Config`; `session.Service` фактически мёртв.

### C6. Строки и мелочи
- `%s` вместо `%q` для идентификаторов в ошибках: `internal/storage/game_result.go:126,154,166,181,215`.
- `fmt.Sprintf("%s:%d")` → `net.JoinHostPort`: `cmd/server.go:274`.
- Конкатенация строк в цикле: `internal/game/game.go:117` (`MakeWord`), `internal/game/table.go:54` (`InitialWord`) — слова короткие, некритично.
- Длинные строки/вызовы 4+ аргументов в одну строку: `internal/storage/achievements.go:28`, `internal/service/balda_service.go:204,243`, `internal/gamecoord/coord.go:284`, `cmd/server.go:253,367,374`.
- Таблицы слов одной строкой по ~5К символов: `internal/flname/flname.go:9` (данные оправдывают, но диффы шумные).

### C7. Излишние экспорты
- `internal/game/game.go:61` — тип `Turn` используется только внутри пакета.
- `internal/game/game.go:131` — `HasDuplicateCells` вызывается только из `SubmitWord`.

---

## Приоритизация

### Этап 1 — быстрые выигрыши (~полдня) ✅ ВЫПОЛНЕН (ветка refactor/stage1-cleanup)
~~C3, C5, C6 (кроме flname), switch-замены из C1 (`coord.go:64`, `lobby.go:112`, `auth.go:110`), `slog.Warn` в dictionary, R4 (инициализация слайсов `[]T{}` в storage)~~

Сделано: R4 (пустые слайсы в storage), C3 (var для zero-value; `OfflineGraceDuration`
оставлен var — тесты его переопределяют), C5 (мёртвый код achievements + legacy
session.Service удалён, −195 строк), C6 (%q, JoinHostPort, strings.Builder),
C1-три switch (coord/lobby/auth buildActiveGame), slog.Warn в dictionary.

### Этап 2 — средние
R1 (options struct для `handlers.New`, `CreateUser`, `SaveRefreshToken`), вынос ELO-хелпера из `SaveGameResultWithAchievements`, R3 (дедупликация `EvGameOver` и `boardToSlice`, статус-switch в `lobby.Join`), C2 (порядок импортов/файлов, ошибки в одном месте).

### Этап 3 — крупные, отдельными задачами с планом
R2 (разбиение `RunE`, `Tick`, `SubmitWord` с defer-unlock), C4 (`Letter` на value semantics — массовое изменение сигнатур), C7 (unexport с проверкой всех вызовов).

## Правила при рефакторинге

- Не менять поведение: после каждого этапа `make test` и `make lint` зелёные.
- Тесты не трогаем, кроме случаев смены сигнатур (тогда — механическая правка вызовов).
- Одна тема — одна ветка/PR; «заодно почистить» запрещено.
