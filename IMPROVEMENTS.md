# Предложения по улучшению Balda

Документ собран по итогам ревью кодовой базы. Пункты сгруппированы по приоритету.
Для каждого указаны затронутые файлы, суть проблемы и предлагаемое решение.

## 🔴 Критические (корректность, конкурентность, безопасность)

### 1. ✅ Игра завершается естественным образом
**Файлы:** `internal/game/fsm.go`, `internal/game/game.go`

Реализовано: `EventGameFinished` заменяет `EventBoardFull` как единое событие естественного
завершения (доска заполнена или все игроки подряд сделали skip). Победитель при любом
естественном конце определяется по `Score` (в `gamecoord.findWinnerByScore`).

Если доска заполнена — `SubmitWord` отправляет `EventGameFinished`.
Если все игроки подряд скипают (`consecutiveSkipsTotal >= len(players)`) — `onSkip`
отправляет `EventGameFinished` как эвристику «нет доступных ходов».

### 3. ✅ Нет graceful shutdown и таймаутов HTTP-сервера
**Файлы:** `cmd/root.go:22` (`rootCmd.Execute()`), `cmd/server.go:159` (`http.ListenAndServe`)

Реализовано: `Execute()` заменён на `ExecuteContext()` с `signal.NotifyContext(SIGINT/SIGTERM)`.
HTTP-сервер создан явно как `&http.Server{...}` с `ReadTimeout`, `WriteTimeout`, `IdleTimeout`
и `ReadHeaderTimeout`. Сервер ожидает `<-cmd.Context().Done()` вместо ручного `signal.Notify`,
после чего вызывается `lby.Shutdown()` (отмена контекста всех игр) и `httpSrv.Shutdown(ctx)`
с таймаутом 30s. Дожидаются завершения `pendingResults` (сохранение результатов игр).

## 🟠 Высокий приоритет (незавершённый функционал, мёртвый код)

### 4. Matchmaking подключён, но не работает
**Файлы:** `cmd/server.go:130`, `internal/service/balda_service.go:16`

`matchmaking.Queue` создаётся, но `mm.Run(ctx)` нигде в проде не вызывается, и ни один хендлер
не делает `mm.Enqueue`. В продакшене очередь — мёртвый код (используется только в тестах).

**Предложение:** либо подключить (эндпоинт постановки в очередь + `go mm.Run(serverCtx)`),
либо убрать из прод-пути до готовности фичи.

### 5. ✅ `Player.Exp` не загружается из БД
**Файлы:** `internal/service/balda_service.go:70,79`

Реализовано: `CreateGame` и `JoinGame` загружают `exp` через `storage.GetPlayerByUID`
(`SELECT player_id, COALESCE(exp, 0) FROM player_state`) и передают в `game.Player{Exp: p.Exp}`.
Тестовые хелперы (`makePlayers` в `lobby_test`, `game_fsm_test`, `coord_test`, `list_games_test`,
`handlers_test`) также обновлены для создания игроков с ненулевым `Exp`.

### 6. ✅ Результаты игр не сохраняются
**Файлы:** `internal/gamecoord/coord.go` (game_over), `internal/storage/game_result.go`

Реализовано: `gamecoord.dispatchGameResult` вызывает `onGameOver` callback (провайдерится в
`cmd/server.go` как `makeOnGameOverCallback`) до публикации в Centrifugo.
`storage.SaveGameResult` в транзакции: (1) INSERT INTO `game_results`, (2) INSERT INTO
`game_result_players` (score, words_count, exp_gained), (3) UPDATE `player_state SET exp = exp + $1`.
Callback обёрнут в retry (3 попытки, backoff 100/200 мс) и учитывается в `pendingResults`
WaitGroup для graceful shutdown. Комментарий в миграции `003_game_results.up.sql` обновлён
с `'board_full'` на `'game_finished'`.

### 7. ✅ Мёртвый/легаси-код в game.go
**Файлы:** `internal/game/game.go:454` (`AddWordToCurrentPlayer`), `:460` (`IsTakenWord`)

Реализовано: удалены `AddWordToCurrentPlayer` и `IsTakenWord`. Проверка дубликатов в
`SubmitWord` оставлена как `slices.Contains` (быстро, без аллокаций). Тесты мёртвого кода
удалены, лишние тесты на `IsTakenWord` убраны.

## 🟡 Средний приоритет (надёжность, дизайн, безопасность)

### 8. ✅ Гомоглиф в имени метода
**Файл:** `internal/game/game.go:75` — `СheckWordExistence`

Реализовано: кириллическая `С` (U+0421) заменена на латинскую `C` (U+0043) в методе
`CheckWordExistence`. Исправлено в PR #102 (`fix/method-typo`).

### 9. ✅ Дублирование запроса player_id
**Файл:** `internal/service/balda_service.go:70,79,87`

Реализовано: SQL-запрос вынесен в единый метод `storage.GetPlayerByUID`
(`SELECT player_id, COALESCE(exp, 0) FROM player_state WHERE user_id = $1`).
Сервисный слой использует этот метод: `CreateGame`/`JoinGame` получают
`PlayerForGame` (player_id + exp), `playerIDByUID` — обёртка для методов,
где нужен только player_id.

### 10. ✅ Утечка кредов в логи и шумное логирование
**Файл:** `internal/server/restapi/handlers/handlers.go:74,84`

Реализовано: убрано логирование значения токена (`slog.String("token", ...)`), удалён
шумный `slog.Info("KeyAuth ... handler called")` на каждый запрос. Сравнение токенов
заменено на `subtle.ConstantTimeCompare` для защиты от timing-атак.

### 11. Единый статический API-токен на всех пользователей
**Файлы:** `cmd/server.go:182`, `handlers.go`

Аутентификация идёт по общему `xAPIToken`; у каждого юзера есть `api_key` в БД, но он не
используется. Дизайн запутан.

**Предложение:** определиться с моделью аутентификации (персональные ключи vs общий токен) и
задокументировать.

### 12. Логическая ошибка в auth.go
**Файл:** `internal/server/restapi/handlers/auth.go:27-32`

Ветка `if uid == 0` после ошибки `Scan` всегда истинна (при ошибке `uid` не присваивается),
поэтому различение «неверный пароль» и «ошибка БД» не работает. Любая ошибка возвращает 401,
даже сбой БД (должен быть 500).

**Предложение:** различать `errors.Is(err, pgx.ErrNoRows)` → 401, иначе → 500.

### 13. Не проверяется минимальная длина слова (≥3)
**Файл:** `internal/game/game.go:280` (`SubmitWord`)

README обещает «3+ буквы», но код отбраковывает только пути длиной <2 (`GapsBetweenLetters`).
Двухбуквенное слово из словаря пройдёт.

**Предложение:** добавить явную проверку `len(word) >= 3` (или вынести константу).

### 14. Дублирование и рассинхрон публикаций game_state
**Файлы:** `internal/server/restapi/handlers/move_game.go:91-97`, `internal/gamecoord/coord.go:51-64`

И хендлер `MoveGame`, и `gamecoord` (на `NotifyTurnStart`) публикуют `game_state` на каждый ход.
Хендлер при этом сам «угадывает» следующего игрока (`nextPlayerID`), дублируя логику FSM, что
может рассинхронизироваться с реальным состоянием игры (особенно при скипах/таймаутах).

**Предложение:** оставить единственный источник истины публикаций (gamecoord), хендлер вернёт
синхронный ответ без второй публикации.

## 🟢 Низкий приоритет (чистота, инфраструктура, тесты)

### 15. CI без линтера и без -race
**Файл:** `.github/workflows/ci.yml`

Есть `.golangci.yml`, но в CI нет шага `golangci-lint`. Тесты гоняются без `-race` (критично для
конкурентного игрового сервера). Шаг `Install swagger` (go-swagger) выглядит устаревшим — code-gen
использует ogen.

**Предложение:** добавить `golangci-lint run`, `go test -race`, убрать ненужный go-swagger.

### 16. Артефакт coverage.out в репозитории
**Файл:** `coverage.out`

Закоммичен отчёт покрытия.

**Предложение:** удалить из git и добавить в `.gitignore`.

### 17. Мелочи в storage
**Файл:** `internal/storage/storage.go`

Поле `t time.Duration` хранится, но не используется. `func (b Balda) Pool()` — value receiver
копирует структуру.

**Предложение:** удалить неиспользуемое поле либо реально применять таймаут; перейти на pointer receiver.

### 18. centrifugo.Client без таймаута
**Файл:** `internal/centrifugo/client.go:21`

`http.Client{}` без `Timeout`; полагается только на ctx вызывающего.

**Предложение:** задать дефолтный `Timeout` как страховку.

### 19. Dictionary.FiveLetters — map вместо slice
**Файл:** `internal/game/dictionary.go:17,52,62`

`map[int]string` с последовательным счётчиком-ключом — фактически срез. Лишний оверхед.

**Предложение:** заменить на `[]string`.

### 20. Неограниченные горутины публикаций в gamecoord
**Файл:** `internal/gamecoord/coord.go:62,63,73,81,90`

Каждое событие порождает `go c.publish...`; нет ограничения количества и нет гарантий порядка
доставки `turn_change` vs `game_state`.

**Предложение:** рассмотреть последовательную публикацию (без `go`) под 3-сек ctx или единый
канал публикаций.

### 21. ✅ Аутентификация и игровое присутствие смешаны в одну сессию
**Файлы:** `internal/session/session.go`, `internal/session/config.go`, `internal/server/restapi/handlers/ping.go`

Реализовано (ветка `feature/separate-auth-presence`): авторизационная сессия (TTL 24h) и игровое
присутствие разделены. Новый пакет `internal/presence/` ведёт ключи `presence:{uid}` в Redis с
TTL 30s. `POST /session/ping` теперь обновляет только presence, не трогая auth-сессию.
Фронт шлёт пинг каждые 5s — запас в 6× относительно TTL.

### 22. Игровая логика не использует presence для ускоренного таймаута
**Файлы:** `internal/gamecoord/coord.go`, `internal/lobby/lobby.go`, `cmd/server.go`

При отключении игрока (presence-ключ истёк) сервер по-прежнему ждёт полных 60s таймаута хода,
а не реагирует сразу.

**Предложение:** пробросить `*presence.Service` в фабрику игр (`cmd/server.go`) и при старте
хода запускать сокращённый grace-таймер (например, 5s) если `presence.IsOnline` вернул false.
