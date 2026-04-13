<script lang="ts">
  import AuthForm from './components/AuthForm.svelte';
  import Lobby from './components/Lobby.svelte';
  import WaitingScreen from './components/WaitingScreen.svelte';
  import GameScreen from './components/GameScreen.svelte';
  import { gameState } from './stores/game.svelte';
  import { centrifugo } from './lib/centrifugo';
  import { ping } from './lib/api';
  import type { CentrifugoEvent } from './types';

  // API key for demo - normally from env or config
  const DEMO_API_KEY = import.meta.env.VITE_API_KEY || 'abcdefuvwxyz';
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const CENTRIFUGO_WS_URL = import.meta.env.VITE_CENTRIFUGO_WS_URL || `${protocol}//${window.location.host}/connection/websocket`;

  let pingCounter = 0;

  // Keep session alive with periodic pings every 5 seconds
  $effect(() => {
    if (gameState.phase === 'auth') return;
    const interval = setInterval(() => {
      ping(gameState.apiKey, gameState.sessionId, ++pingCounter).catch(() => {});
    }, 5000);
    return () => clearInterval(interval);
  });

  let connected = $state(false);

  // Connect to Centrifugo once after auth
  $effect(() => {
    if (!connected && gameState.centrifugoToken) {
      connected = true;
      centrifugo.connect(CENTRIFUGO_WS_URL, gameState.centrifugoToken);
    }
  });

  // Handle Centrifugo events
  centrifugo.onEvent((ev: CentrifugoEvent) => {
    console.log('[app] centrifugo event', ev.type, ev);
    switch (ev.type) {
      case 'game_state':
        gameState.applyGameState(ev);
        break;
      case 'game_over':
        gameState.finishGame(ev);
        break;
      case 'game_started':
        // Only the creator (waiting phase) needs this event to transition.
        // The joiner already called startGame() synchronously in join().
        if (gameState.phase === 'waiting' && gameState.game?.id === ev.game_id) {
          gameState.startGame({
            id: ev.game_id,
            player_ids: ev.player_ids,
            status: ev.status,
            started_at: ev.started_at,
          });
        }
        break;
      case 'game_created':
        // Lobby will refresh via its own polling or we could trigger refresh here
        break;
    }
  });
</script>

<main class="flex min-h-screen flex-col items-center justify-center p-4">
  {#if gameState.phase === 'auth'}
    <AuthForm apiKey={DEMO_API_KEY} />
  {:else if gameState.phase === 'lobby'}
    <Lobby />
  {:else if gameState.phase === 'waiting'}
    <WaitingScreen />
  {:else}
    <GameScreen />
  {/if}
</main>
