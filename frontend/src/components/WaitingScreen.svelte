<script lang="ts">
  import { gameState } from '../stores/game.svelte';
</script>

<div class="mx-auto flex w-full max-w-md flex-col items-center rounded-2xl bg-white p-8 shadow-lg">
  <div class="mb-4 h-12 w-12 animate-spin rounded-full border-4 border-stone-200 border-t-blue-600"></div>
  <h2 class="text-xl font-bold text-stone-800">Ожидание соперника</h2>
  <p class="mt-2 text-center text-stone-500">
    Игра создана. Поделитесь ID игры с другом:<br />
    <span class="mt-1 inline-block rounded-lg bg-stone-100 px-3 py-1 font-mono text-sm text-stone-700">{gameState.game?.id}</span>
  </p>
  <button
    onclick={() => gameState.setLobby()}
    class="mt-6 rounded-xl bg-stone-200 px-6 py-2 font-bold text-stone-700 transition hover:bg-stone-300"
  >
    Отменить
  </button>
</div>
