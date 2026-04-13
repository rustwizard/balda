# Balda Frontend

Фронтенд для игры [Балда](https://github.com/rustwizard/balda) — мультиплеерной словесной игры.

## Стек

- **Svelte 5** (Runes) — реактивность
- **TypeScript** — типобезопасность
- **Vite** — сборка и dev-сервер
- **Tailwind CSS v4** — стили
- **Centrifuge-js** — WebSocket / real-time

## Структура

```
src/
├── components/      # UI-компоненты
│   ├── AuthForm.svelte
│   ├── Board.svelte
│   ├── GameScreen.svelte
│   ├── Icon.svelte
│   ├── Lobby.svelte
│   ├── PlayerCard.svelte
│   ├── Timer.svelte
│   ├── WaitingScreen.svelte
│   └── WordBar.svelte
├── lib/
│   ├── api.ts       # REST-клиент
│   └── centrifugo.ts # WebSocket-клиент
├── stores/
│   └── game.svelte.ts # Игровое состояние (Svelte 5 Runes)
├── types.ts         # Модели API
├── App.svelte
└── main.ts
```

## Установка

```bash
cd frontend
npm install
```

## Запуск

```bash
# Dev-сервер с прокси на бэкенд (порт 5173)
npm run dev

# Сборка
npm run build

# Проверка типов
npm run check
```

## Прокси

В `vite.config.ts` настроены прокси:
- `/balda/api/v1` → `http://127.0.0.1:9666` (Go-сервер)
- `/api` → `http://127.0.0.1:8000` (Centrifugo)

## Генерация типов из OpenAPI

```bash
npx openapi-typescript ../api/openapi/http-api.yaml -o src/types-api.ts
```

## TODO

- [ ] Эндпоинты для хода (`POST /games/{id}/move` или RPC)
- [ ] Интеграция с Telegram Mini App SDK
- [ ] Звуковые эффекты
- [ ] Анимации фишек
