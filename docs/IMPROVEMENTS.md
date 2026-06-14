# Предложения по улучшению Balda

Документ собран по итогам ревью кодовой базы. Пункты сгруппированы по статусу и приоритету.
Для каждого указаны затронутые файлы, суть проблемы и предлагаемое решение.

---

## ✅ Реализовано

### 1. Игра завершается естественным образом
**Файлы:** `internal/game/fsm.go`, `internal/game/game.go`

`EventGameFinished` заменяет `EventBoardFull` как единое событие естественного
завершения (доска заполнена или все игроки подряд сделали skip). Победитель при любом
естественном конце определяется по `Score` (в `gamecoord.findWinnerByScore`).

### 2. Graceful shutdown и таймауты HTTP-сервера
**Файлы:** `cmd/root.go`, `cmd/server.go`

`Execute()` заменён на `ExecuteContext()` с `signal.NotifyContext(SIGINT/SIGTERM)`.
HTTP-сервер создан явно как `&http.Server{...}` с `ReadTimeout`, `WriteTimeout`, `IdleTimeout`
и `ReadHeaderTimeout`. Сервер ожидает `<-cmd.Context().Done()`, после чего вызывается
`lby.Shutdown()` (отмена контекста всех игр) и `httpSrv.Shutdown(ctx)` с таймаутом 30s.
Дожидаются завершения `pendingResults` (сохранение результатов игр).

### 3. `Player.Exp` загружается из БД
**Файлы:** `internal/service/balda_service.go`

`CreateGame` и `JoinGame` загружают `exp` через `storage.GetPlayerByUID`
(`SELECT player_id, COALESCE(exp, 0) FROM player_state`) и передают в `game.Player{Exp: p.Exp}`.

### 4. Результаты игр сохраняются
**Файлы:** `internal/gamecoord/coord.go`, `internal/storage/game_result.go`

`gamecoord.dispatchGameResult` вызывает `onGameOver` callback до публикации в Centrifugo.
`storage.SaveGameResult` в транзакции: (1) INSERT INTO `game_results`, (2) INSERT INTO
`game_result_players` (score, words_count, exp_gained), (3) UPDATE `player_state SET exp = exp + $1`.
Callback обёрнут в retry (3 попытки, backoff 100/200 мс) и учитывается в `pendingResults` WaitGroup.

### 5. Удалён мёртвый код в game.go
**Файл:** `internal/game/game.go`

Удалены `AddWordToCurrentPlayer` и `IsTakenWord`. Проверка дубликатов в `SubmitWord`
оставлена как `slices.Contains`.

### 6. Исправлен гомоглиф в имени метода
**Файл:** `internal/game/game.go` — `CheckWordExistence`

Кириллическая `С` (U+0421) заменена на латинскую `C` (U+0043). PR #102.

### 7. SQL-запросы вынесены в storage-слой
**Файлы:** `internal/service/balda_service.go`, `internal/server/restapi/handlers/`, `internal/storage/`

Добавлены методы `storage.AuthUser`, `storage.CreateUser`, `storage.GetPlayerState`,
`storage.GetPlayerByUID`. `service.Balda` лишился метода `DB()` — хендлеры больше не трогают
`Pool()` напрямую.

### 8. Утечка кредов в логи устранена
**Файл:** `internal/server/restapi/handlers/handlers.go`

Убрано логирование значения токена, удалён шумный `slog.Info("KeyAuth ... handler called")`.
Валидация токена вынесена в `storage.ValidateAPIKey` — проверка UUID формата + запрос
`SELECT EXISTS` в БД (см. пункт 9).

### 9. Единый статический API-токен заменён на персональные ключи
**Файлы:** `cmd/server.go`, `internal/server/restapi/handlers/handlers.go`,
`internal/storage/user.go`, `internal/service/balda_service.go`

Глобальный `xAPIToken` из конфига удалён. `HandleAPIKeyHeader` и `HandleAPIKeyQueryParam`
теперь вызывают `storage.ValidateAPIKey`, который проверяет, существует ли переданный UUID
в колонке `api_key` таблицы `users`. Каждый пользователь получает свой ключ при регистрации
(`POST /signup → Key`). Не-UUID формат быстро отклоняется без обращения к БД.

### 10. Проверка минимальной длины слова (≥3)
**Файл:** `internal/game/game.go` (`SubmitWord`)

Добавлен `ErrWordTooShort` и проверка `len(word) < 3`. Тесты, использовавшие 2-буквенные
пути, обновлены до 3-буквенных.

### 11. Разделение аутентификации и игрового присутствия
**Файлы:** `internal/session/`, `internal/presence/`, `handlers/ping.go`

Авторизационная сессия (TTL 24h) и игровое присутствие разделены. Новый пакет `internal/presence/`
ведёт ключи `presence:{uid}` в Redis с TTL 30s. `POST /session/ping` обновляет только presence.
Фронт шлёт пинг каждые 5s — запас в 6× относительно TTL.

### 12. Логическая ошибка в auth.go исправлена
**Файл:** `internal/server/restapi/handlers/auth.go`

Ошибка БД и неверный пароль теперь различаются: `errors.Is(err, pgx.ErrNoRows)` → 401,
любая другая ошибка → 500. Логирование `slog.Info("auth handler called")` добавлено,
но без утечки чувствительных данных.

---

## 🔴 Критические (корректность, конкурентность, безопасность)

_Все критические задачи решены._

---

## 🟠 Высокий приоритет (незавершённый функционал, мёртвый код)

### 13. Matchmaking подключён, но не работает
**Файлы:** `cmd/server.go`, `internal/service/balda_service.go`, `internal/matchmaking/`

`matchmaking.Queue` создаётся, но `mm.Run(ctx)` нигде в проде не вызывается, и ни один хендлер
не делает `mm.Enqueue`. В продакшене очередь — мёртвый код (используется только в тестах).

**Предложение:** либо подключить (эндпоинт постановки в очередь + `go mm.Run(serverCtx)`),
либо убрать из прод-пути до готовности фичи.

---

## 🟡 Средний приоритет (надёжность, дизайн, безопасность)

### 14. Дублирование и рассинхрон публикаций game_state
**Файлы:** `internal/server/restapi/handlers/move_game.go`, `internal/gamecoord/coord.go`

И хендлер `MoveGame`, и `gamecoord` (на `NotifyTurnStart`) публикуют `game_state` на каждый ход.
Хендлер при этом сам «угадывает» следующего игрока (`nextPlayerID`), дублируя логику FSM, что
может рассинхронизироваться с реальным состоянием игры (особенно при скипах/таймаутах).

**Предложение:** оставить единственный источник истины публикаций (gamecoord), хендлер вернёт
синхронный ответ без второй публикации.

---

## 🟢 Низкий приоритет (чистота, инфраструктура, тесты)

### 15. CI без линтера и без -race
**Файл:** `.github/workflows/ci.yml`

Есть `.golangci.yml`, но в CI нет шага `golangci-lint`. Тесты гоняются без `-race` (критично
для конкурентного игрового сервера). Шаг `Install swagger` (go-swagger) устарел — code-gen
использует ogen.

**Предложение:** добавить `golangci-lint run`, `go test -race`, убрать ненужный go-swagger.

### 16. ✅ Артефакт coverage.out в репозитории
**Файл:** `coverage.out`

Закоммичен отчёт покрытия.

**Предложение:** удалить из git и добавить в `.gitignore`.

### 17. ✅ Мелочи в storage
**Файл:** `internal/storage/storage.go`

`func (b Balda) Pool()` — value receiver копирует структуру. консистентность ресиверов

**Предложение:** перейти на pointer receiver.

### 18. ✅ centrifugo.Client — таймаут задан
**Файл:** `internal/centrifugo/client.go`

`http.Client{}` заменён на `http.Client{Timeout: 10 * time.Second}`.

### 19. ✅ Dictionary.FiveLetters — map вместо slice
**Файл:** `internal/game/dictionary.go`

`map[int]string` с последовательным счётчиком-ключом — фактически срез. Лишний оверхед.

**Предложение:** заменить на `[]string`.

### 20. 🔶 Горутины публикаций в gamecoord (частично)
**Файл:** `internal/gamecoord/coord.go`

Каждое событие порождает `go c.publish...`. `go` здесь намеренный: `Notify*` вызываются из
горутины `game.Run` под `g.mu`, и синхронная публикация (сетевой вызов до 3s) заблокировала бы
FSM и лок.

**Сделано:** `NotifyTurnStart` публикует `turn_change` и `game_state` в одной горутине
последовательно — `turn_change` гарантированно раньше `game_state` (клиенты на это
рассчитывают), при этом I/O по-прежнему вне `g.mu`.

**Осталось (низкий приоритет):** глобального ограничения числа горутин нет. На игру событий
мало (ходы раз в ~60s), пик ≈ активные игры × константа — не runaway. Полное решение —
один publisher-горутина на игру с буферизованным каналом (FIFO + 1 горутина/игра), но это
требует лайфцикла (флаш `game_over` перед выходом `Run`); отложено как несрочное.

### 21. Игровая логика не использует presence для ускоренного таймаута
**Файлы:** `internal/gamecoord/coord.go`, `internal/lobby/lobby.go`, `cmd/server.go`

При отключении игрока (presence-ключ истёк) сервер по-прежнему ждёт полных 60s таймаута хода,
а не реагирует сразу.

**Предложение:** пробросить `*presence.Service` в фабрику игр (`cmd/server.go`) и при старте
хода запускать сокращённый grace-таймер (например, 5s) если `presence.IsOnline` вернул false.

---

## 🆕 Игровые механики (новый функционал)

Потенциальные механики для расширения геймплея. Сгруппированы по темам, внутри — по приоритету.

### Режимы игры

### 22. Боты (P0 — план уже готов, docs/bots.md)
**Затрагивает:** `internal/game/bot/` (новый), `internal/lobby/`, `internal/game/`, `cmd/server.go`, фронт

Три варианта стратегии: `RandomValidStrategy` (MVP), `GreedyStrategy` (макс. длина слова),
`MinimaxStrategy` (оценка позиции). Добавить `PlayerType` в `Player`, `CompositeNotifier` для
раздельной нотификации человека и бота, эндпоинт `POST /games/with-bot`.

Детальный план реализации: `docs/bots.md`.

### 23. Мгновенная сдача / forfeit (P0)
**Затрагивает:** `internal/game/game.go`, `internal/game/fsm.go`, REST API, фронт

Сейчас можно только «предложить конец» — соперник должен согласиться. Нужна односторонняя
сдача: `POST /games/{id}/forfeit`, новое событие `EventForfeit` в FSM, моментальный
переход в `StateGameOver` с поражением сдавшегося.

### 24. Одиночный режим / тренировка (P1)
**Затрагивает:** `internal/game/`, `internal/lobby/`, REST API, фронт

Игра без соперника: игрок просто набирает очки на время или до заполнения доски. Без рейтинга и
сохранения в `game_results`. Полезно для обучения новичков.

**Предложение:** добавить `POST /games/solo`, пропускать проверки второго игрока в лобби,
не публиковать избыточные Centrifugo-события.

### 25. Контроль времени на выбор (P1)
**Затрагивает:** `internal/game/game.go` (`TurnDuration`), REST API, фронт

Сейчас жёстко 60s на ход. Добавить режимы: блиц (15s), рапид (30s), классика (60s).
Параметр `turn_duration` в `CreateGame`. Отображать режим в списке игр и в лобби.

### 26. Вариативность размера доски (P2)
**Затрагивает:** `internal/game/table.go` (`[5][5]`), `internal/game/game.go`, REST API

Поддержка 3×3 (быстрая), 5×5 (классика), 7×7 (марафон). Параметр `board_size` в `CreateGame`.
3×3 — стартовое слово из 3 букв, 7×7 — из 7.

### 27. Ежедневное испытание (P2)
**Затрагивает:** `internal/game/`, `internal/lobby/`, `internal/storage/`, REST API, фронт

Всем игрокам даётся одинаковое стартовое слово на 24 часа. Соревнование по очкам.
Хранить в БД (`daily_challenges`) без привязки к `game_results`. Таблица лидеров за сегодня.

**Предложение:** cron или фоновая горутина для смены испытания в 00:00 UTC.

### Рейтинг и прогрессия

### 28. ELO / рейтинговая система (P1)
**Затрагивает:** `internal/storage/player_state`, `internal/matchmaking/`

Сейчас есть только EXP (опыт), который всегда растёт. Нужен рейтинг (ELO/Glicko) для честного
подбора соперников. Переиспользовать существующую `matchmaking.Queue` — она уже реализована
с окном по EXP, нужно заменить на рейтинг.

**Предложение:** добавить поле `rating int` в `player_state`, формулу ELO с K=32, обновлять
рейтинг в `storage.SaveGameResult`.

### 29. Таблица лидеров (P2)
**Затрагивает:** `internal/storage/`, REST API, фронт

Недельный и месячный топ игроков по рейтингу/EXP. Эндпоинт `GET /leaderboard`.
Кешировать в Redis (TTL 5 мин) для снижения нагрузки на БД.

### 30. Достижения (P2)
**Затрагивает:** `internal/game/`, `internal/storage/player_state` (`flags`), REST API, фронт

Поле `flags` (bigint, битовая маска) уже есть в `player_state`, но не используется.
Примеры достижений: «Первая победа», «Слово из 10+ букв», «Заполнил доску» (25 ходов без
скипов), «5 побед подряд», «Сыграно 100 игр».

**Предложение:** проверять условия в `onGameOver`, выставлять биты через `storage.AddAchievement`,
публиковать событие `achievement_unlocked` в Centrifugo.

### Качество жизни

### 31. Реванш (P2)
**Затрагивает:** `internal/lobby/`, REST API, фронт

После окончания игры — кнопка «Реванш». Создаёт новую игру с тем же соперником мгновенно,
без возврата в лобби. Эндпоинт `POST /games/{id}/rematch` или параметр `rematch_of` в `CreateGame`.

### 32. Чат в игре (P3)
**Затрагивает:** `internal/centrifugo/` (новый канал), фронт

Centrifugo уже есть — достаточно добавить канал `game:{id}:chat` и хендлер `POST /games/{id}/chat`.
Сообщения только в реальном времени, без хранения в БД (MVP).

### 33. История игр с реплеем (P3)
**Затрагивает:** `internal/storage/`, REST API, фронт

Просмотр прошлых партий пошагово. Данные в `game_results` и `game_result_players` уже пишутся,
но не хранится последовательность ходов.

**Предложение:** добавить таблицу `game_moves` (game_id, move_number, player_id, letter, word, coords)
и эндпоинт `GET /games/{id}/replay`. Запись хода в `SubmitWord`.

### 34. Статистика игрока (P3)
**Затрагивает:** `internal/storage/`, REST API, фронт

Средняя длина слова, лучшее слово, винрейт, любимая буква. Аггрегировать из `game_result_players`
или добавить материализованную статистику в `player_state`.

### Механики доски

### 35. Бонусные клетки (P3)
**Затрагивает:** `internal/game/table.go`, `internal/game/game.go`

×2 слова, ×3 буквы — как в Scrabble/«Эрудит». Случайно размещаются на доске при старте игры.
Умножать счёт хода на коэффициент клетки. Требует UI-отображения бонусов на доске.

### 36. Джокеры / запас wildcard-букв (P3)
**Затрагивает:** `internal/game/game.go`, `internal/game/player.go`

Каждому игроку даётся 1-2 wildcard-буквы на всю игру. Можно поставить любую букву без проверки
словаря (слово всё равно валидируется). Добавить поле `Wildcards int` в `Player`.

### 37. Блокировка клетки (P4)
**Затрагивает:** `internal/game/game.go`, `internal/game/fsm.go`, REST API

Раз за игру игрок может запретить сопернику использовать конкретную пустую клетку на 1 ход.
Эндпоинт `POST /games/{id}/block-cell`, состояние блокировки в `LettersTable`.

### Баланс и усложнение

### 38. Рост минимальной длины слова (P3)
**Затрагивает:** `internal/game/game.go` (`SubmitWord`)

По мере заполнения доски минималка растёт с 3 до 4→5 букв. Например: `< 10 букв на доске` → 3,
`10–16` → 4, `> 16` → 5. Усложняет эндшпиль и уменьшает количество ничьих.

### 39. Турнирная сетка (P4)
**Затрагивает:** `internal/lobby/`, `internal/storage/`, REST API, фронт

Плей-офф на 4/8 игроков. Нужна таблица `tournaments`, регистрация, жеребьёвка, автоматическое
создание игр по сетке.
