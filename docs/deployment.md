# Деплой в прод (одна VPS)

Минимальный маршрут «Балды» в прод: одна VPS, Caddy с авто-HTTPS, весь стек
в docker-compose. Подходит и для раздачи ссылки напрямую, и как база для
Telegram Mini App.

## Что понадобится

- VPS 1 vCPU / 2 ГБ RAM / 20 ГБ SSD (Ubuntu 24.04) — ~150–500 ₽/мес.
- Домен с A-записью на IP сервера (или бесплатный субдомен DuckDNS).
  DNS должен пропагироваться **до** первого запуска — Caddy сразу пойдёт
  за сертификатом.

## Первичный деплой

```bash
# 1. Docker
curl -fsSL https://get.docker.com | sh

# 2. Код
git clone https://github.com/rustwizard/balda.git /opt/balda
cd /opt/balda

# 3. Секреты
cp .env.prod.example .env
$EDITOR .env
# DOMAIN — твой домен без https://
# остальное сгенерировать: openssl rand -hex 32

# 4. Запуск
docker compose -f docker-compose.prod.yml up --build -d

# 5. Проверка
docker compose -f docker-compose.prod.yml ps
curl -I https://$DOMAIN          # 200, отдаёт index.html
curl https://$DOMAIN/balda/api/v1/player/stats   # 401 Unauthorized — API жив
```

Миграции накатываются автоматически при старте `server` (нужен
`MIGRATION_CONN_STRING` — уже собран в compose из `PG_PASSWORD`).

Наружу торчат только порты 80/443 (Caddy). Postgres, Redis, Centrifugo и API
доступны только внутри docker-сети. Маршрутизация:
`Caddy → frontend(nginx) → /balda/api/v1/* → server`,
`/connection/websocket → centrifugo`.

## Обновление

```bash
cd /opt/balda
git pull
docker compose -f docker-compose.prod.yml up --build -d
```

Откат: `git checkout <sha>` + та же команда. Миграции только вперёд —
перед обновлением смажь бэкап (см. ниже).

## Бэкапы Postgres

```bash
crontab -e
# ежедневно в 03:17:
17 3 * * * /opt/balda/scripts/backup.sh >> /var/log/balda-backup.log 2>&1
```

Дампы: `/opt/balda/backups/balda-*.sql.gz`, хранятся 14 дней. Восстановление:

```bash
gunzip -c backups/balda-XXXX.sql.gz | \
  docker compose -f docker-compose.prod.yml exec -T pg psql -U balda -d balda
```

Для паранойи раз в неделю уносить `backups/` куда-нибудь наружу (rsync на
домашнюю машину / S3).

## Логи и диагностика

```bash
docker compose -f docker-compose.prod.yml logs -f server
docker compose -f docker-compose.prod.yml logs --tail 200 centrifugo
```

Логи ограничены 10 МБ × 3 файла на сервис (настроено в compose).

## Telegram Mini App (следующий шаг после прода)

1. `@BotFather` → `/newbot` — получить токен бота (вида `123456:ABC-...`).
2. Вписать токен в `.env` (`TELEGRAM_BOT_TOKEN`) и перезапустить:
   `docker compose -f docker-compose.prod.yml up -d`.
   Без токена `/auth/telegram` отвечает 503, остальное работает как раньше.
3. `/newapp` (или «Bot Settings → Mini Apps») — указать `https://$DOMAIN`
   как URL мини-аппа, выбрать короткое имя ссылки (вида `t.me/<bot>/<app>`).
4. Пользователь открывает Mini App → фронт обменивает подписанный `initData`
   на нашу сессию через `POST /auth/telegram` — без форм и паролей; первый
   визит создаёт пользователя (привязка по `telegram_id`).
5. Menu button бота (Bot Settings → Menu Button) — «Играть» с тем же URL.

Регистрация по email в проде выключена (`AUTH_EMAIL_SIGNUP_ENABLED: "false"`
в `docker-compose.prod.yml`) — это локально-тестовый способ входа. Логин
существующих аккаунтов при этом работает. Флаг отдаётся клиенту через
`GET /config`.

## Чего в этом контуре сознательно нет

- Мониторинга/алертов (Uptime Kuma или бесплатный uptimerobot.com наружу —
  добавить при первых пользователях).
- Горизонтального масштабирования (lobby и игры в памяти одного процесса).
- Rate limiting на signup/auth — до публичной раскрутки следить за логами.
