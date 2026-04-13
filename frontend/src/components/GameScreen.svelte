<script lang="ts">
  import Board from './Board.svelte';
  import PlayerCard from './PlayerCard.svelte';
  import Timer from './Timer.svelte';
  import WordBar from './WordBar.svelte';
  import Icon from './Icon.svelte';
  import { gameState } from '../stores/game.svelte';

  let newLetter = $state('');

  function handleCellClick(row: number, col: number) {
    if (!gameState.isMyTurn) return;

    const cell = gameState.board[row][col];
    if (!cell) {
      // Empty cell - set as new letter placement
      gameState.setNewLetterCell(row, col);
      return;
    }

    gameState.selectCell(row, col);
  }

  function handlePlaceLetter() {
    const char = newLetter.trim().toLowerCase();
    if (char.length === 1 && /^[а-яё]$/i.test(char)) {
      gameState.setLetterAtCell(char);
      newLetter = '';
    }
  }

  function handleSkip() {
    // TODO: call API to skip turn
    alert('Пропуск хода (заглушка)');
  }

  function handleSubmit() {
    // TODO: call API to submit word
    alert(`Отправлено слово: ${gameState.currentWord} (заглушка)`);
    gameState.clearSelection();
  }

  // Timer tick
  $effect(() => {
    const interval = setInterval(() => {
      if (gameState.phase === 'playing') {
        gameState.tickTimer();
      }
    }, 1000);
    return () => clearInterval(interval);
  });
</script>

<div class="mx-auto flex w-full max-w-lg flex-col gap-4 p-4">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div class="text-2xl font-extrabold tracking-tight text-stone-700">БАЛДА</div>
    <Timer seconds={gameState.turnSecondsLeft} maxSeconds={60} isRunning={gameState.phase === 'playing'} />
  </div>

  <!-- Players -->
  <div class="flex gap-3">
    {#each gameState.players as p}
      <PlayerCard
        name={p.nickname}
        score={p.score}
        wordsCount={p.wordsCount}
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

  <!-- New letter input -->
  {#if gameState.isMyTurn && gameState.newLetterCell}
    <div class="flex items-center justify-center gap-2 rounded-xl bg-blue-50 p-3">
      <span class="text-sm text-stone-600">Новая буква:</span>
      <input
        type="text"
        maxlength="1"
        bind:value={newLetter}
        oninput={handlePlaceLetter}
        class="h-10 w-10 rounded-lg border-2 border-blue-300 text-center text-xl font-bold uppercase outline-none focus:border-blue-500"
        placeholder="?"
      />
      <span class="text-xs text-stone-500">
        ({gameState.newLetterCell.row + 1}, {gameState.newLetterCell.col + 1})
      </span>
    </div>
  {/if}

  <!-- Word bar -->
  <WordBar word={gameState.currentWord} />

  <!-- Actions -->
  <div class="grid grid-cols-2 gap-3">
    <button
      onclick={handleSkip}
      disabled={!gameState.isMyTurn}
      class="flex items-center justify-center gap-2 rounded-xl bg-stone-200 px-4 py-3 font-bold text-stone-700 transition hover:bg-stone-300 disabled:opacity-50"
    >
      <Icon name="skip" size={18} />
      Пропустить
    </button>
    <button
      onclick={handleSubmit}
      disabled={!gameState.isMyTurn || gameState.currentWord.length < 3}
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
