<script lang="ts">
  import AuthForm from './components/AuthForm.svelte';
  import Lobby from './components/Lobby.svelte';
  import WaitingScreen from './components/WaitingScreen.svelte';
  import GameScreen from './components/GameScreen.svelte';
  import { gameState } from './stores/game.svelte';
  import { centrifugo } from './lib/centrifugo';
  import type { CentrifugoEvent } from './types';

  // API key for demo - normally from env or config
  const DEMO_API_KEY = import.meta.env.VITE_API_KEY || 'abcdefuvwxyz';
  const CENTRIFUGO_WS_URL = import.meta.env.VITE_CENTRIFUGO_WS_URL || 'ws://localhost:8080/connection/websocket';

  // Connect to Centrifugo after auth
  $effect(() => {
    if (gameState.phase !== 'auth' && gameState.centrifugoToken) {
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
        if (gameState.game?.id === ev.game.id) {
          gameState.startGame(ev.game);
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
