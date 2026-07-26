// Generated from api/openapi/http-api.yaml

export interface Player {
    uid: string;
    firstname: string;
    lastname: string;
    exp?: number;
    rating?: number;
}

export interface TokenPair {
    access_token?: string;
    refresh_token?: string;
    token_type?: string;
    expires_in?: number;
}

export interface LobbyPlayer {
    uid: string;
    exp?: number;
    rating?: number;
}

export interface SignupRequest {
    firstname: string;
    lastname: string;
    email: string;
    password: string;
}

export interface SignupResponse extends TokenPair {
    user: Player;
    centrifugo_token?: string;
    lobby_token?: string;
}

export interface AuthRequest {
    email: string;
    password: string;
}

export interface AuthResponse extends TokenPair {
    player: Player;
    centrifugo_token?: string;
    lobby_token?: string;
    active_game?: ActiveGame;
}

export interface RefreshResponse extends TokenPair {}

export type GameStatus = "waiting" | "in_progress" | "finished";

export interface GameSummary {
    id: string;
    player_ids: string[];
    players?: LobbyPlayer[];
    status: GameStatus;
    started_at: number;
}

export interface ActiveGame {
    game_id: string;
    game_token?: string;
    board?: string[][];
    current_turn_uid?: string;
    move_number?: number;
    status?: GameStatus;
    players?: PlayerGameState[];
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
    rating?: number;
    rating_gained?: number;
    exp_gained?: number;
    score: number;
    words_count: number;
    words?: string[];
}

export interface PlayerState {
    uid: string;
    nickname: string;
    exp: number;
    rating: number;
    lives: number;
    flags: number;
    game_id?: string;
}

export interface EvGameState {
    type: "game_state";
    game_id: string;
    board: string[][];
    current_turn_uid: string;
    players: PlayerGameState[];
    status: GameStatus;
    move_number: number;
}

export interface EvGameOver {
    type: "game_over";
    game_id: string;
    winner_uid?: string | null;
    players: PlayerGameState[];
    reason?: "kick" | "game_finished" | "accept_end";
}

export interface EvGameCreated {
    type: "game_created";
    game_id: string;
    status: GameStatus;
    player_ids: string[];
}

export interface EvGameStarted {
    type: "game_started";
    game_id: string;
    status: GameStatus;
    player_ids: string[];
    started_at: number;
}

export interface EvTurnChange {
    type: "turn_change";
    game_id: string;
    current_turn_uid: string;
    reason: "game_start" | "move" | "skip" | "timeout";
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
    type: "lobby_update";
    games: GameSummary[];
}

export interface EvEndProposalResult {
    type: "end_proposal_result";
    game_id: string;
    accepted: boolean;
    remaining_ms?: number;
}

export interface EvSkipWarn {
    type: "skip_warn";
    game_id: string;
    player_uid: string;
    skips_used: number;
    skips_left: number;
}

export interface EvEndProposal {
    type: "end_proposal";
    game_id: string;
    proposer_uid: string;
}

export type AchievementID =
    | "first_game"
    | "first_win"
    | "high_scorer_50"
    | "wordsmith_10"
    | "giant_word"
    | "winning_streak_3"
    | "veteran_10";

export interface Achievement {
    id: AchievementID;
    name: string;
    description: string;
    unlocked: boolean;
}

export interface PlayerAchievementsResponse {
    achievements: Achievement[];
}

export interface PlayerStats {
    games_played: number;
    wins: number;
    losses: number;
    draws: number;
    win_rate: number;
    avg_word_length: number;
    best_word: string;
    favorite_letter: string;
}

export interface EvAchievementUnlocked {
    type: "achievement_unlocked";
    game_id: string;
    player_uid: string;
    achievement_id: AchievementID;
    name: string;
}

export type CentrifugoEvent =
    | EvGameState
    | EvGameOver
    | EvGameCreated
    | EvGameStarted
    | EvTurnChange
    | EvSkipWarn
    | EvLobbyUpdate
    | EvEndProposalResult
    | EvEndProposal
    | EvAchievementUnlocked;
