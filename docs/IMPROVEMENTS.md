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

### 8. ✅ Утечка кредов в логи устранена
**Файл:** `internal/server/restapi/handlers/handlers.go`

Убрано логирование значения токена, удалён шумный `slog.Info("KeyAuth ... handler called")`.
Аутентификация переведена на JWT Bearer (см. пункт 9): токен больше не логируется и не
передаётся в storage напрямую.

### 9. ✅ Единый статический API-токен заменён на JWT Bearer
**Файлы:** `cmd/server.go`, `internal/server/restapi/handlers/handlers.go`,
`internal/service/balda_service.go`, `internal/auth/`

Глобальный `xAPIToken` из конфига удалён. Вместо персональных API-ключей реализована
JWT-аутентификация: `/signup` и `/auth` выдают пару `access_token` (1h) + `refresh_token` (30d),
защищённые эндпоинты принимают `Authorization: Bearer <access_token>`, а `HandleBearerAuth`
проверяет подпись JWT без обращения к БД на каждый запрос.

**Нюанс:** персональные API-ключи в колонке `users.api_key` не используются кодом
(колонка создаётся/пересоздаётся миграциями, но не читается).

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

### 13. ✅ Matchmaking — подключён (быстрый бой)
**Файлы:** `cmd/server.go`, `internal/service/balda_service.go`, `internal/matchmaking/`

`matchmaking.Queue` пэйрит игроков по `Rating` (ELO) с расширяющимся окном.

**Сделано (п. 42):** очередь подключена в проде как «Быстрый бой»: эндпоинты
`POST /matchmaking/join|leave`, `go mm.Run(ctx)`, событие `match_found` в канал
`lobby`, серверный фолбэк в игру с ботом по `MaxWait` (15s), офлайн-записи
выбиваются по presence. Кнопки «создать/с ботом» в лобби заменены одной «Играть»
(+ вторичная «Создать приватную игру»).

---

## 🟡 Средний приоритет (надёжность, дизайн, безопасность)

### 14. ✅ Дублирование и рассинхрон публикаций game_state
**Файлы:** `internal/server/restapi/handlers/move_game.go`, `internal/server/restapi/handlers/skip_game.go`

И `MoveGame`, и `SkipGame` публиковали `game_state` с «угаданным» `nextPlayerID`, дублируя то,
что `gamecoord` публикует на `NotifyTurnStart`, и рискуя рассинхроном с FSM.

**Сделано:** обе хендлер-публикации `game_state` удалены — единственный источник истины теперь
`gamecoord` (`turn_change` + `game_state` на старте хода). `MoveGame` по-прежнему возвращает
синхронный HTTP-ответ с доской и `current_turn_uid` как оптимистичным хинтом для мовера (не
бродкаст); `SkipGame` просто отдаёт 204. Удалён осиротевший `buildGameState`.

---

## 🟢 Низкий приоритет (чистота, инфраструктура, тесты)

### 15. ✅ CI: линтер + -race, разбиение job'ов
**Файл:** `.github/workflows/ci.yml`

Убран мёртвый шаг `Install swagger` (code-gen на ogen). `Lint` вынесен в отдельный
параллельный job (`golangci/golangci-lint-action@v7`, pin `v2.12.2` под конфиг v2).
Тесты разделены: быстрые unit-пакеты гоняются с `-race`
(`go test -race $(go list ./... | grep -v '/tests$')`), а тяжёлые интеграционные
(`./tests/...`, testcontainers: Postgres/Redis/Centrifugo) — **без** `-race`, чтобы не
раздувать CI (с `-race` они шли ~172s против ~68s). Их конкурентность покрыта raced
unit-тестами. Локально: линт 0 issues, `-race` на unit-пакетах зелёный (гонок нет).

### 16. ✅ Артефакт coverage.out удалён из репозитория
**Файл:** `coverage.out`

Отчёт покрытия больше не коммитится; файл добавлен в `.gitignore`.

**Статус:** предложение выполнено — артефакт исключён из git.

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

### 21. ✅ Presence для ускоренного таймаута хода
**Файлы:** `internal/presence/presence.go`, `internal/server/restapi/handlers/ping.go`,
`internal/game/game.go`, `cmd/server.go`

При старте хода, если текущий игрок-человек офлайн, таймер хода сокращается до грейс-окна
(`game.OfflineGraceDuration`, 10s) вместо полных 60s — брошенная игра быстро продвигается и
чистится (3 таймаута → kick за ~30s вместо ~180s).

**Сделано:**
- `presence.Service` переведён с ключа `uid int64` на `player_id` (string) — это идентичность,
  которой оперирует FSM. `ping` рефрешит по `claims.PlayerID`.
- В `game` добавлен узкий интерфейс `OnlineChecker { IsOnline(playerID) bool }` и опция
  `WithOnlineChecker`; `startTurn` сокращает таймер только для **человека** и только в меньшую
  сторону (боты и кастомные короткие длительности не затрагиваются).
- `cmd/server.go` пробрасывает адаптер `presenceChecker` (оборачивает `*presence.Service`
  ctx+таймаутом) в фабрику игр.
- Тесты: офлайн-игрок таймаутится в грейс-окне, онлайн — держит полный ход.

**Нюанс:** presence ставится после первого пинга клиента (фронт пингует с входа в лобби,
TTL 30s), так что к старту партии игрок обычно уже «онлайн». Если клиент не пинговал — первый
ход может получить грейс; для реального фронта это не воспроизводится.

---

## 🆕 Игровые механики (новый функционал)

Потенциальные механики для расширения геймплея. Сгруппированы по темам, внутри — по приоритету.

### Режимы игры

### 22. 🔶 Боты (частично реализовано)
**Затрагивает:** `internal/game/bot/` (новый), `internal/lobby/`, `internal/game/`, `cmd/server.go`, фронт

Базовая игра против бота работает.

**Сделано:**
- `internal/game/bot/` — движок, trie и стратегия `RandomValidStrategy`;
- `PlayerType` (`Human`/`Bot`) добавлен в `Player`;
- `CompositeNotifier` раздельно нотифицирует человека и бота;
- эндпоинт `POST /games/with-bot` реализован.

**Осталось:** стратегии `GreedyStrategy` (макс. длина слова) и `MinimaxStrategy` не
реализованы. Детальный план `docs/bots.md` больше не существует.

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

### 25. 🔶 Контроль времени на выбор (P1, частично)
**Затрагивает:** `internal/game/game.go` (`TurnDuration`), REST API, фронт

Сейчас жёстко 60s на ход. Добавить режимы: блиц (15s), рапид (30s), классика (60s).
Параметр `turn_duration` в `CreateGame`. Отображать режим в списке игр и в лобби.

**Сделано:** внутренняя поддержка есть — `game.WithTurnDuration` + `Game.turnDuration`,
по умолчанию 60s.

**Осталось:** не выведено в API — `CreateGame` не принимает `turn_duration`, нет режимов
блиц/рапид/классика и отображения в лобби.

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

### 28. ✅ ELO / рейтинговая система (P1)
**Затрагивает:** `internal/storage/player_state`, `internal/storage/game_result.go`,
`internal/matchmaking/`, REST API, `internal/centrifugo/`, `internal/gamecoord/`

Реализована ELO-рейтинговая система.

**Сделано:**
- Добавлена миграция `006_add_rating.up.sql`: колонка `rating int not null default 1000` в `player_state`.
- `storage.PlayerState`, `storage.PlayerForGame`, `game.Player`, `lobby.PlayerInfo`,
  `centrifugo.PlayerState`, `centrifugo.LobbyPlayer` теперь содержат `Rating`.
- `storage.SaveGameResult` внутри транзакции загружает текущие рейтинги двух игроков,
  вычисляет дельты по формуле ELO с `K=32` (`storage.EloDelta`) и обновляет `player_state`.
- `matchmaking.Queue` теперь пэйрит игроков по `Rating` вместо `Exp`.
- Рейтинг пробрасывается в игру через `service.Balda` и возвращается клиенту в API-ответах
  (`Player`, `LobbyPlayer`, `PlayerGameState`, `PlayerState`) и Centrifugo-событиях
  (`game_state`, `game_over`, `lobby_update`).

**Осталось (не в рамках P1):** матчмейкинг всё ещё не подключён в проде — для запуска
«Быстрого боя» нужен отдельный эндпоинт + UI-кнопка (см. пункт 13).

### 29. ✅  Таблица лидеров (P2)
**Затрагивает:** `internal/storage/`, REST API, фронт

Недельный и месячный топ игроков по рейтингу/EXP. Эндпоинт `GET /leaderboard`.
Кешировать в Redis (TTL 5 мин) для снижения нагрузки на БД.

### 30. ✅ Достижения (P2)
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

### 34. ✅ Статистика игрока (P3)
**Затрагивает:** `internal/storage/`, REST API, фронт

Средняя длина слова, лучшее слово, винрейт, любимая буква. Аггрегируется на лету из
`game_result_players` (без материализованной статистики в `player_state`).

**Сделано:**
- Миграция `010_game_result_words.up.sql`: колонка `words jsonb` в `game_result_players`;
  `storage.SaveGameResult` сохраняет слова игрока (из `game.PlayerState.Words`).
- `storage.GetPlayerStats`: игры/победы/поражения/ничьи, винрейт, средняя длина слова
  (из `SUM(score)/SUM(words_count)` — работает и для старых игр без `words`), лучшее
  слово и любимая буква (по сохранённым словам; `ё` приравнена к `е`).
- Эндпоинт `GET /player/stats` (BearerAuth, по JWT-claims — как `/player/achievements`),
  схема `PlayerStatsResponse`.
- Фронт: `StatsModal.svelte` с метриками, кнопка 📊 в лобби рядом с достижениями.

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

### Telegram и рост

### 40. ✅ Приглашение друга в партию через startapp (P1)
**Затрагивает:** фронт (`WaitingScreen`/`AuthForm`), `GET /config`

**Сделано:**
- `GET /config` отдаёт `telegram_app_url` (флаг `telegram.app_url` / env `TELEGRAM_APP_URL`).
- WaitingScreen: кнопка «Пригласить друга» → ссылка `<app_url>?startapp=game_<id>`
  (в Telegram — нативный шаринг, в браузере — копирование в буфер).
- Приём: после авто-логина по initData читается `initDataUnsafe.start_param`;
  `game_<id>` → автоджойн через `joinGame`; недоступная игра → лобби + нотификация.

### 41. Бот-бэкенд: /start-ответчик и уведомления (P2)
**Затрагивает:** `internal/server/restapi/` (webhook), `cmd/server.go`, `internal/storage/`

Живой бот вместо «пустого»: webhook `POST /telegram/webhook` (секретный заголовок),
регистрация через `setWebhook`. На `/start` отвечает приветствием с inline-кнопкой
web_app (онбординг с картинкой и правилами). Персональные приглашения:
`t.me/<bot>?start=invite_<id>` → сообщение «X зовёт тебя в партию».

Побочный эффект — канал уведомлений: пометить `users.bot_started_at`, и бот сможет
писать первым («твой ход», «соперник найден», «тебя обогнали в лидерборде») —
главный инструмент возврата игроков. Реализуется в том же процессе server, без
отдельного сервиса.

### 42. ✅ Быстрый бой / matchmaking с фолбэком в бота (P1)
**Затрагивает:** `internal/matchmaking/`, REST API, `internal/centrifugo/`, фронт, `internal/storage/`, `internal/gamecoord/`, фронт

Одна главная кнопка «Играть»: сервер ищет соперника по ELO, через 15 секунд
безуспешного поиска — автоматически создаёт игру с ботом.

**Сделано:**
- `Queue` += `MaxWait`/`onExpire`/`isOnline`: записи старше 15s уходят в бот-фолбэк,
  офлайн-записи выбиваются по presence.
- Эндпоинты `POST /matchmaking/join|leave` (join → 200 queued / 409 уже в игре
  или очереди; leave идемпотентен). `go mm.Run(ctx)` в проде.
- Событие `match_found` в канал `lobby`: game_id, `vs_bot`, доска, `current_turn_uid`,
  per-player game-токены (по числовому uid через `storage.GetUIDByPlayerID`) —
  без гонки publish-before-subscribe.
- Фронт: кнопка «Играть» + состояние поиска (счётчик, отмена); «Играть с ботом»
  убрана (бот приходит фолбэком); «Создать приватную игру» — вторичная.
- **Игры с ботом теперь полноценные для человека:** бот имеет фиксированный
  `bot.BotPlayerID` и не существует в БД; `storage.PlayerResult.Bot` пропускает
  его при сохранении, но человек получает ELO (против 1000), EXP, счётчики и
  статистику (`/player/stats`). Гейт `hasBotPlayer` в `cmd/server.go` снят.
