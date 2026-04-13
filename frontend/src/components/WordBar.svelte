<script lang="ts">
  interface Props {
    word: string;
  }

  let { word }: Props = $props();
</script>

<div class="rounded-2xl bg-stone-100 p-4 text-center">
  <div class="text-xs font-medium uppercase tracking-wide text-stone-500">Ваше слово</div>
  <div class="mt-1 min-h-[2rem] text-2xl font-bold tracking-widest text-stone-800">
    {word || '\u00A0'}
  </div>
</div>
