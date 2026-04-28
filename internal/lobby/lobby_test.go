package lobby_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func makePlayers(ids ...string) []*game.Player {
	out := make([]*game.Player, len(ids))
	for i, id := range ids {
		out[i] = &game.Player{ID: id}
	}
	return out
}

// realFactory creates a real game (using a fixed initial word to skip random dict lookup).
func realFactory(_ context.Context, _ string, players []*game.Player, n game.Notifier) (*game.Game, error) {
	return game.NewGameWithWord(players, "волна", n)
}

func newLobby() *lobby.Lobby {
	return lobby.New(realFactory)
}

// ─── unit tests ─────────────────────────────────────────────────────────────

func TestLobby_StartGame_RegistersGame(t *testing.T) {
	l := newLobby()
	players := makePlayers("p1", "p2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec, err := l.StartGame(ctx, players, &notifier.Noop{})
	require.NoError(t, err)
	require.NotEmpty(t, rec.ID)

	games := l.List()
	require.Len(t, games, 1)
	assert.Equal(t, rec.ID, games[0].ID)
	gotIDs := make([]string, len(games[0].Players))
	for i, p := range games[0].Players {
		gotIDs[i] = p.ID
	}
	assert.ElementsMatch(t, []string{"p1", "p2"}, gotIDs)
}

func TestLobby_StartGame_PlayerAlreadyInGame(t *testing.T) {
	l := newLobby()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := l.StartGame(ctx, makePlayers("p1", "p2"), &notifier.Noop{})
	require.NoError(t, err)

	// p1 tries to join another game.
	_, err = l.StartGame(ctx, makePlayers("p1", "p3"), &notifier.Noop{})
	assert.ErrorIs(t, err, lobby.ErrPlayerInGame)
}

func TestLobby_Remove_DeletesGame(t *testing.T) {
	l := newLobby()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec, err := l.StartGame(ctx, makePlayers("p1", "p2"), &notifier.Noop{})
	require.NoError(t, err)

	require.NoError(t, l.Remove(rec.ID))

	assert.Empty(t, l.List())

	_, err = l.FindByPlayer("p1")
	assert.ErrorIs(t, err, lobby.ErrGameNotFound)
}

func TestLobby_Remove_UnknownID(t *testing.T) {
	l := newLobby()
	err := l.Remove("nonexistent-id")
	assert.ErrorIs(t, err, lobby.ErrGameNotFound)
}

func TestLobby_Get_Found(t *testing.T) {
	l := newLobby()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec, err := l.StartGame(ctx, makePlayers("p1", "p2"), &notifier.Noop{})
	require.NoError(t, err)

	got, err := l.Get(rec.ID)
	require.NoError(t, err)
	assert.Equal(t, rec.ID, got.ID)
}

func TestLobby_Get_NotFound(t *testing.T) {
	l := newLobby()
	_, err := l.Get("ghost")
	assert.ErrorIs(t, err, lobby.ErrGameNotFound)
}

func TestLobby_FindByPlayer_Found(t *testing.T) {
	l := newLobby()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec, err := l.StartGame(ctx, makePlayers("p1", "p2"), &notifier.Noop{})
	require.NoError(t, err)

	summary, err := l.FindByPlayer("p1")
	require.NoError(t, err)
	assert.Equal(t, rec.ID, summary.ID)
}

func TestLobby_FindByPlayer_NotFound(t *testing.T) {
	l := newLobby()
	_, err := l.FindByPlayer("nobody")
	assert.ErrorIs(t, err, lobby.ErrGameNotFound)
}

// ─── integration tests ───────────────────────────────────────────────────────

// TestLobby_GameEndsAutomatically verifies that when a game's Run goroutine
// finishes (here via context cancel), the lobby removes the game automatically.
func TestLobby_GameEndsAutomatically(t *testing.T) {
	l := newLobby()

	ctx, cancel := context.WithCancel(context.Background())

	_, err := l.StartGame(ctx, makePlayers("p1", "p2"), &notifier.Noop{})
	require.NoError(t, err)

	require.Len(t, l.List(), 1)

	cancel() // stops game.Run

	require.Eventually(t, func() bool {
		return len(l.List()) == 0
	}, time.Second, 5*time.Millisecond, "game should be removed from lobby after Run exits")

	_, err = l.FindByPlayer("p1")
	assert.ErrorIs(t, err, lobby.ErrGameNotFound)
}

// TestLobby_ConcurrentAccess checks for data races under concurrent load.
func TestLobby_ConcurrentAccess(t *testing.T) {
	l := newLobby()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		playerID := "player-" + string(rune('A'+i))
		go func(pid string) {
			defer wg.Done()
			rec, err := l.StartGame(ctx, makePlayers(pid), &notifier.Noop{})
			if err != nil && !errors.Is(err, lobby.ErrPlayerInGame) {
				t.Errorf("unexpected StartGame error: %v", err)
				return
			}
			if err == nil {
				_ = l.List()
				_, _ = l.FindByPlayer(pid)
				_ = l.Remove(rec.ID)
			}
		}(playerID)
	}
	wg.Wait()
}
