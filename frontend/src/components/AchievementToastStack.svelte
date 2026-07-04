<script lang="ts">
  import { achievements } from '../stores/achievements.svelte';
  import Icon from './Icon.svelte';
</script>

<div
  class="pointer-events-none fixed right-4 top-4 z-50 flex w-full max-w-xs flex-col gap-2 max-[360px]:right-2 max-[360px]:max-w-[calc(100%-1rem)]"
>
  {#each achievements.toasts as toast (toast.id)}
    <div
      class="pointer-events-auto flex items-start gap-3 rounded-2xl bg-white p-4 shadow-lg ring-1 ring-stone-200"
      role="status"
    >
      <div
        class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-yellow-100 text-yellow-600"
      >
        <Icon name="trophy" size={20} />
      </div>
      <div class="flex-1">
        <div class="text-sm font-bold text-stone-800">Достижение разблокировано!</div>
        <div class="text-sm text-stone-600">{toast.achievement.name}</div>
      </div>
      <button
        type="button"
        class="text-stone-400 hover:text-stone-600"
        onclick={() => achievements.removeToast(toast.id)}
        aria-label="Закрыть"
      >
        ✕
      </button>
    </div>
  {/each}
</div>
