# Балда

Многопользовательский пошаговый игровой сервер для игры в «Балду», написанный на Go, с интерфейсом на Svelte 5 в реальном времени. Игроки соревнуются на поле 5×5, составляя слова и набирая очки.

> Проект в процессе разработки — личный проект «для души».

## Что такое Балда?

Балда — классическая русская игра в слова. Два игрока используют общее поле 5×5. Игра начинается со случайного пятибуквенного русского слова в центральной строке. На каждом ходу игрок обязан:

1. Поставить ровно одну новую букву на поле (рядом с уже существующей буквой)
2. Составить слово из букв, уже находящихся на поле (включая новую)
3. Слово должно быть в русском словаре и не должно использоваться повторно

Побеждает игрок, набравший больше слов к концу игры.

## Технологический стек

| Слой | Технология |
|------|-----------|
| Язык | Go 1.26 |
| REST API | [ogen](https://github.com/ogen-go/ogen) (генерация кода из спецификации OpenAPI 3.0) |
| CLI | [cobra](https://github.com/spf13/cobra) |
| База данных | PostgreSQL 16 (драйвер pgx/v5) |
| Хранилище сессий | Redis 8 |
| События реального времени | [Centrifugo v6](https://centrifugal.dev) (WebSocket pub/sub) |
| Фронтенд | Svelte 5 (runes API) + Tailwind CSS, раздаётся через Nginx |
| Миграции | [tern](https://github.com/jackc/tern) (встроенный SQL, запускается при старте сервера) |
| Логирование | `log/slog` (стандартная библиотека) |
| Образ для запуска | Debian trixie-slim |

## Структура проекта

```
balda/
├── cmd/                    # Точки входа CLI (сервер)
├── internal/
│   ├── game/               # Основная игровая логика, FSM, таблица букв, словарь
│   ├── gamecoord/          # Координатор: связывает игровые события → Centrifugo
│   ├── lobby/              # Реестр активных игр в памяти
│   ├── matchmaking/        # Очередь матчмейкинга по рейтингу
│   ├── centrifugo/         # Клиент HTTP API Centrifugo + типы событий
│   ├── notifier/           # Абстракция уведомлений (отправка через Redis)
│   ├── server/
│   │   ├── ogen/           # Сгенерированный ogen-кодом сервер (не редактировать)
│   │   └── restapi/
│   │       └── handlers/   # Обработчики HTTP-запросов (move_game.go, skip_game.go и др.)
│   ├── session/            # Управление сессиями через Redis
│   ├── service/            # Сервисный слой приложения
│   ├── storage/            # Доступ к PostgreSQL
│   ├── flname/             # Автогенерация никнеймов игроков
│   └── rnd/                # Утилиты для генерации случайных чисел
├── frontend/               # Фронтенд на Svelte 5
│   └── src/
│       ├── App.svelte      # Корень: подключение к Centrifugo + диспетчеризация событий
│       ├── components/     # AuthForm, Lobby, GameScreen, Board, Alphabet, …
│       ├── stores/         # Реактивное состояние игры (game.svelte.ts)
│       ├── lib/            # api.ts, centrifugo.ts
│       └── types.ts        # TypeScript-интерфейсы
├── api/openapi/            # Спецификация OpenAPI 3.0
├── migrations/             # SQL-файлы миграций
├── tests/                  # Интеграционные тесты (testcontainers)
├── Makefile
└── docker-compose.yml
```

## Архитектура

Архитектура системы описана в виде [модели C4](docs/architecture.md) (все четыре уровня: Context, Container, Component, Code).

## Быстрый старт

### Требования

- Docker и Docker Compose

### Запуск через Docker Compose

```bash
docker compose up
```

Запускает PostgreSQL, Redis, Centrifugo, игровой сервер на порту `9666` и фронтенд на порту `8080`.

Откройте `http://localhost:8080` для игры. Фронтенд также доступен с других устройств в локальной сети по адресу `http://<ip-хоста>:8080` (например, `http://192.168.1.42:8080`).

### Разработка фронтенда

```bash
cd frontend
npm install
npm run dev
```

Vite dev-сервер слушает на всех интерфейсах (`0.0.0.0:5173`), поэтому можно открыть игру с телефона или другого компьютера в той же Wi-Fi-сети по адресу `http://<ip-хоста>:5173`.

Прокси-адреса для бэкенда и Centrifugo настраиваются через переменные окружения (см. `frontend/.env.example`):

```bash
BALDA_API_PROXY_URL=http://127.0.0.1:9666 \
BALDA_CENTRIFUGO_PROXY_URL=http://127.0.0.1:8000 \
npm run dev
```

### Пересборка и перезапуск

```bash
make restart
```

### Ручная сборка бинарника сервера

```bash
make build
```

```bash
export MIGRATION_CONN_STRING="postgres://balda:password@localhost:5432/balda"

./bin/balda server \
  --server.addr 0.0.0.0 \
  --server.port 9666 \
  --server.x_api_token ваш-api-токен \
  --pg.host localhost --pg.port 5432 \
  --pg.user balda --pg.database balda --pg.password password \
  --redis.addr localhost:6379
```

Все флаги можно также задавать через переменные окружения (например, `SERVER_ADDR`, `PG_HOST`, `REDIS_ADDR`).

> **Примечание:** `MIGRATION_CONN_STRING` должна быть задана до запуска сервера. Миграции применяются автоматически при старте.

### Регенерация кода API

```bash
make code-gen
```

Регенерирует типизированный Go-код сервера из [api/openapi/http-api.yaml](api/openapi/http-api.yaml) с помощью [ogen](https://github.com/ogen-go/ogen) и вендорит результат.

### Запуск тестов

```bash
make test
```

Интеграционные тесты в `tests/` поднимают временные контейнеры PostgreSQL и Redis через [testcontainers-go](https://golang.testcontainers.org/) — Docker должен быть запущен.

---

## API

Базовый путь: `/balda/api/v1`

Аутентификация использует заголовок `X-API-Key` (или параметр запроса `api_key`). Эндпоинты, требующие сессии, также ожидают `X-API-Session`.

Swagger UI доступен по адресу `/balda/api/v1/docs` при запущенном сервере.

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/signup` | Регистрация нового аккаунта |
| POST | `/auth` | Аутентификация и получение сессии |
| POST | `/session/ping` | Keepalive — обновляет TTL сессии |
| GET | `/player/state/{uid}` | Получить профиль и состояние игрока |
| GET | `/games` | Список всех активных игр |
| POST | `/games` | Создать новую игру в ожидании |
| POST | `/games/{id}/join` | Присоединиться к игре в ожидании |
| POST | `/games/{id}/move` | Отправить ход (поставить букву + слово) |
| POST | `/games/{id}/skip` | Пропустить текущий ход |

### POST /signup

```json
// Запрос
{ "firstname": "Иван", "lastname": "Петров", "email": "ivan@example.com", "password": "secret" }

// Ответ
{
  "user": { "uid": "…", "firstname": "Иван", "lastname": "Петров", "sid": "…", "key": "…" },
  "centrifugo_token": "…",
  "lobby_token": "…"
}
```

### POST /auth

```json
// Запрос
{ "email": "ivan@example.com", "password": "secret" }

// Ответ
{
  "player": { "uid": "…", "firstname": "Иван", "lastname": "Петров", "sid": "…", "key": "…" },
  "centrifugo_token": "…",
  "lobby_token": "…"
}
```

### POST /games

Создаёт новую игру в статусе `waiting`. Возвращает `game_token` для подписки на канал Centrifugo этой игры.

```json
// Ответ
{
  "game": { "id": "…", "player_ids": ["<creator>"], "status": "waiting", "started_at": 1712600000000 },
  "game_token": "…"
}
```

### POST /games/{id}/join

Присоединяется к игре в ожидании. Когда второй игрок подключается, игра начинается немедленно. Возвращает начальное состояние доски, чтобы избежать гонки «публикация до подписки» с Centrifugo.

```json
// Ответ
{
  "game": { "id": "…", "player_ids": ["<creator>", "<joiner>"], "status": "in_progress", "started_at": 1712600000000 },
  "game_token": "…",
  "board": [["","","","",""],["","","","",""],["с","л","о","в","о"],["","","","",""],["","","","",""]],
  "current_turn_uid": "<uid-создателя>"
}
```

### POST /games/{id}/move

Отправляет ход: ставит одну новую букву на поле и указывает путь слова.

```json
// Запрос
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

// Ответ
{
  "board": [["","","","",""],…],
  "current_turn_uid": "…",
  "players": [{"uid":"…","score":5,"words_count":1}],
  "status": "in_progress",
  "move_number": 1
}
```

### POST /games/{id}/skip

Пропускает текущий ход. При успехе возвращает `204 No Content`.

---

## События реального времени (Centrifugo)

После аутентификации клиент подключается к Centrifugo с `centrifugo_token`. События передаются по каналам:

| Канал | Тип события | Когда |
|-------|-------------|-------|
| `lobby` | `game_created` | После `POST /games` |
| `lobby` + `game:{id}` | `game_started` | После `POST /games/{id}/join` |
| `game:{id}` | `game_state` | При начале хода и после каждого принятого хода |
| `game:{id}` | `turn_change` | При каждой смене хода (любая причина) |
| `game:{id}` | `game_over` | Когда игра завершается |

### `game_state`

Полный снимок доски — отправляется после начала игры и после каждого хода.

```json
{ "type": "game_state", "game_id": "…", "board": [["","…"]],
  "current_turn_uid": "…", "players": [{"uid":"…","score":0,"words_count":0}],
  "status": "in_progress", "move_number": 0 }
```

### `turn_change`

Общее уведомление о смене хода — отправляется при каждом начале хода. Поле `reason` указывает причину смены.

```json
{ "type": "turn_change", "game_id": "…", "current_turn_uid": "…",
  "reason": "game_start" }
```

Возможные значения `reason`: `game_start`, `move`, `skip`, `timeout`.

### `game_over`

```json
{ "type": "game_over", "game_id": "…", "winner_uid": "…",
  "players": [{"uid":"…","score":5,"words_count":2}] }
```

Отправляется при завершении игры — либо потому что поле заполнено, либо игрок был исключён. `winner_uid` отсутствует при ничьей.

---

## Игровая механика

### Поле

Сетка 5×5. Начальное слово занимает центральную строку (индекс строки 2). Координаты задаются как `(RowID, ColID)` от `(0,0)` до `(4,4)`.

```
[ ][ ][ ][ ][ ]   строка 0
[ ][ ][ ][ ][ ]   строка 1
[С][л][о][в][о]   строка 2  ← начальное слово
[ ][ ][ ][ ][ ]   строка 3
[ ][ ][ ][ ][ ]   строка 4
```

### Ход

- Каждому игроку отводится **60 секунд** на ход.
- По истечении времени ход автоматически переходит к другому игроку; никаких действий от клиентов не требуется.
- После **3 последовательных таймаутов** игрок исключается и игра завершается.
- Игрок может добровольно пропустить ход через `POST /games/{id}/skip`.
- Игра также завершается автоматически, когда поле заполнено (все 25 клеток заняты).

### Проверка слов

Отправляемые слова должны:
- Содержать **3 или более букв**
- Включать новую поставленную букву
- Состоять из букв, отслеживаемых на доске (только соседние клетки)
- Существовать во встроенном словаре русских существительных
- Не использоваться повторно в текущей игре
- Не совпадать с начальным словом на доске

> **Примечание:** `е` и `ё` считаются одной буквой при поиске в словаре, проверке повторов и отображении на доске. Например, слово с `ё` совпадёт со словарной записью с `е`, и наоборот.

### Конечный автомат

Каждая игра запускает цикл FSM (`Game.Run`), управляемый значениями `TurnEvent`, отправляемыми по внутреннему каналу.

```
┌─────────────────────┬────────────────────┬─────────────────────┐
│ Состояние           │ Событие            │ Следующее состояние  │
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

- 60-секундный таймер автоматически вызывает `TurnTimeout`. Координатор (`internal/gamecoord/`) подтверждает его через `AckTimeout`, переходя к следующему игроку.
- `MoveSubmitted` и `TurnSkipped` сбрасывают счётчик последовательных таймаутов.
- После третьего подряд таймаута игра автоматически ставит в очередь `Kick` → `GameOver`.

---

## Схема базы данных

**users**

| Столбец | Тип | Примечания |
|---------|-----|------------|
| user_id | bigserial | PK |
| first_name | text | |
| last_name | text | |
| email | text | уникальный |
| hash_password | text | bcrypt через pgcrypto |
| api_key | uuid | |
| confirmed | boolean | по умолчанию false |
| created_at | timestamp | |
| updated_at | timestamp | |

**user_state**

| Столбец | Тип | Примечания |
|---------|-----|------------|
| user_id | bigint | PK, FK → users |
| nickname | text | автогенерируемый |
| exp | bigint | очки опыта |
| flags | bigint | флаги функций |
| lives | bigint | |
| created_at | timestamp | |
| updated_at | timestamp | |

---

## Справочник по конфигурации

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--server.addr` | `127.0.0.1` | Адрес привязки |
| `--server.port` | `9666` | HTTP-порт |
| `--server.x_api_token` | | API-ключ для запросов |
| `--pg.host` | `127.0.0.1` | Хост PostgreSQL |
| `--pg.port` | `5432` | Порт PostgreSQL |
| `--pg.user` | | Пользователь PostgreSQL |
| `--pg.database` | | База данных PostgreSQL |
| `--pg.password` | | Пароль PostgreSQL |
| `--pg.max_pool_size` | `10` | Максимальный размер пула соединений |
| `--pg.ssl` | `disable` | Режим SSL PostgreSQL |
| `--redis.addr` | `127.0.0.1:6379` | Адрес Redis |
| `--redis.username` | | Имя пользователя Redis |
| `--redis.password` | | Пароль Redis |
| `--redis.db_num` | `0` | Номер базы данных Redis |
| `--redis.expiration` | `30s` | Длительность сессии |
| `--centrifugo.api_url` | | URL HTTP API Centrifugo |
| `--centrifugo.api_key` | | API-ключ Centrifugo |
| `--centrifugo.token_hmac_secret_key` | | Секрет для подписи токенов Centrifugo |
| `MIGRATION_CONN_STRING` | | DSN PostgreSQL для миграций (переменная окружения) |

## Лицензия

[Apache 2.0](LICENSE)
