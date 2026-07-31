<script lang="ts">
  import { gameState } from '../stores/game.svelte';
  import { leaveGame } from '../lib/api';

  let error = $state('');

  async function cancel() {
    error = '';
    const gameId = gameState.game?.id;
    try {
      if (gameId) await leaveGame(gameId);
    } catch (e: any) {
      error = e?.message || 'Не удалось отменить игру';
      return;
    }
    gameState.setLobby();
  }
</script>

<div class="mx-auto flex w-full max-w-md flex-col items-center rounded-2xl bg-white p-8 shadow-lg">
  <div class="mb-4 h-12 w-12 animate-spin rounded-full border-4 border-stone-200 border-t-blue-600"></div>
  <h2 class="text-xl font-bold text-stone-800">Ожидание соперника</h2>
  <p class="mt-2 text-center text-stone-500">
    Игра создана. Поделитесь ID игры с другом:<br />
    <span class="mt-1 inline-block rounded-lg bg-stone-100 px-3 py-1 font-mono text-sm text-stone-700">{gameState.game?.id}</span>
  </p>
  {#if error}
    <div class="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
  {/if}
  <button
    onclick={cancel}
    class="mt-6 rounded-xl bg-stone-200 px-6 py-2 font-bold text-stone-700 transition hover:bg-stone-300"
  >
    Отменить
  </button>
</div>
