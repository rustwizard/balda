<script lang="ts">
  import { auth, signup } from '../lib/api';
  import { gameState } from '../stores/game.svelte';
  import type { AuthResponse, SignupResponse } from '../types';

  interface Props {
    apiKey: string;
  }

  let { apiKey }: Props = $props();

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
        res = await auth({ email, password }, apiKey);
      }

      const player = 'player' in res ? res.player : res.user;
      gameState.setAuth({
        apiKey,
        sessionId: player.sid,
        playerUid: player.uid,
        nickname: player.firstname,
        centrifugoToken: res.centrifugo_token || '',
        lobbyToken: res.lobby_token || '',
      });
      gameState.setLobby();
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
