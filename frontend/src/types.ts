// Generated from api/openapi/http-api.yaml

export interface Player {
  uid: string;
  firstname: string;
  lastname: string;
  sid: string;
  key: string;
  exp?: number;
}

export interface LobbyPlayer {
  uid: string;
  exp?: number;
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
  players?: LobbyPlayer[];
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
  board?: string[][];
  current_turn_uid?: string;
}

export interface ListGamesResponse {
  games: GameSummary[];
}

export interface PlayerGameState {
  uid: string;
  exp?: number;
  exp_gained?: number;
  score: number;
  words_count: number;
  words?: string[];
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
  players: PlayerGameState[];
  status: GameStatus;
  move_number: number;
}

export interface EvGameOver {
  type: 'game_over';
  game_id: string;
  winner_uid?: string | null;
  players: PlayerGameState[];
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

export interface EvTurnChange {
  type: 'turn_change';
  game_id: string;
  current_turn_uid: string;
  reason: 'game_start' | 'move' | 'skip' | 'timeout';
}

export interface BoardCell {
  row: number;
  col: number;
}

export interface MoveRequest {
  new_letter: {
    row: number;
    col: number;
    char: string;
  };
  word_path: BoardCell[];
}

export interface MoveResponse {
  board: string[][];
  current_turn_uid: string;
  players: PlayerGameState[];
  status: GameStatus;
  move_number: number;
}

export interface EvLobbyUpdate {
  type: 'lobby_update';
  games: GameSummary[];
}

export interface EvSkipWarn {
  type: 'skip_warn';
  game_id: string;
  player_uid: string;
  skips_used: number;
  skips_left: number;
}

export type CentrifugoEvent = EvGameState | EvGameOver | EvGameCreated | EvGameStarted | EvTurnChange | EvSkipWarn | EvLobbyUpdate;
