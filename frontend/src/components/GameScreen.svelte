<script lang="ts">
  import Board from './Board.svelte';
  import PlayerCard from './PlayerCard.svelte';
  import Timer from './Timer.svelte';
  import WordBar from './WordBar.svelte';
  import Icon from './Icon.svelte';
  import Alphabet from './Alphabet.svelte';
  import { gameState } from '../stores/game.svelte';
  import * as api from '../lib/api';

  let showAlphabet = $state(false);

  function handleCellClick(row: number, col: number) {
    if (!gameState.isMyTurn || gameState.moveLoading) return;

    const cell = gameState.board[row][col];
    if (!cell) {
      // Clicking the same empty cell again cancels the selection
      if (gameState.newLetterCell?.row === row && gameState.newLetterCell?.col === col) {
        gameState.undoNewLetter();
        showAlphabet = false;
        return;
      }
      // Empty cell - set as new letter placement and show alphabet
      gameState.setNewLetterCell(row, col);
      showAlphabet = true;
      return;
    }

    gameState.selectCell(row, col);
  }

  function handleAlphabetSelect(char: string) {
    gameState.setLetterAtCell(char);
    showAlphabet = false;
  }

  function handleAlphabetCancel() {
    gameState.undoNewLetter();
    showAlphabet = false;
  }

  async function handleSkip() {
    if (!gameState.isMyTurn || gameState.moveLoading || !gameState.game) return;
    gameState.setMoveLoading(true);
    try {
      await api.skipTurn(gameState.game.id, gameState.apiKey, gameState.sessionId);
      gameState.clearSelection();
      gameState.undoNewLetter();
    } catch (err: any) {
      gameState.showNotif(err?.message || 'Не удалось пропустить ход');
    } finally {
      gameState.setMoveLoading(false);
    }
  }

  async function handleSubmit() {
    if (!gameState.isMyTurn || gameState.moveLoading || !gameState.game) return;
    if (!gameState.newLetterCell) {
      gameState.showNotif('Выберите клетку для новой буквы', 'warn');
      return;
    }
    if (gameState.currentWord.length < 3) {
      gameState.showNotif('Слово должно быть минимум из 3 букв', 'warn');
      return;
    }

    const payload = {
      new_letter: {
        row: gameState.newLetterCell.row,
        col: gameState.newLetterCell.col,
        char: gameState.board[gameState.newLetterCell.row][gameState.newLetterCell.col],
      },
      word_path: gameState.selectedPath.map((p) => ({ row: p.row, col: p.col })),
    };

    gameState.setMoveLoading(true);
    try {
      const resp = await api.submitMove(gameState.game.id, gameState.apiKey, gameState.sessionId, payload);
      gameState.applyMoveResponse(resp);
    } catch (err: any) {
      gameState.showNotif(err?.message || 'Не удалось отправить слово');
      gameState.undoNewLetter();
    } finally {
      gameState.setMoveLoading(false);
    }
  }

  // Timer tick: only count down once both players are in the game
  $effect(() => {
    const interval = setInterval(() => {
      if (gameState.phase === 'playing' && gameState.opponent) {
        gameState.tickTimer();
      }
    }, 1000);
    return () => clearInterval(interval);
  });
</script>

<div class="mx-auto flex w-full max-w-lg flex-col gap-4 p-4">
  <!-- Waiting overlay: shown until the opponent joins -->
  {#if !gameState.opponent}
    <div class="rounded-2xl bg-blue-50 p-5 text-center">
      <div class="mb-1 h-6 w-6 animate-spin rounded-full border-4 border-blue-200 border-t-blue-600 mx-auto"></div>
      <div class="mt-2 font-semibold text-blue-800">Ожидание соперника…</div>
      <div class="mt-1 text-sm text-blue-600">
        ID игры: <span class="font-mono">{gameState.game?.id}</span>
      </div>
    </div>
  {/if}

  <!-- Header -->
  <div class="flex items-center justify-between">
    <div class="text-2xl font-extrabold tracking-tight text-stone-700">БАЛДА</div>
    {#if gameState.opponent}
      <Timer seconds={gameState.turnSecondsLeft} maxSeconds={60} isRunning={gameState.phase === 'playing'} />
    {/if}
  </div>

  <!-- Players -->
  <div class="flex gap-3">
    {#each gameState.players as p}
      <PlayerCard
        name={p.nickname}
        score={p.score}
        exp={p.exp}
        expGained={p.expGained}
        wordsCount={p.wordsCount}
        words={p.words}
        consecutiveSkips={p.consecutiveSkips}
        isActive={gameState.currentTurnUid === p.uid}
        isWinner={gameState.phase === 'finished' && gameState.winnerUid === p.uid}
      />
    {/each}
  </div>

  <!-- Board -->
  <Board
    board={gameState.board}
    selectedPath={gameState.selectedPath}
    newLetterCell={gameState.newLetterCell}
    isMyTurn={gameState.isMyTurn}
    onCellClick={handleCellClick}
  />

  <!-- Alphabet panel or cancel-letter button -->
  {#if gameState.isMyTurn && gameState.newLetterCell}
    {#if showAlphabet}
      <Alphabet onSelect={handleAlphabetSelect} onCancel={handleAlphabetCancel} />
    {:else if gameState.board[gameState.newLetterCell.row][gameState.newLetterCell.col]}
      <button
        type="button"
        onclick={handleAlphabetCancel}
        class="w-full rounded-xl bg-blue-50 px-4 py-2 text-sm font-semibold text-blue-700 ring-1 ring-blue-300 transition hover:bg-blue-100"
      >
        ✕ Удалить букву
      </button>
    {/if}
  {/if}

  <!-- Word bar -->
  <WordBar word={gameState.currentWord} />

  <!-- In-game notification -->
  {#if gameState.notif}
    <div
      class="flex items-center justify-between gap-2 rounded-xl px-4 py-2 text-sm font-medium
        {gameState.notif.kind === 'warn'
          ? 'bg-amber-50 text-amber-800 ring-1 ring-amber-300'
          : 'bg-red-50 text-red-800 ring-1 ring-red-300'}"
    >
      <span>{gameState.notif.message}</span>
      <button
        type="button"
        onclick={() => gameState.clearNotif()}
        class="shrink-0 text-current opacity-50 hover:opacity-100"
        aria-label="Закрыть"
      >✕</button>
    </div>
  {/if}

  <!-- Actions -->
  <div class="grid grid-cols-2 gap-3">
    <button
      onclick={handleSkip}
      disabled={!gameState.isMyTurn || gameState.moveLoading}
      class="flex items-center justify-center gap-2 rounded-xl bg-stone-200 px-4 py-3 font-bold text-stone-700 transition hover:bg-stone-300 disabled:opacity-50"
    >
      <Icon name="skip" size={18} />
      Пропустить
    </button>
    <button
      onclick={handleSubmit}
      disabled={!gameState.isMyTurn || gameState.moveLoading || gameState.currentWord.length < 3}
      class="flex items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-3 font-bold text-white transition hover:bg-blue-700 disabled:opacity-50"
    >
      <Icon name="send" size={18} />
      Отправить
    </button>
  </div>

  {#if gameState.phase === 'finished'}
    <div class="rounded-2xl bg-yellow-50 p-4 text-center">
      <div class="text-lg font-bold text-yellow-800">
        {#if gameState.winnerUid}
          {#if gameState.winnerUid === gameState.playerUid}
            🎉 Вы победили!
          {:else}
            Победил соперник
          {/if}
        {:else}
          Ничья!
        {/if}
      </div>
      <button
        onclick={() => gameState.setLobby()}
        class="mt-3 rounded-xl bg-yellow-500 px-6 py-2 font-bold text-white transition hover:bg-yellow-600"
      >
        В лобби
      </button>
    </div>
  {/if}
</div>
