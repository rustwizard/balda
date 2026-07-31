<script lang="ts">
  import { createGame, joinGame, listGames, joinMatchmaking, leaveMatchmaking } from '../lib/api';
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
  let searchSeconds = $state(0);
  let searchTimer: ReturnType<typeof setInterval> | null = null;

  async function quickMatch() {
    error = '';
    // Set searching before the request: when nobody is queued the server
    // starts a bot game immediately and match_found may arrive before the
    // HTTP response, clearing the flag again.
    gameState.setSearching(true);
    searchSeconds = 0;
    searchTimer = setInterval(() => (searchSeconds += 1), 1000);
    try {
      await joinMatchmaking();
    } catch (err: any) {
      stopSearch();
      error = err.message;
    }
  }

  async function cancelSearch() {
    try {
      await leaveMatchmaking();
    } catch {
      // Leaving is idempotent; ignore network hiccups.
    }
    stopSearch();
  }

  function stopSearch() {
    gameState.setSearching(false);
    if (searchTimer) {
      clearInterval(searchTimer);
      searchTimer = null;
    }
  }

  async function create() {
    loading = true;
    error = '';
    try {
      const res = await createGame();
      if (res.game_token) {
        centrifugo.subscribe(`game:${res.game.id}`, res.game_token);
      }
      // A freshly created game is waiting for an opponent — show the
      // waiting screen (with a working cancel), not the game board.
      if (res.game.status === 'waiting') {
        gameState.setWaiting(res.game);
      } else {
        gameState.startGame(res.game);
      }
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

  // The search state is global (match_found can arrive any time); when it
  // flips off, stop the local seconds counter too.
  $effect(() => {
    if (!gameState.searching) stopSearch();
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

  {#if gameState.searching}
    <div class="mb-4 rounded-xl bg-blue-50 p-4 text-center">
      <div class="mb-1 text-lg font-bold text-blue-800">Ищем соперника…</div>
      <div class="mb-3 text-sm text-blue-600">{searchSeconds} сек</div>
      <button
        onclick={cancelSearch}
        class="rounded-lg bg-white px-4 py-2 text-sm font-semibold text-stone-600 ring-1 ring-stone-200 hover:bg-stone-50"
      >
        Отмена
      </button>
    </div>
  {:else}
    <button
      onclick={quickMatch}
      class="mb-2 w-full rounded-xl bg-blue-600 px-4 py-3 text-lg font-bold text-white transition hover:bg-blue-700"
    >
      ▶ Играть
    </button>
    <button
      onclick={create}
      disabled={loading}
      class="mb-4 w-full rounded-xl bg-stone-100 px-4 py-2 text-sm font-semibold text-stone-600 transition hover:bg-stone-200 disabled:opacity-50"
    >
      Создать приватную игру
    </button>
  {/if}

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
