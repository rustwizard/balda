import type { GameSummary, PlayerState, EvGameState, EvGameOver, EvTurnChange } from '../types';

export type GamePhase = 'auth' | 'lobby' | 'waiting' | 'playing' | 'finished';

export interface PlayerInfo {
  uid: string;
  nickname: string;
  score: number;
  wordsCount: number;
}

export function createGameState() {
  // Auth & session
  let apiKey = $state<string>('');
  let sessionId = $state<string>('');
  let playerUid = $state<string>('');
  let nickname = $state<string>('');
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

  // Turn interaction
  let selectedPath = $state<{ row: number; col: number }[]>([]);
  let newLetterCell = $state<{ row: number; col: number } | null>(null);
  let currentWord = $state<string>('');
  let turnSecondsLeft = $state<number>(60);

  // Derived
  const isMyTurn = $derived(currentTurnUid === playerUid);
  const myPlayer = $derived(players.find((p) => p.uid === playerUid));
  const opponent = $derived(players.find((p) => p.uid !== playerUid));

  function resetBoard() {
    board = Array.from({ length: 5 }, () => Array(5).fill(''));
  }

  function setAuth(data: { apiKey: string; sessionId: string; playerUid: string; nickname: string; centrifugoToken: string; lobbyToken: string }) {
    apiKey = data.apiKey;
    sessionId = data.sessionId;
    playerUid = data.playerUid;
    nickname = data.nickname;
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
    players = g.player_ids.map((uid) => ({
      uid,
      nickname: uid === playerUid ? nickname : 'Соперник',
      score: 0,
      wordsCount: 0,
    }));
  }

  function applyGameState(ev: EvGameState) {
    board = ev.board;
    currentTurnUid = ev.current_turn_uid;
    moveNumber = ev.move_number;
    players = ev.players.map((p) => {
      const existing = players.find((ep) => ep.uid === p.uid);
      return {
        uid: p.uid,
        nickname: existing?.nickname || (p.uid === playerUid ? nickname : 'Соперник'),
        score: p.score,
        wordsCount: p.score > 0 ? Math.floor(p.score / 4) : 0,
      };
    });
    if (ev.status === 'finished') {
      phase = 'finished';
    }
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
    turnSecondsLeft = 60;
  }

  function finishGame(ev: EvGameOver) {
    phase = 'finished';
    winnerUid = ev.winner_uid;
    players = ev.players.map((p) => {
      const existing = players.find((ep) => ep.uid === p.uid);
      return {
        uid: p.uid,
        nickname: existing?.nickname || (p.uid === playerUid ? nickname : 'Соперник'),
        score: p.score,
        wordsCount: p.score > 0 ? Math.floor(p.score / 4) : 0,
      };
    });
  }

  function applyTurnChange(ev: EvTurnChange) {
    currentTurnUid = ev.current_turn_uid;
    turnSecondsLeft = 60;
    selectedPath = [];
    newLetterCell = null;
    currentWord = '';
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

  function clearSelection() {
    selectedPath = [];
    currentWord = '';
  }

  return {
    get apiKey() { return apiKey; },
    get sessionId() { return sessionId; },
    get playerUid() { return playerUid; },
    get nickname() { return nickname; },
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
    get isMyTurn() { return isMyTurn; },
    get myPlayer() { return myPlayer; },
    get opponent() { return opponent; },

    setAuth,
    setLobby,
    setWaiting,
    startGame,
    applyGameState,
    applyTurnChange,
    finishGame,
    setTurnTimer,
    tickTimer,
    selectCell,
    setNewLetterCell,
    setLetterAtCell,
    clearSelection,
  };
}

export const gameState = createGameState();
