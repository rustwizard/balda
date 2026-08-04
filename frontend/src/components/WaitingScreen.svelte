<script lang="ts">
  import { gameState } from '../stores/game.svelte';
  import { leaveGame } from '../lib/api';
  import { isMiniApp, shareTelegramLink } from '../lib/telegram';

  let error = $state('');
  let copied = $state(false);

  let inviteUrl = $derived(
    gameState.telegramAppUrl && gameState.game?.id
      ? `${gameState.telegramAppUrl}?startapp=game_${gameState.game.id}`
      : ''
  );

  async function invite() {
    if (!inviteUrl) return;
    if (isMiniApp()) {
      shareTelegramLink(inviteUrl);
      return;
    }
    try {
      await navigator.clipboard.writeText(inviteUrl);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      error = 'Не удалось скопировать ссылку';
    }
  }

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
  {#if inviteUrl}
    <button
      onclick={invite}
      class="mt-6 w-full rounded-xl bg-blue-600 px-6 py-2 font-bold text-white transition hover:bg-blue-700"
    >
      {copied ? 'Ссылка скопирована ✓' : 'Пригласить друга'}
    </button>
  {/if}
  <button
    onclick={cancel}
    class="mt-3 rounded-xl bg-stone-200 px-6 py-2 font-bold text-stone-700 transition hover:bg-stone-300"
  >
    Отменить
  </button>
</div>
