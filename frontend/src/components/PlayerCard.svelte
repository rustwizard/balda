<script lang="ts">
  import Icon from './Icon.svelte';

  const MAX_SKIPS = 3;

  interface Props {
    name: string;
    score: number;
    exp?: number;
    wordsCount: number;
    words?: string[];
    consecutiveSkips?: number;
    isActive?: boolean;
    isWinner?: boolean;
  }

  let { name, score, exp = 0, wordsCount, words = [], consecutiveSkips = 0, isActive = false, isWinner = false }: Props = $props();

  let showWords = $state(false);

  function toggleWords() {
    if (words.length > 0) showWords = !showWords;
  }
</script>

<div class="flex-1 rounded-2xl p-4 text-center transition-all {isActive ? 'bg-blue-50 ring-2 ring-blue-500' : 'bg-stone-100'}">
  <div class="mb-1 flex items-center justify-center gap-1 text-sm font-semibold text-stone-700">
    {#if isWinner}
      <Icon name="crown" size={14} class="text-yellow-500" />
    {/if}
    {name}
  </div>
  <div class="text-2xl font-extrabold text-blue-600">{score}</div>
  <div class="text-xs text-stone-500">{exp} XP</div>
  <button
    class="mt-1 text-xs text-stone-500 {words.length > 0 ? 'cursor-pointer hover:text-blue-500' : 'cursor-default'}"
    onclick={toggleWords}
    type="button"
  >
    {wordsCount} {wordsCount === 1 ? 'слово' : wordsCount < 5 ? 'слова' : 'слов'}
    {#if words.length > 0}
      <span class="ml-0.5">{showWords ? '▲' : '▼'}</span>
    {/if}
  </button>
  {#if showWords && words.length > 0}
    <ul class="mt-2 max-h-32 overflow-y-auto text-left text-xs text-stone-600">
      {#each words as word}
        <li class="truncate px-1 py-0.5 odd:bg-stone-200/50">{word}</li>
      {/each}
    </ul>
  {/if}
  {#if consecutiveSkips > 0}
    <div class="mt-2 flex items-center justify-center gap-1" title="Пропуски подряд: {consecutiveSkips}/{MAX_SKIPS}">
      {#each Array(MAX_SKIPS) as _, i}
        <span class="h-2 w-2 rounded-full {i < consecutiveSkips ? 'bg-red-400' : 'bg-stone-300'}"></span>
      {/each}
    </div>
  {/if}
</div>
