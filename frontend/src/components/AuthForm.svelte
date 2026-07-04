<script lang="ts">
  import { auth, signup, setTokens } from '../lib/api';
  import { centrifugo } from '../lib/centrifugo';
  import { gameState } from '../stores/game.svelte';
  import type { AuthResponse, SignupResponse } from '../types';

  let isSignup = $state(false);
  let email = $state('');
  let password = $state('');
  let firstname = $state('');
  let lastname = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    error = '';
    loading = true;

    try {
      let res: AuthResponse | SignupResponse;
      if (isSignup) {
        res = await signup({ firstname, lastname, email, password });
      } else {
        res = await auth({ email, password });
      }

      const player = 'player' in res ? res.player : res.user;
      setTokens(res.access_token || '', res.refresh_token || '');
      gameState.setAuth({
        playerUid: player.uid,
        nickname: player.firstname,
        exp: player.exp ?? 0,
        rating: player.rating ?? 0,
        centrifugoToken: res.centrifugo_token || '',
        lobbyToken: res.lobby_token || '',
      });

      // Restore an active game after reconnect (e.g. page refresh). The backend
      // returns the game token and snapshot so the player can rejoin without
      // being stuck in the lobby with a "player already in a game" conflict.
      const activeGame = 'active_game' in res ? res.active_game : undefined;
      if (activeGame?.game_id && activeGame.game_token) {
        centrifugo.subscribe(`game:${activeGame.game_id}`, activeGame.game_token);
        const playerIds = activeGame.players?.map((p) => p.uid) ?? [];
        if (activeGame.status === 'waiting') {
          gameState.setWaiting({
            id: activeGame.game_id,
            player_ids: playerIds,
            status: 'waiting',
            started_at: 0,
          });
        } else {
          gameState.startGame({
            id: activeGame.game_id,
            player_ids: playerIds,
            status: activeGame.status || 'in_progress',
            started_at: 0,
          });
          if (activeGame.board && activeGame.current_turn_uid) {
            gameState.applyGameState({
              type: 'game_state',
              game_id: activeGame.game_id,
              board: activeGame.board,
              current_turn_uid: activeGame.current_turn_uid,
              players: activeGame.players ?? [],
              status: activeGame.status || 'in_progress',
              move_number: activeGame.move_number ?? 0,
            });
          }
        }
      } else {
        gameState.setLobby();
      }
    } catch (err: any) {
      error = err.message || 'Ошибка авторизации';
    } finally {
      loading = false;
    }
  }
</script>

<div class="mx-auto w-full max-w-sm rounded-2xl bg-white p-6 shadow-lg">
  <h2 class="mb-4 text-center text-2xl font-bold text-stone-800">
    {isSignup ? 'Регистрация' : 'Вход в игру'}
  </h2>

  <form onsubmit={handleSubmit} class="flex flex-col gap-3">
    {#if isSignup}
      <input
        type="text"
        placeholder="Имя"
        bind:value={firstname}
        required
        class="rounded-xl border border-stone-200 px-4 py-3 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-200"
      />
      <input
        type="text"
        placeholder="Фамилия"
        bind:value={lastname}
        required
        class="rounded-xl border border-stone-200 px-4 py-3 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-200"
      />
    {/if}
    <input
      type="email"
      placeholder="Email"
      bind:value={email}
      required
      class="rounded-xl border border-stone-200 px-4 py-3 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-200"
    />
    <input
      type="password"
      placeholder="Пароль"
      bind:value={password}
      required
      class="rounded-xl border border-stone-200 px-4 py-3 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-200"
    />

    {#if error}
      <div class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</div>
    {/if}

    <button
      type="submit"
      disabled={loading}
      class="mt-2 rounded-xl bg-blue-600 px-4 py-3 font-bold text-white transition hover:bg-blue-700 disabled:opacity-50"
    >
      {loading ? 'Загрузка...' : isSignup ? 'Зарегистрироваться' : 'Войти'}
    </button>
  </form>

  <button
    type="button"
    onclick={() => (isSignup = !isSignup)}
    class="mt-4 w-full text-center text-sm text-stone-500 hover:text-stone-700"
  >
    {isSignup ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Зарегистрироваться'}
  </button>
</div>
