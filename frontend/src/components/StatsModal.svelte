<script lang="ts">
  import { getPlayerStats } from '../lib/api';
  import type { PlayerStats } from '../types';

  interface Props {
    onClose: () => void;
  }

  let { onClose }: Props = $props();

  let stats = $state<PlayerStats | null>(null);
  let loading = $state(true);
  let error = $state('');

  $effect(() => {
    getPlayerStats()
      .then((s) => (stats = s))
      .catch((e) => (error = e instanceof Error ? e.message : 'Не удалось загрузить статистику'))
      .finally(() => (loading = false));
  });

  function plural(n: number, one: string, few: string, many: string): string {
    const mod100 = Math.abs(n) % 100;
    const mod10 = mod100 % 10;
    if (mod100 > 10 && mod100 < 20) return many;
    if (mod10 > 1 && mod10 < 5) return few;
    if (mod10 === 1) return one;
    return many;
  }
</script>

<div
  class="fixed inset-0 z-40 flex items-center justify-center bg-black/40 p-4"
  role="dialog"
  aria-modal="true"
  aria-label="Статистика"
  tabindex="-1"
  onclick={(e) => e.target === e.currentTarget && onClose()}
  onkeydown={(e) => e.key === 'Escape' && onClose()}
>
  <div class="mx-auto w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-2xl font-bold text-stone-800">Статистика</h2>
      <button
        type="button"
        class="text-stone-400 hover:text-stone-600"
        onclick={onClose}
        aria-label="Закрыть"
      >
        ✕
      </button>
    </div>

    {#if loading}
      <div class="py-8 text-center text-stone-500">Загрузка…</div>
    {:else if error}
      <div class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
        {error}
      </div>
    {:else if stats}
      {#if stats.games_played === 0}
        <div class="py-8 text-center text-stone-500">Пока нет сыгранных игр</div>
      {:else}
        <div class="grid grid-cols-3 gap-2 text-center">
          <div class="rounded-xl bg-stone-50 p-3">
            <div class="text-xl font-extrabold text-stone-800">{stats.games_played}</div>
            <div class="text-xs text-stone-500">{plural(stats.games_played, 'игра', 'игры', 'игр')}</div>
          </div>
          <div class="rounded-xl bg-green-50 p-3">
            <div class="text-xl font-extrabold text-green-700">{stats.wins}</div>
            <div class="text-xs text-stone-500">{plural(stats.wins, 'победа', 'победы', 'побед')}</div>
          </div>
          <div class="rounded-xl bg-red-50 p-3">
            <div class="text-xl font-extrabold text-red-600">{stats.losses}</div>
            <div class="text-xs text-stone-500">{plural(stats.losses, 'поражение', 'поражения', 'поражений')}</div>
          </div>
        </div>

        <div class="mt-2 grid grid-cols-3 gap-2 text-center">
          <div class="rounded-xl bg-stone-50 p-3">
            <div class="text-xl font-extrabold text-stone-800">{stats.draws}</div>
            <div class="text-xs text-stone-500">{plural(stats.draws, 'ничья', 'ничьи', 'ничьих')}</div>
          </div>
          <div class="rounded-xl bg-blue-50 p-3">
            <div class="text-xl font-extrabold text-blue-700">{Math.round(stats.win_rate * 100)}%</div>
            <div class="text-xs text-stone-500">винрейт</div>
          </div>
          <div class="rounded-xl bg-stone-50 p-3">
            <div class="text-xl font-extrabold text-stone-800">{stats.avg_word_length.toFixed(1)}</div>
            <div class="text-xs text-stone-500">ср. длина слова</div>
          </div>
        </div>

        <div class="mt-2 grid grid-cols-2 gap-2 text-center">
          <div class="rounded-xl bg-yellow-50 p-3">
            <div class="truncate text-xl font-extrabold text-yellow-700">{stats.best_word || '—'}</div>
            <div class="text-xs text-stone-500">лучшее слово</div>
          </div>
          <div class="rounded-xl bg-amber-50 p-3">
            <div class="text-xl font-extrabold uppercase text-amber-700">{stats.favorite_letter || '—'}</div>
            <div class="text-xs text-stone-500">любимая буква</div>
          </div>
        </div>
      {/if}
    {/if}
  </div>
</div>
