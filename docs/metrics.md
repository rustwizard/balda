# Metrics Plan

План подключения метрик в Balda. Цель — наблюдаемость рантайма, HTTP-слоя и
игровых процессов через Prometheus + Grafana, с минимумом новых зависимостей.

## Решения (зафиксировано)

- **Экспорт:** Prometheus pull — OpenTelemetry SDK + `otel/exporters/prometheus`,
  эндпоинт `/metrics`, который скрейпит Prometheus.
- **Эндпоинт:** отдельный внутренний listener (порт по умолчанию `:9100`), **не**
  на публичном API-порту `:9666`. Метрики не торчат наружу.
- **Один реестр на всё:** и авто-метрики ogen, и кастомные доменные метрики идут
  через один `MeterProvider` → один `/metrics`.

## Текущее состояние (факты)

- **ogen уже инструментирует HTTP-слой** OpenTelemetry-метриками: счётчик запросов,
  счётчик ошибок, гистограмма длительности по операциям (`internal/server/ogen/oas_cfg_gen.go`:
  `requests`/`errors`/`duration`, имена через `otelogen`). Сейчас пишутся в **no-op**
  `otel.GetMeterProvider()`. Достаточно передать реальный provider через
  `baldaapi.WithMeterProvider(mp)` — HTTP-метрики появятся без ручной инструментации.
- В `go.mod` уже есть `go.opentelemetry.io/otel` (core/metric/trace). **Нет** SDK-метрик
  и Prometheus-экспортёра — их надо добавить.
- В `cmd/server.go` уже есть `mux` и graceful shutdown (можно добавить второй listener
  для метрик и закрывать его в Shutdown).
- `pgxpool.Pool.Stat()` даёт статистику пула (для DB-метрик). Лобби/presence — in-memory.

## Целевая архитектура

```
balda server
 ├─ :9666  публичный API (ogen) ── метрики ogen ──┐
 ├─ доменный код (lobby/game/auth/...) ── instruments ┤
 │                                                    ▼
 │                                          OTel MeterProvider
 │                                          (SDK + prometheus exporter)
 └─ :9100  /metrics  ◀──────────────────────────────┘
                ▲
        Prometheus scrape ──▶ Grafana
```

## Фаза 1 — Фундамент (почти бесплатно)

Новый пакет `internal/metrics`:
- `Setup(cfg) (*MeterProvider, http.Handler, error)` — собирает `sdkmetric.MeterProvider`
  с `prometheus.New()` экспортёром (`go.opentelemetry.io/otel/exporters/prometheus`),
  возвращает provider + `promhttp` handler.
- Регистрирует Go-runtime метрики через `go.opentelemetry.io/contrib/instrumentation/runtime`
  (goroutines, GC, heap, и т.д.).

`cmd/server.go`:
- Поднять provider, передать в ogen: `baldaapi.WithMeterProvider(mp)`.
- Поднять **второй** `http.Server` на `cfg.Metrics.Addr` с `/metrics`, закрывать в shutdown
  (как `httpSrv`), учитывать в общем graceful-пути.
- Флаги: `--metrics.addr` (env `METRICS_ADDR`, default `127.0.0.1:9100`),
  `--metrics.enabled` (default true).

docker-compose:
- Сервис `prometheus` (`prom/prometheus`), `prometheus.yml` со скрейпом `server:9100`.
- (опц.) `grafana` с провиженингом datasource.

**Результат фазы 1:** HTTP RED-метрики (по операциям) + рантайм Go — без ручной инструментации.

## Фаза 2 — Доменные метрики

Инструменты определяются в `internal/metrics` (или рядом с доменом) и инжектируются туда,
где события происходят. Имена даны в OTel-нотации; Prometheus-экспортёр отрендерит их как
`balda_*` (точки → подчёркивания, к счётчикам добавится `_total`).

| Метрика (OTel) | Тип | Лейблы | Где инкрементить |
|----------------|-----|--------|------------------|
| `balda.games.active` | gauge (observable) | — | `lobby` (len реестра) |
| `balda.games.created` | counter | — | `lobby.Create` |
| `balda.games.started` | counter | `mode=pvp\|bot` | `lobby.StartGame/Join` |
| `balda.games.finished` | counter | `reason=game_finished\|kick\|accept_end` | `gamecoord` (game_over) |
| `balda.moves.submitted` | counter | — | `game.SubmitWord` (успех) |
| `balda.moves.rejected` | counter | `reason=too_short\|not_in_dict\|gaps\|...` | `game.SubmitWord` (ошибки) |
| `balda.turns.timeout` | counter | `offline=true\|false` | `gamecoord.NotifyTimeout` |
| `balda.turns.skipped` | counter | — | `gamecoord.NotifySkip` |
| `balda.players.kicked` | counter | — | `gamecoord.NotifyKick` |
| `balda.auth.signups` / `.logins` / `.refreshes` | counter | — | `handlers` auth/signup/refresh |
| `balda.auth.refresh_replays` | counter | — | `handlers.RefreshToken` (replay-ветка) — security-сигнал |
| `balda.auth.failures` | counter | `kind=bad_credentials\|invalid_token` | `handlers` |
| `balda.centrifugo.publish` | counter | `event`, `result=ok\|error` | `centrifugo.Client.Publish` |
| `balda.centrifugo.publish.duration` | histogram | `event` | `centrifugo.Client.Publish` |
| `balda.presence.online` | gauge (observable) | — | `presence` (опц., SCAN дорог — можно отложить) |
| `balda.db.pool.*` | gauge (observable) | — | `pgxpool.Stat()`: acquired/idle/total/max, wait count/duration |

Подход: завести `metrics.Game`, `metrics.Auth`, `metrics.RT` структуры с уже созданными
инструментами; передавать их в соответствующие компоненты (DI, как `Notifier`). Доменные
пакеты (`game`) остаются чистыми — инкремент через узкий интерфейс/колбэк или через
`gamecoord` (который уже видит все события FSM и подходит как точка съёма игровых метрик).

> **Заметка по чистоте:** большинство игровых метрик удобнее снимать в `gamecoord`
> (он уже реализует `Notifier` и видит timeout/skip/kick/turn/game_over), не трогая
> пакет `game`. Это повторяет приём из #21 (presence через интерфейс).

## Фаза 3 — Инфраструктура и дашборды

- **Centrifugo** отдаёт собственные Prometheus-метрики — включить в его конфиге, добавить
  target в Prometheus.
- (опц.) `postgres_exporter`, `redis_exporter` сервисами в compose — метрики БД/Redis вне Go.
- **Grafana-дашборды:** HTTP RED (rate/errors/duration по операциям), воронка игр
  (created→started→finished, причины), рантайм Go, пул БД, Centrifugo.
- Алерты (позже): рост `auth.refresh_replays`, всплеск 5xx, насыщение пула БД, отсутствие
  скрейпа (`up == 0`).

## Зависимости (добавить)

- `go.opentelemetry.io/otel/sdk/metric`
- `go.opentelemetry.io/otel/exporters/prometheus`
- `go.opentelemetry.io/contrib/instrumentation/runtime`
- `github.com/prometheus/client_golang` (приходит транзитивно с prometheus-экспортёром)

После `go get` — `go mod vendor` (проект на vendor).

## Безопасность

- `/metrics` только на внутреннем интерфейсе (`127.0.0.1:9100` / внутренняя docker-сеть),
  не за публичным API. В compose Prometheus ходит по внутренней сети, порт наружу не
  публикуется.
- Метки без PII: использовать `player_id`/`game_id` как метки **нельзя** (высокая
  кардинальность + утечка) — только агрегаты и низкокардинальные лейблы (reason, event, mode).

## Открытые вопросы / на будущее

- Трейсинг: ogen уже умеет (`WithTracerProvider`) — можно добавить OTLP-экспортёр трейсов
  позже (отдельная задача).
- Exemplars (связка метрика↔трейс) — после трейсинга.
- Если появится OTel Collector — переключить экспорт на OTLP push без изменения инструментации.
