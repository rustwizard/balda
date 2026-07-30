// Thin wrapper around the Telegram Mini App JS API. Outside the Telegram
// WebView the global is absent and every helper degrades to a no-op.

interface TelegramWebApp {
  initData: string;
  ready(): void;
  expand(): void;
}

interface TelegramWindow extends Window {
  Telegram?: { WebApp?: TelegramWebApp };
}

function webApp(): TelegramWebApp | null {
  return (window as TelegramWindow).Telegram?.WebApp ?? null;
}

// isMiniApp reports whether the page runs inside the Telegram WebView with
// signed init data available (empty string means the app was opened outside
// a bot context).
export function isMiniApp(): boolean {
  const app = webApp();
  return !!app && app.initData.length > 0;
}

// getTelegramInitData returns the signed init data to send to the backend,
// or null when not running as a Mini App.
export function getTelegramInitData(): string | null {
  const app = webApp();
  return app && app.initData.length > 0 ? app.initData : null;
}

// initMiniApp tells Telegram the app is ready and asks for the full-height
// layout. Safe to call outside the WebView.
export function initMiniApp(): void {
  const app = webApp();
  if (!app) return;
  app.ready();
  app.expand();
}
