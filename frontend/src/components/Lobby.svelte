<script lang="ts">
  import { createGame, joinGame, listGames } from '../lib/api';
  import { centrifugo } from '../lib/centrifugo';
  import { gameState } from '../stores/game.svelte';

  let subscribed = $state(false);
  let error = $state('');
  let loading = $state(false);

  async function create() {
    loading = true;
    error = '';
    try {
      const res = await createGame(gameState.apiKey, gameState.sessionId);
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

  async function join(id: string) {
    loading = true;
    error = '';
    try {
      const res = await joinGame(id, gameState.apiKey, gameState.sessionId);
      if (res.game_token) {
        centrifugo.subscribe(`game:${res.game.id}`, res.game_token);
      }
      gameState.startGame(res.game);
      if (res.board && res.current_turn_uid) {
        gameState.applyGameState({
          type: 'game_state',
          game_id: res.game.id,
          board: res.board,
          current_turn_uid: res.current_turn_uid,
          players: res.game.player_ids.map((uid) => ({ uid, score: 0, words_count: 0, words: [] })),
          status: 'in_progress',
          move_number: 0,
        });
      }
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }

  // Load initial game list and subscribe to lobby channel once
  $effect(() => {
    listGames(gameState.apiKey, gameState.sessionId)
      .then((res) => gameState.setLobbyGames(res.games))
      .catch((err) => { error = err.message; });

    if (!subscribed && gameState.lobbyToken) {
      centrifugo.subscribe('lobby', gameState.lobbyToken);
      subscribed = true;
    }
  });
</script>

<div class="mx-auto w-full max-w-md rounded-2xl bg-white p-6 shadow-lg">
  <h2 class="mb-4 text-center text-2xl font-bold text-stone-800">Лобби</h2>

  <button
    onclick={create}
    disabled={loading}
    class="mb-4 w-full rounded-xl bg-blue-600 px-4 py-3 font-bold text-white transition hover:bg-blue-700 disabled:opacity-50"
  >
    Создать игру
  </button>

  {#if error}
    <div class="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
  {/if}

  <div class="flex flex-col gap-2">
    {#if gameState.lobbyGames.length === 0}
      <div class="rounded-xl bg-stone-50 p-4 text-center text-stone-500">Нет активных игр</div>
    {:else}
      {#each gameState.lobbyGames as g}
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
