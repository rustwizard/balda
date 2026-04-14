<script lang="ts">
  import Icon from './Icon.svelte';

  interface Props {
    seconds: number;
    maxSeconds?: number;
    isRunning?: boolean;
  }

  let { seconds, maxSeconds = 60, isRunning = true }: Props = $props();

  const mins = $derived(Math.floor(seconds / 60));
  const secs = $derived(seconds % 60);
  const isLow = $derived(seconds <= 10 && isRunning);
  const pct = $derived(maxSeconds > 0 ? (seconds / maxSeconds) * 100 : 0);
</script>

<div class="flex items-center gap-2 rounded-full px-4 py-2 font-bold tabular-nums transition-colors {isLow ? 'bg-orange-100 text-orange-700' : 'bg-blue-50 text-blue-600'}">
  <Icon name="timer" size={18} />
  <span>{mins}:{secs.toString().padStart(2, '0')}</span>
  <div class="ml-1 h-2 w-16 overflow-hidden rounded-full bg-current/20">
    <div class="h-full rounded-full bg-current transition-all" style="width: {pct}%"></div>
  </div>
</div>
