import type {
  AuthRequest,
  AuthResponse,
  SignupRequest,
  SignupResponse,
  RefreshResponse,
  CreateGameResponse,
  JoinGameResponse,
  ListGamesResponse,
  PlayerState,
  MoveRequest,
  MoveResponse,
} from '../types';

const API_BASE = '/balda/api/v1';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

// JWT access token attached as `Authorization: Bearer` to every protected
// request, plus the opaque refresh token used to renew it. Held in module
// scope so callers don't thread credentials through every call.
let accessToken = '';
let refreshToken = '';

export function setTokens(access: string, refresh: string) {
  accessToken = access;
  refreshToken = refresh;
}

export function clearTokens() {
  accessToken = '';
  refreshToken = '';
}

export function getRefreshToken(): string {
  return refreshToken;
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const headers = new Headers(options?.headers);
  headers.set('Content-Type', 'application/json');
  if (accessToken) headers.set('Authorization', `Bearer ${accessToken}`);

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(res.status, body.message || `HTTP ${res.status}`);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return res.json() as Promise<T>;
}

export function signup(data: SignupRequest): Promise<SignupResponse> {
  return apiFetch('/signup', { method: 'POST', body: JSON.stringify(data) });
}

export function auth(data: AuthRequest): Promise<AuthResponse> {
  return apiFetch('/auth', { method: 'POST', body: JSON.stringify(data) });
}

export function refresh(): Promise<RefreshResponse> {
  return apiFetch('/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}

export function logout(): Promise<void> {
  return apiFetch('/auth/logout', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}

export function ping(requestId: number): Promise<void> {
  return apiFetch('/session/ping', {
    method: 'POST',
    headers: { 'X-Request-ID': String(requestId) },
  });
}

export function getPlayerState(uid: string): Promise<PlayerState> {
  return apiFetch(`/player/state/${uid}`);
}

export function listGames(): Promise<ListGamesResponse> {
  return apiFetch('/games', { method: 'GET' });
}

export function createGame(): Promise<CreateGameResponse> {
  return apiFetch('/games', { method: 'POST' });
}

export function createGameWithBot(): Promise<JoinGameResponse> {
  return apiFetch('/games/with-bot', { method: 'POST' });
}

export function joinGame(gameId: string): Promise<JoinGameResponse> {
  return apiFetch(`/games/${gameId}/join`, { method: 'POST' });
}

export function submitMove(gameId: string, payload: MoveRequest): Promise<MoveResponse> {
  return apiFetch(`/games/${gameId}/move`, { method: 'POST', body: JSON.stringify(payload) });
}

export function skipTurn(gameId: string): Promise<void> {
  return apiFetch(`/games/${gameId}/skip`, { method: 'POST' });
}

export function proposeEnd(gameId: string): Promise<void> {
  return apiFetch(`/games/${gameId}/propose-end`, { method: 'POST' });
}

export function acceptEnd(gameId: string): Promise<void> {
  return apiFetch(`/games/${gameId}/accept-end`, { method: 'POST' });
}

export function rejectEnd(gameId: string): Promise<void> {
  return apiFetch(`/games/${gameId}/reject-end`, { method: 'POST' });
}
