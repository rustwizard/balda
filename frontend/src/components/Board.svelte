<script lang="ts">
  interface Props {
    board: string[][];
    selectedPath: { row: number; col: number }[];
    newLetterCell?: { row: number; col: number } | null;
    isMyTurn: boolean;
    onCellClick: (row: number, col: number) => void;
  }

  let { board, selectedPath, newLetterCell, isMyTurn, onCellClick }: Props = $props();

  function isSelected(row: number, col: number) {
    return selectedPath.some(p => p.row === row && p.col === col);
  }

  function isNewLetter(row: number, col: number) {
    return newLetterCell?.row === row && newLetterCell?.col === col;
  }

  function getCellClasses(row: number, col: number, hasLetter: boolean) {
    const base = 'relative flex items-center justify-center rounded-xl text-[clamp(20px,5vw,32px)] font-bold uppercase select-none transition-all active:scale-95';
    
    if (hasLetter) {
      if (isSelected(row, col)) {
        return `${base} bg-gradient-to-b from-amber-200 to-amber-300 text-stone-800 shadow-[0_2px_0_#d4d0c8,0_3px_6px_rgba(0,0,0,0.06)] ring-2 ring-blue-500`;
      }
      if (isNewLetter(row, col)) {
        return `${base} bg-blue-50 text-blue-700 ring-2 ring-blue-500 ring-dashed shadow-[0_2px_0_#93c5fd]`;
      }
      return `${base} bg-gradient-to-b from-[#f3e9d8] to-[#e6d5b8] text-stone-800 shadow-[0_4px_0_#c9b896,0_6px_10px_rgba(0,0,0,0.12)]`;
    }
    
    // Empty cell
    if (isNewLetter(row, col)) {
      return `${base} bg-blue-50 ring-2 ring-blue-500 ring-dashed`;
    }
    return `${base} bg-[#faf8f3] shadow-[0_2px_0_#d4d0c8,0_3px_6px_rgba(0,0,0,0.06)]`;
  }
</script>

<div class="grid aspect-square grid-cols-5 gap-2 rounded-2xl bg-[#e0ddd5] p-3 shadow-inner">
  {#each board as row, r}
    {#each row as cell, c}
      <button
        type="button"
        class={getCellClasses(r, c, !!cell)}
        onclick={() => onCellClick(r, c)}
        disabled={!isMyTurn && !cell}
        aria-label="Клетка {r + 1}-{c + 1}"
      >
        {#if isNewLetter(r, c) && !cell}
          <span class="text-blue-600 text-2xl">?</span>
        {:else}
          {cell}
        {/if}
      </button>
    {/each}
  {/each}
</div>
