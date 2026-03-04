package lobby_test

// Integration tests for lobby + matchmaking working together.
// These tests wire both packages the same way a real server would and verify
// end-to-end behaviour: from a player entering the queue to a game running in
// the lobby and eventually ending.

import (
	"context"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/rustwizard/balda/internal/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── system helpers ──────────────────────────────────────────────────────────

// mmCfg is a matchmaking config suitable for fast-running tests.
// TickInterval is 5 ms so the Run-based tests finish well within a second.
func mmCfg() matchmaking.Config {
	return matchmaking.Config{
		InitialRange:   100,
		ExpandStep:     100,
		ExpandInterval: 10 * time.Second,
		TickInterval:   5 * time.Millisecond,
	}
}

// newSystem wires a Lobby and a Queue together the same way the real server
// does. The returned cancel stops both the queue loop and all running games.
// Callers must call cancel() (typically via defer).
func newSystem(t testing.TB) (*lobby.Lobby, *matchmaking.Queue, context.CancelFunc) {
	t.Helper()
	lby := newLobby() // defined in lobby_test.go: uses realFactory + "волна" board
	ctx, cancel := context.WithCancel(context.Background())
	q := matchmaking.New(mmCfg(), func(players []*game.Player) error {
		_, err := lby.StartGame(ctx, players, &notifier.Noop{})
		return err
	})
	return lby, q, cancel
}

// ratedPlayer is a shorthand that sets both ID and Exp.
func ratedPlayer(id string, exp int) *game.Player {
	return &game.Player{ID: id, Exp: exp}
}

// ─── lobby × matchmaking integration tests ───────────────────────────────────

// TestIntegration_MatchStartsGameInLobby verifies the fundamental wiring:
// when the queue finds two compatible players it calls StartGame and both
// players become findable in the lobby.
func TestIntegration_MatchStartsGameInLobby(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	p1 := ratedPlayer("p1", 500)
	p2 := ratedPlayer("p2", 550) // diff=50, within InitialRange=100

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(time.Now())

	// StartGame is called synchronously inside Tick → game already registered.
	require.Len(t, lby.List(), 1)
	assert.Equal(t, 0, q.Len())

	_, err := lby.FindByPlayer("p1")
	assert.NoError(t, err)
	_, err = lby.FindByPlayer("p2")
	assert.NoError(t, err)
}

// TestIntegration_BothPlayersInSameGame verifies that matched players end up
// in the same game (same game ID returned by FindByPlayer for each).
func TestIntegration_BothPlayersInSameGame(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	p1 := ratedPlayer("p1", 500)
	p2 := ratedPlayer("p2", 560)

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(time.Now())

	s1, err := lby.FindByPlayer("p1")
	require.NoError(t, err)
	s2, err := lby.FindByPlayer("p2")
	require.NoError(t, err)

	assert.Equal(t, s1.ID, s2.ID, "matched players must be in the same game")
}

// TestIntegration_GameEnds_LobbyClears verifies that when the game's context
// is cancelled (server shutdown or explicit remove) the lobby removes the entry
// automatically, so List() eventually returns empty.
func TestIntegration_GameEnds_LobbyClears(t *testing.T) {
	lby, q, cancel := newSystem(t)

	p1 := ratedPlayer("p1", 500)
	p2 := ratedPlayer("p2", 500)

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(time.Now())
	require.Len(t, lby.List(), 1)

	cancel() // stops game.Run → lobby.onDone fires asynchronously

	require.Eventually(t, func() bool {
		return len(lby.List()) == 0
	}, time.Second, 5*time.Millisecond, "lobby must be empty after game ends")
}

// TestIntegration_GameEnds_PlayersCanRequeue verifies that after a game ends
// and the lobby clears, both players can enter the matchmaking queue again.
// (The queue never held them after the match; the lobby no longer holds them
// after the game ends — so re-enqueue must not return ErrAlreadyQueued.)
func TestIntegration_GameEnds_PlayersCanRequeue(t *testing.T) {
	lby, q, cancel := newSystem(t)

	p1 := ratedPlayer("p1", 500)
	p2 := ratedPlayer("p2", 510)

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(time.Now())
	require.Len(t, lby.List(), 1)

	cancel() // ends the game

	require.Eventually(t, func() bool {
		return len(lby.List()) == 0
	}, time.Second, 5*time.Millisecond)

	// Both players are free to search for a new game.
	assert.NoError(t, q.Enqueue(p1))
	assert.NoError(t, q.Enqueue(p2))
}

// TestIntegration_RatingBasedPairing places two close pairs far apart in Exp.
// The matcher must pair A↔B and C↔D, never crossing pairs.
func TestIntegration_RatingBasedPairing(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	// Pair 1: Exp 500 / 510 (diff=10, well within window=100)
	pA := ratedPlayer("pA", 500)
	pB := ratedPlayer("pB", 510)
	// Pair 2: Exp 1000 / 1010 (diff=10, well within window=100;
	//         distance to pair 1 = 490, outside window=100)
	pC := ratedPlayer("pC", 1000)
	pD := ratedPlayer("pD", 1010)

	require.NoError(t, q.Enqueue(pA))
	require.NoError(t, q.Enqueue(pB))
	require.NoError(t, q.Enqueue(pC))
	require.NoError(t, q.Enqueue(pD))

	q.Tick(time.Now())

	require.Len(t, lby.List(), 2, "two games should start")
	assert.Equal(t, 0, q.Len())

	sA, _ := lby.FindByPlayer("pA")
	sB, _ := lby.FindByPlayer("pB")
	sC, _ := lby.FindByPlayer("pC")
	sD, _ := lby.FindByPlayer("pD")

	assert.Equal(t, sA.ID, sB.ID, "pA and pB must be in the same game")
	assert.Equal(t, sC.ID, sD.ID, "pC and pD must be in the same game")
	assert.NotEqual(t, sA.ID, sC.ID, "the two games must be distinct")
}

// TestIntegration_DequeueBeforeMatch verifies that a player who cancels their
// search before the tick fires is not matched, and the remaining player stays
// in the queue unaffected.
func TestIntegration_DequeueBeforeMatch(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	p1 := ratedPlayer("p1", 500)
	p2 := ratedPlayer("p2", 510)

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	require.NoError(t, q.Dequeue("p1")) // p1 cancels before tick

	q.Tick(time.Now())

	assert.Empty(t, lby.List(), "no game should start with only one eligible player")
	assert.Equal(t, 1, q.Len(), "p2 must still be waiting")

	_, err := lby.FindByPlayer("p2")
	assert.ErrorIs(t, err, lobby.ErrGameNotFound)
}

// TestIntegration_IncompatibleRatings_NoMatch verifies that players outside
// each other's initial window are NOT matched on an immediate tick.
func TestIntegration_IncompatibleRatings_NoMatch(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	// diff=600 >> InitialRange=100
	p1 := ratedPlayer("p1", 0)
	p2 := ratedPlayer("p2", 600)

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(time.Now()) // window=100 < 600 → no match

	assert.Empty(t, lby.List())
	assert.Equal(t, 2, q.Len())
}

// TestIntegration_WindowExpansion_MatchesFarRatingPlayers verifies that players
// whose ratings are outside the initial window eventually match as their window
// expands with wait time.
//
// Setup: InitialRange=100, ExpandStep=100, ExpandInterval=10s.
// Exp diff=600. At t=0: window=100 (no match).
// At enqueuedAt+60s: steps=6 → window=700 > 600 (match).
func TestIntegration_WindowExpansion_MatchesFarRatingPlayers(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	p1 := ratedPlayer("p1", 0)
	p2 := ratedPlayer("p2", 600)

	// No match at time of enqueue.
	now := time.Now()
	require.NoError(t, q.EnqueueAt(p1, now))
	require.NoError(t, q.EnqueueAt(p2, now))
	q.Tick(now) // window=100 → no match
	assert.Empty(t, lby.List())
	assert.Equal(t, 2, q.Len())

	// Simulate 60 seconds of waiting → window=700 → match.
	q.Tick(now.Add(60 * time.Second))
	require.Len(t, lby.List(), 1)
	assert.Equal(t, 0, q.Len())
}

// TestIntegration_OnlyFasterWindow_MustBeSatisfiedByBoth ensures that the
// min(window(A), window(B)) rule is enforced: if only A's window is large
// enough but B just joined, the match must not fire.
//
// A has been waiting 60s (window=700). B just joined (window=100).
// diff=600 > min(700,100)=100 → no match.
func TestIntegration_WindowMustSatisfyBoth(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	now := time.Now()
	p1 := ratedPlayer("p1", 0)
	p2 := ratedPlayer("p2", 600)

	require.NoError(t, q.EnqueueAt(p1, now.Add(-60*time.Second))) // waited 60s
	require.NoError(t, q.EnqueueAt(p2, now))                      // just joined

	q.Tick(now) // A's window=700, B's window=100 → min=100 < 600 → no match

	assert.Empty(t, lby.List(), "match must not fire when only one player's window covers the gap")
	assert.Equal(t, 2, q.Len())
}

// TestIntegration_Run_MultipleMatches verifies that the Run goroutine correctly
// handles several concurrent pairs: all matching players get paired and all
// games appear in the lobby.
func TestIntegration_Run_MultipleMatches(t *testing.T) {
	lby, q, cancel := newSystem(t)
	defer cancel()

	// 6 players in 3 close pairs.
	pairs := [][2]*game.Player{
		{ratedPlayer("a1", 100), ratedPlayer("a2", 110)},
		{ratedPlayer("b1", 500), ratedPlayer("b2", 490)},
		{ratedPlayer("c1", 900), ratedPlayer("c2", 905)},
	}
	for _, pair := range pairs {
		require.NoError(t, q.Enqueue(pair[0]))
		require.NoError(t, q.Enqueue(pair[1]))
	}

	go q.Run(context.Background()) // use background ctx; parent cancel stops games only

	require.Eventually(t, func() bool {
		return len(lby.List()) == 3
	}, time.Second, 5*time.Millisecond, "all 3 pairs must be matched and their games started")

	assert.Equal(t, 0, q.Len())
}

// TestIntegration_PlayerInGame_BlocksSecondGame verifies the contract between
// matchmaking and lobby: if onMatch is called with a player already in a game
// (e.g. due to a late duplicate match), the lobby rejects it with ErrPlayerInGame
// and both players are re-enqueued by the queue.
func TestIntegration_OnMatchError_PlayersReEnqueued(t *testing.T) {
	// Use a callback that always fails to simulate lobby rejection.
	q := matchmaking.New(mmCfg(), func(_ []*game.Player) error {
		return lobby.ErrPlayerInGame // pretend player is already in a game
	})

	p1 := ratedPlayer("p1", 500)
	p2 := ratedPlayer("p2", 510)

	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(time.Now())

	// Match fired but callback failed → both re-enqueued.
	assert.Equal(t, 2, q.Len(), "both players must be back in queue after lobby rejection")
}
