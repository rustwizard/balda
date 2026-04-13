// Generated from api/openapi/http-api.yaml

export interface Player {
  uid: string;
  firstname: string;
  lastname: string;
  sid: string;
  key: string;
}

export interface SignupRequest {
  firstname: string;
  lastname: string;
  email: string;
  password: string;
}

export interface SignupResponse {
  user: Player;
  centrifugo_token?: string;
  lobby_token?: string;
}

export interface AuthRequest {
  email: string;
  password: string;
}

export interface AuthResponse {
  player: Player;
  centrifugo_token?: string;
  lobby_token?: string;
}

export type GameStatus = 'waiting' | 'in_progress' | 'finished';

export interface GameSummary {
  id: string;
  player_ids: string[];
  status: GameStatus;
  started_at: number;
}

export interface CreateGameResponse {
  game: GameSummary;
  game_token?: string;
}

export interface JoinGameResponse {
  game: GameSummary;
  game_token?: string;
}

export interface ListGamesResponse {
  games: GameSummary[];
}

export interface PlayerScore {
  uid: string;
  score: number;
}

export interface PlayerState {
  uid: string;
  nickname: string;
  exp: number;
  lives: number;
  flags: number;
  game_id?: string;
}

export interface EvGameState {
  type: 'game_state';
  game_id: string;
  board: string[][];
  current_turn_uid: string;
  players: PlayerScore[];
  status: GameStatus;
  move_number: number;
}

export interface EvGameOver {
  type: 'game_over';
  game_id: string;
  winner_uid?: string | null;
  players: PlayerScore[];
}

export interface EvGameCreated {
  type: 'game_created';
  game_id: string;
  status: GameStatus;
  player_ids: string[];
}

export interface EvGameStarted {
  type: 'game_started';
  game_id: string;
  status: GameStatus;
  player_ids: string[];
  started_at: number;
}

export type CentrifugoEvent = EvGameState | EvGameOver | EvGameCreated | EvGameStarted;
