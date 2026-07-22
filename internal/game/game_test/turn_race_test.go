package game

// Regression tests for the turn-validation race: before the fix, SubmitWord
// and Skip validated under g.mu but committed the state change asynchronously
// via eventCh, so a second request validated against the stale state passed
// too. That allowed a double move (extra score) and a move+skip combo that
// attributed the skip to the opponent (three of those kicked them).
// The FSM transition is now applied synchronously, before the call returns.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubmitWord_TurnAdvancesSynchronously verifies that once SubmitWord
// returns, the turn already belongs to the opponent: the mover cannot move
// again and cannot skip the opponent's turn.
func TestSubmitWord_TurnAdvancesSynchronously(t *testing.T) {
	n := &mockNotifier{}
	g, players := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	addTestWord(t, testWordStr)

	nl := testNewLetter
	require.NoError(t, g.SubmitWord("p1", &nl, testWord))

	// No waiting: the transition must already be committed.
	assert.Equal(t, "p2", g.CurrentPlayerID())

	err := g.SubmitWord("p1", &nl, testWord)
	assert.ErrorIs(t, err, game.ErrNotYourTurn, "second move by the same player must be rejected")

	assert.ErrorIs(t, g.Skip("p1"), game.ErrNotYourTurn, "move+skip combo must not skip the opponent's turn")

	assert.Len(t, players[0].Words, 1, "only one word may be recorded")
	assert.Equal(t, 0, players[1].ConsecutiveSkips, "skip must not be attributed to the opponent")
}

// TestSubmitWord_ConcurrentDoubleMove hammers SubmitWord from many goroutines;
// exactly one move may be accepted.
//
// Run: go test -race -run TestSubmitWord_ConcurrentDoubleMove ./internal/game/...
func TestSubmitWord_ConcurrentDoubleMove(t *testing.T) {
	n := &mockNotifier{}
	players := makePlayers("p1", "p2")
	g, err := game.NewGameWithWord(players, testBoardWord, n)
	require.NoError(t, err)
	addTestWord(t, testWordStr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	const goroutines = 10
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nl := testNewLetter
			if err := g.SubmitWord("p1", &nl, testWord); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), successes.Load(), "exactly one concurrent move may be accepted")
	assert.Equal(t, "p2", g.CurrentPlayerID())
	assert.Equal(t, len(testWord), players[0].Score, "score must be credited exactly once")
}

// TestSkip_ThirdSkipKicksSkipper verifies that the kick after the third
// consecutive skip hits the skipping player, not whoever happens to be the
// current player when a queued kick event would have been processed.
func TestSkip_ThirdSkipKicksSkipper(t *testing.T) {
	n := &mockNotifier{}
	g, players := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")

	// Simulate that p1 has already skipped twice in a row.
	players[0].ConsecutiveSkips = game.MaxConsecutiveSkips - 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); g.Run(ctx) }()

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	require.NoError(t, g.Skip("p1"))

	// The kick is applied synchronously: p1 is kicked, p2 is untouched,
	// and the game is over (Run exits).
	assert.True(t, players[0].Kicked, "the skipping player must be kicked")
	assert.False(t, players[1].Kicked, "the opponent must not be kicked")
	waitRunDone(t, done)

	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.kicks, 1)
	assert.Equal(t, "p1", n.kicks[0])
}
