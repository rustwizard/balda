<script lang="ts">
  import { createGame, createGameWithBot, joinGame, listGames } from '../lib/api';
  import { centrifugo } from '../lib/centrifugo';
  import { gameState } from '../stores/game.svelte';
  import { achievements } from '../stores/achievements.svelte';
  import AchievementsModal from './AchievementsModal.svelte';
  import StatsModal from './StatsModal.svelte';

  let subscribed = $state(false);
  let error = $state('');
  let loading = $state(false);
  let showAchievements = $state(false);
  let showStats = $state(false);

  async function create() {
    loading = true;
    error = '';
    try {
      const res = await createGame();
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
      const res = await joinGame(id);
      if (res.game_token) {
        centrifugo.subscribe(`game:${res.game.id}`, res.game_token);
      }
      gameState.startGame(res.game);
      if (res.board && res.current_turn_uid) {
        const players = res.game.players?.length
          ? res.game.players.map((p) => ({ uid: p.uid, rating: p.rating, score: 0, words_count: 0, words: [] }))
          : res.game.player_ids.map((uid) => ({ uid, rating: 0, score: 0, words_count: 0, words: [] }));
        gameState.applyGameState({
          type: 'game_state',
          game_id: res.game.id,
          board: res.board,
          current_turn_uid: res.current_turn_uid,
          players,
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

  async function createWithBot() {
    loading = true;
    error = '';
    try {
      const res = await createGameWithBot();
      if (res.game_token) {
        centrifugo.subscribe(`game:${res.game.id}`, res.game_token);
      }
      gameState.startGame(res.game);
      if (res.board && res.current_turn_uid) {
        const players = res.game.players?.length
          ? res.game.players.map((p) => ({ uid: p.uid, rating: p.rating, score: 0, words_count: 0, words: [] }))
          : res.game.player_ids.map((uid) => ({ uid, rating: 0, score: 0, words_count: 0, words: [] }));
        gameState.applyGameState({
          type: 'game_state',
          game_id: res.game.id,
          board: res.board,
          current_turn_uid: res.current_turn_uid,
          players,
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
    listGames()
      .then((res) => gameState.setLobbyGames(res.games))
      .catch((err) => { error = err.message; });

    if (!subscribed && gameState.lobbyToken) {
      centrifugo.subscribe('lobby', gameState.lobbyToken);
      subscribed = true;
    }
  });
</script>

<div class="mx-auto w-full max-w-md rounded-2xl bg-white p-6 shadow-lg">
  <div class="mb-4 flex items-center justify-between">
    <h2 class="text-center text-2xl font-bold text-stone-800">Лобби</h2>
    <div class="flex items-center gap-2">
      <button
        type="button"
        onclick={() => (showAchievements = true)}
        class="rounded-lg bg-yellow-100 px-2 py-1 text-sm font-semibold text-yellow-700 hover:bg-yellow-200"
      >
        🏆 {achievements.unlockedCount}/{achievements.totalCount}
      </button>
      <button
        type="button"
        onclick={() => (showStats = true)}
        class="rounded-lg bg-blue-100 px-2 py-1 text-sm font-semibold text-blue-700 hover:bg-blue-200"
        aria-label="Статистика"
      >
        📊
      </button>
      <div class="text-sm font-medium text-stone-600">
        {gameState.nickname}
        <span class="ml-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700">{gameState.exp} XP</span>
        <span class="ml-1 rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-700">{gameState.rating} ELO</span>
      </div>
    </div>
  </div>

  <button
    onclick={create}
    disabled={loading}
    class="mb-4 w-full rounded-xl bg-blue-600 px-4 py-3 font-bold text-white transition hover:bg-blue-700 disabled:opacity-50"
  >
    Создать игру
  </button>

  <button
    onclick={createWithBot}
    disabled={loading}
    class="mb-4 w-full rounded-xl bg-purple-600 px-4 py-3 font-bold text-white transition hover:bg-purple-700 disabled:opacity-50"
  >
    🤖 Играть с ботом
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

{#if showAchievements}
  <AchievementsModal onClose={() => (showAchievements = false)} />
{/if}

{#if showStats}
  <StatsModal onClose={() => (showStats = false)} />
{/if}
