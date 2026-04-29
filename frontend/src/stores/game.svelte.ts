import type { GameSummary, PlayerGameState, EvGameState, EvGameOver, EvTurnChange, EvEndProposalResult, EvEndProposal, EvSkipWarn, EvLobbyUpdate, MoveResponse } from '../types';

export type GamePhase = 'auth' | 'lobby' | 'waiting' | 'playing' | 'finished';

export interface PlayerInfo {
  uid: string;
  nickname: string;
  exp: number;
  expGained: number;
  score: number;
  wordsCount: number;
  words: string[];
  consecutiveSkips: number;
}

export function createGameState() {
  // Auth & session
  let apiKey = $state<string>('');
  let sessionId = $state<string>('');
  let playerUid = $state<string>('');
  let nickname = $state<string>('');
  let exp = $state<number>(0);
  let centrifugoToken = $state<string>('');
  let lobbyToken = $state<string>('');

  // Game
  let phase = $state<GamePhase>('auth');
  let game = $state<GameSummary | null>(null);
  let board = $state<string[][]>([]);
  let currentTurnUid = $state<string>('');
  let players = $state<PlayerInfo[]>([]);
  let moveNumber = $state<number>(0);
  let winnerUid = $state<string | null | undefined>(null);

  // In-game notification (replaces browser alert)
  let notif = $state<{ message: string; kind: 'error' | 'warn' } | null>(null);

  // Lobby game list — updated via lobby_update Centrifugo events
  let lobbyGames = $state<GameSummary[]>([]);

  // End proposal
  let endProposalPending = $state<boolean>(false);
  let endProposalByMe = $state<boolean>(false);

  // Turn interaction
  let selectedPath = $state<{ row: number; col: number }[]>([]);
  let newLetterCell = $state<{ row: number; col: number } | null>(null);
  let currentWord = $state<string>('');
  let turnSecondsLeft = $state<number>(60);
  let moveLoading = $state<boolean>(false);

  // Derived
  const isMyTurn = $derived(currentTurnUid === playerUid);
  const myPlayer = $derived(players.find((p) => p.uid === playerUid));
  const opponent = $derived(players.find((p) => p.uid !== playerUid));

  function resetBoard() {
    board = Array.from({ length: 5 }, () => Array(5).fill(''));
  }

  function setAuth(data: { apiKey: string; sessionId: string; playerUid: string; nickname: string; exp: number; centrifugoToken: string; lobbyToken: string }) {
    apiKey = data.apiKey;
    sessionId = data.sessionId;
    playerUid = data.playerUid;
    nickname = data.nickname;
    exp = data.exp;
    centrifugoToken = data.centrifugoToken;
    lobbyToken = data.lobbyToken;
  }

  function setLobby() {
    phase = 'lobby';
    game = null;
    resetBoard();
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
    winnerUid = null;
    endProposalPending = false;
    endProposalByMe = false;
  }

  function setWaiting(g: GameSummary) {
    game = g;
    phase = 'waiting';
  }

  function startGame(g: GameSummary) {
    game = g;
    phase = 'playing';
    resetBoard();
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
    winnerUid = null;
    turnSecondsLeft = 60;
    if (g.players && g.players.length > 0) {
      players = g.players.map((p) => ({
        uid: p.uid,
        nickname: p.uid === playerUid ? nickname : 'Соперник',
        exp: p.exp ?? 0,
        expGained: 0,
        score: 0,
        wordsCount: 0,
        words: [],
        consecutiveSkips: 0,
      }));
    } else {
      players = g.player_ids.map((uid) => ({
        uid,
        nickname: uid === playerUid ? nickname : 'Соперник',
        exp: 0,
        expGained: 0,
        score: 0,
        wordsCount: 0,
        words: [],
        consecutiveSkips: 0,
      }));
    }
  }

  function mergePlayerState(p: PlayerGameState): PlayerInfo {
    const existing = players.find((ep) => ep.uid === p.uid);
    return {
      uid: p.uid,
      nickname: existing?.nickname || (p.uid === playerUid ? nickname : 'Соперник'),
      // Use Math.max so that a transient 0 from the server never wipes a
      // previously-known positive EXP value.
      exp: Math.max(p.exp ?? 0, existing?.exp ?? 0),
      expGained: p.exp_gained ?? 0,
      score: p.score,
      wordsCount: p.words_count ?? 0,
      words: p.words ?? existing?.words ?? [],
      consecutiveSkips: existing?.consecutiveSkips ?? 0,
    };
  }

  function applyGameState(ev: EvGameState) {
    board = ev.board;
    currentTurnUid = ev.current_turn_uid;
    moveNumber = ev.move_number;
    players = ev.players.map(mergePlayerState);
    if (ev.status === 'finished') {
      phase = 'finished';
    }
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
    turnSecondsLeft = 60;
    notif = null;
  }

  function finishGame(ev: EvGameOver) {
    phase = 'finished';
    winnerUid = ev.winner_uid;
    players = ev.players.map(mergePlayerState);
  }

  function applyEndProposal(ev: EvEndProposal) {
    endProposalPending = true;
    endProposalByMe = ev.proposer_uid === playerUid;
  }

  function applyEndProposalResult(ev: EvEndProposalResult) {
    endProposalPending = false;
    endProposalByMe = false;
    if (!ev.accepted) {
      turnSecondsLeft = Math.ceil((ev.remaining_ms ?? 0) / 1000);
    }
  }

  function applySkipWarn(ev: EvSkipWarn) {
    players = players.map((p) =>
      p.uid === ev.player_uid ? { ...p, consecutiveSkips: ev.skips_used } : p
    );
  }

  function applyTurnChange(ev: EvTurnChange) {
    currentTurnUid = ev.current_turn_uid;
    turnSecondsLeft = 60;
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
    endProposalPending = false;
    endProposalByMe = false;
  }

  function applyMoveResponse(resp: MoveResponse) {
    board = resp.board;
    currentTurnUid = resp.current_turn_uid;
    moveNumber = resp.move_number;
    players = resp.players.map((p) => ({ ...mergePlayerState(p), consecutiveSkips: 0 }));
    if (resp.status === 'finished') {
      phase = 'finished';
    }
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
    turnSecondsLeft = 60;
    notif = null;
  }

  function setTurnTimer(seconds: number) {
    turnSecondsLeft = seconds;
  }

  function tickTimer() {
    if (turnSecondsLeft > 0) turnSecondsLeft--;
  }

  // Interaction helpers
  function selectCell(row: number, col: number) {
    const idx = selectedPath.findIndex((p) => p.row === row && p.col === col);
    if (idx >= 0) {
      // Deselect and everything after
      selectedPath = selectedPath.slice(0, idx);
    } else {
      const last = selectedPath[selectedPath.length - 1];
      if (!last) {
        selectedPath = [{ row, col }];
      } else {
        const dr = Math.abs(last.row - row);
        const dc = Math.abs(last.col - col);
        if (dr + dc === 1) {
          selectedPath = [...selectedPath, { row, col }];
        }
      }
    }
    rebuildWord();
  }

  function rebuildWord() {
    currentWord = selectedPath.map((p) => board[p.row][p.col]).join('');
  }

  function setNewLetterCell(row: number, col: number) {
    newLetterCell = { row, col };
  }

  function setLetterAtCell(letter: string) {
    if (newLetterCell) {
      board[newLetterCell.row][newLetterCell.col] = letter;
      rebuildWord();
    }
  }

  function undoNewLetter() {
    if (newLetterCell) {
      board[newLetterCell.row][newLetterCell.col] = '';
      newLetterCell = null;
      rebuildWord();
    }
  }

  function clearSelection() {
    selectedPath = [];
    currentWord = '';
  }

  function applyLobbyUpdate(ev: EvLobbyUpdate) {
    lobbyGames = ev.games;
  }

  function setLobbyGames(gs: GameSummary[]) {
    lobbyGames = gs;
  }

  function showNotif(message: string, kind: 'error' | 'warn' = 'error') {
    notif = { message, kind };
  }

  function clearNotif() {
    notif = null;
  }

  return {
    get apiKey() { return apiKey; },
    get sessionId() { return sessionId; },
    get playerUid() { return playerUid; },
    get nickname() { return nickname; },
    get exp() { return exp; },
    get centrifugoToken() { return centrifugoToken; },
    get lobbyToken() { return lobbyToken; },
    get phase() { return phase; },
    get game() { return game; },
    get board() { return board; },
    get currentTurnUid() { return currentTurnUid; },
    get players() { return players; },
    get moveNumber() { return moveNumber; },
    get winnerUid() { return winnerUid; },
    get selectedPath() { return selectedPath; },
    get newLetterCell() { return newLetterCell; },
    get currentWord() { return currentWord; },
    get turnSecondsLeft() { return turnSecondsLeft; },
    get moveLoading() { return moveLoading; },
    get isMyTurn() { return isMyTurn; },
    get myPlayer() { return myPlayer; },
    get opponent() { return opponent; },
    get notif() { return notif; },
    get lobbyGames() { return lobbyGames; },
    get endProposalPending() { return endProposalPending; },
    get endProposalByMe() { return endProposalByMe; },

    setAuth,
    setLobby,
    setWaiting,
    startGame,
    applyGameState,
    applyMoveResponse,
    applyTurnChange,
    applyEndProposal,
    applyEndProposalResult,
    applySkipWarn,
    applyLobbyUpdate,
    setLobbyGames,
    finishGame,
    setTurnTimer,
    tickTimer,
    selectCell,
    setNewLetterCell,
    setLetterAtCell,
    undoNewLetter,
    clearSelection,
    showNotif,
    clearNotif,
    setMoveLoading(value: boolean) { moveLoading = value; },
  };
}

export const gameState = createGameState();
