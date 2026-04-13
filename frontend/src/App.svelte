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
  const CENTRIFUGO_WS_URL = import.meta.env.VITE_CENTRIFUGO_WS_URL || 'ws://localhost:8080/connection/websocket';

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
    if (!connected && gameState.phase !== 'auth' && gameState.centrifugoToken) {
      connected = true;
      centrifugo.connect(CENTRIFUGO_WS_URL, gameState.centrifugoToken);
      return () => centrifugo.disconnect();
    }
  });

  // Handle Centrifugo events
  centrifugo.onEvent((ev: CentrifugoEvent) => {
    switch (ev.type) {
      case 'game_state':
        gameState.applyGameState(ev);
        break;
      case 'game_over':
        gameState.finishGame(ev);
        break;
      case 'game_started':
        if (gameState.game?.id === ev.game_id) {
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
