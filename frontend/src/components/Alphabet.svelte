<script lang="ts">
  interface Props {
    onSelect: (char: string) => void;
    onCancel: () => void;
  }

  let { onSelect, onCancel }: Props = $props();

  const letters = [
    'А','Б','В','Г','Д','Е','Ж','З','И','Й',
    'К','Л','М','Н','О','П','Р','С','Т','У',
    'Ф','Х','Ц','Ч','Ш','Щ','Ъ','Ы','Ь','Э',
    'Ю','Я'
  ];
</script>

<div class="rounded-xl bg-white p-3 shadow">
  <div class="grid grid-cols-8 gap-2">
    {#each letters as char}
      <button
        type="button"
        onclick={() => onSelect(char.toLowerCase())}
        class="aspect-square rounded-lg bg-blue-50 text-lg font-bold text-blue-700 transition hover:bg-blue-100 active:scale-95"
      >
        {char}
      </button>
    {/each}
  </div>
  <button
    type="button"
    onclick={onCancel}
    class="mt-2 w-full rounded-lg bg-stone-100 py-2 text-sm font-semibold text-stone-600 transition hover:bg-stone-200"
  >
    Отмена
  </button>
</div>
