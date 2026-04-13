<script lang="ts">
  import Icon from './Icon.svelte';

  interface Props {
    name: string;
    score: number;
    wordsCount: number;
    isActive?: boolean;
    isWinner?: boolean;
  }

  let { name, score, wordsCount, isActive = false, isWinner = false }: Props = $props();
</script>

<div class="flex-1 rounded-2xl p-4 text-center transition-all {isActive ? 'bg-blue-50 ring-2 ring-blue-500' : 'bg-stone-100'}">
  <div class="mb-1 flex items-center justify-center gap-1 text-sm font-semibold text-stone-700">
    {#if isWinner}
      <Icon name="crown" size={14} class="text-yellow-500" />
    {/if}
    {name}
  </div>
  <div class="text-2xl font-extrabold text-blue-600">{score}</div>
  <div class="mt-1 text-xs text-stone-500">
    {wordsCount} {wordsCount === 1 ? 'слово' : wordsCount < 5 ? 'слова' : 'слов'}
  </div>
</div>
