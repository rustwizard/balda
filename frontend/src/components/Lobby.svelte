<script lang="ts">
  import { createGame, joinGame, listGames } from '../lib/api';
  import { centrifugo } from '../lib/centrifugo';
  import { gameState } from '../stores/game.svelte';
  import type { GameSummary } from '../types';

  let games = $state<GameSummary[]>([]);
  let subscribed = $state(false);
  let error = $state('');
  let loading = $state(false);

  async function refresh() {
    try {
      const res = await listGames(gameState.apiKey, gameState.sessionId);
      games = res.games;
    } catch (err: any) {
      error = err.message;
    }
  }

  async function create() {
    loading = true;
    error = '';
    try {
      const res = await createGame(gameState.apiKey, gameState.sessionId);
      if (res.game_token) {
        centrifugo.subscribe(`game:${res.game.id}`, res.game_token);
      }
      gameState.setWaiting(res.game);
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  async function join(id: string) {
    loading = true;
    error = '';
    try {
      const res = await joinGame(id, gameState.apiKey, gameState.sessionId);
      if (res.game_token) {
        centrifugo.subscribe(`game:${res.game.id}`, res.game_token);
      }
      gameState.startGame(res.game);
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  // Auto-refresh on mount and subscribe to lobby
  $effect(() => {
    refresh();
    if (!subscribed && gameState.lobbyToken) {
      centrifugo.subscribe('lobby', gameState.lobbyToken);
      subscribed = true;
    }
  });
</script>

<div class="mx-auto w-full max-w-md rounded-2xl bg-white p-6 shadow-lg">
  <h2 class="mb-4 text-center text-2xl font-bold text-stone-800">Лобби</h2>

  <div class="mb-4 flex gap-2">
    <button
      onclick={create}
      disabled={loading}
      class="flex-1 rounded-xl bg-blue-600 px-4 py-3 font-bold text-white transition hover:bg-blue-700 disabled:opacity-50"
    >
      Создать игру
    </button>
    <button
      onclick={refresh}
      disabled={loading}
      class="rounded-xl bg-stone-200 px-4 py-3 font-bold text-stone-700 transition hover:bg-stone-300 disabled:opacity-50"
    >
      Обновить
    </button>
  </div>

  {#if error}
    <div class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
  {/if}

  <div class="flex flex-col gap-2">
    {#if games.length === 0}
      <div class="rounded-xl bg-stone-50 p-4 text-center text-stone-500">Нет активных игр</div>
    {:else}
      {#each games as g}
        <div class="flex items-center justify-between rounded-xl bg-stone-50 p-3">
          <div class="text-sm">
            <div class="font-semibold text-stone-700">
              Игра {g.status === 'waiting' ? '(ожидание)' : '(в процессе)'}
            </div>
            <div class="text-xs text-stone-500">Игроков: {g.player_ids.length}</div>
          </div>
          {#if g.status === 'waiting' && !g.player_ids.includes(gameState.playerUid)}
            <button
              onclick={() => join(g.id)}
              disabled={loading}
              class="rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-bold text-white transition hover:bg-blue-700 disabled:opacity-50"
            >
              Войти
            </button>
          {:else}
            <span class="text-xs text-stone-400">{g.player_ids.includes(gameState.playerUid) ? 'Вы в игре' : ''}</span>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>
