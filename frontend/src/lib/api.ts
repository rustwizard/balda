import type {
  AuthRequest,
  AuthResponse,
  SignupRequest,
  SignupResponse,
  CreateGameResponse,
  JoinGameResponse,
  ListGamesResponse,
  PlayerState,
} from '../types';

const API_BASE = '/balda/api/v1';

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function apiFetch<T>(path: string, options?: RequestInit, apiKey?: string, sessionId?: string): Promise<T> {
  const headers = new Headers(options?.headers);
  headers.set('Content-Type', 'application/json');
  if (apiKey) headers.set('X-API-Key', apiKey);
  if (sessionId) headers.set('X-API-Session', sessionId);

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

export function auth(data: AuthRequest, apiKey: string): Promise<AuthResponse> {
  return apiFetch('/auth', { method: 'POST', body: JSON.stringify(data) }, apiKey);
}

export function ping(apiKey: string, sessionId: string, requestId: number): Promise<void> {
  return apiFetch('/session/ping', { method: 'POST' }, apiKey, sessionId);
}

export function getPlayerState(uid: string): Promise<PlayerState> {
  return apiFetch(`/player/state/${uid}`);
}

export function listGames(apiKey: string, sessionId: string): Promise<ListGamesResponse> {
  return apiFetch('/games', { method: 'GET' }, apiKey, sessionId);
}

export function createGame(apiKey: string, sessionId: string): Promise<CreateGameResponse> {
  return apiFetch('/games', { method: 'POST' }, apiKey, sessionId);
}

export function joinGame(gameId: string, apiKey: string, sessionId: string): Promise<JoinGameResponse> {
  return apiFetch(`/games/${gameId}/join`, { method: 'POST' }, apiKey, sessionId);
}
