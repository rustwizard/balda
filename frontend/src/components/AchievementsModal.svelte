<script lang="ts">
  import { achievements } from '../stores/achievements.svelte';
  import Icon from './Icon.svelte';

  interface Props {
    onClose: () => void;
  }

  let { onClose }: Props = $props();

  $effect(() => {
    achievements.load();
  });
</script>

<div
  class="fixed inset-0 z-40 flex items-center justify-center bg-black/40 p-4"
  role="dialog"
  aria-modal="true"
  aria-label="Достижения"
  tabindex="-1"
  onclick={(e) => e.target === e.currentTarget && onClose()}
  onkeydown={(e) => e.key === 'Escape' && onClose()}
>
  <div
    class="mx-auto w-full max-w-md rounded-2xl bg-white p-6 shadow-xl"
  >
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-2xl font-bold text-stone-800">Достижения</h2>
      <button
        type="button"
        class="text-stone-400 hover:text-stone-600"
        onclick={onClose}
        aria-label="Закрыть"
      >
        ✕
      </button>
    </div>

    {#if achievements.loading}
      <div class="py-8 text-center text-stone-500">Загрузка…</div>
    {:else if achievements.error}
      <div class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
        {achievements.error}
      </div>
    {:else}
      <div class="flex max-h-[60vh] flex-col gap-2 overflow-y-auto">
        {#each achievements.list as achievement (achievement.id)}
          <div
            class="flex items-start gap-3 rounded-xl p-3 {achievement.unlocked
              ? 'bg-yellow-50 ring-1 ring-yellow-200'
              : 'bg-stone-50'}"
          >
            <div
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full {achievement.unlocked
                ? 'bg-yellow-100 text-yellow-600'
                : 'bg-stone-200 text-stone-400'}"
            >
              <Icon name="trophy" size={20} />
            </div>
            <div class="flex-1">
              <div class="font-semibold text-stone-800">{achievement.name}</div>
              <div class="text-sm text-stone-500">{achievement.description}</div>
            </div>
            {#if achievement.unlocked}
              <span
                class="rounded-full bg-yellow-200 px-2 py-0.5 text-xs font-bold text-yellow-800"
              >
                ✓
              </span>
            {/if}
          </div>
        {/each}
      </div>

      <div class="mt-4 text-center text-sm text-stone-500">
        Разблокировано: {achievements.unlockedCount} / {achievements.totalCount}
      </div>
    {/if}
  </div>
</div>
