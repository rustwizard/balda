package game

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTurnTimeout_StaleEventIgnored is a white-box regression test for the
// stale-timeout race: a turn timer that fires just as the turn is changing
// queues EventTurnTimeout, and by the time Run processes it the turn already
// belongs to the next player. Without turn sequence tagging, the timeout
// would be charged to that next player (three of those kick them).
//
// The test injects events into eventCh directly to make the race
// deterministic instead of relying on timer scheduling.
func TestTurnTimeout_StaleEventIgnored(t *testing.T) {
	Dict.Definition["волне"] = "test-definition"
	t.Cleanup(func() { delete(Dict.Definition, "волне") })

	players := []*Player{
		{ID: "p1", Type: PlayerTypeHuman},
		{ID: "p2", Type: PlayerTypeHuman},
	}
	g, err := NewGameWithWord(players, "волна", &NoopNotifier{}, WithTurnDuration(time.Hour))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	// Wait for the first turn to start and remember its sequence number.
	require.Eventually(t, func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.turnSeq == 1
	}, time.Second, 5*time.Millisecond)

	g.mu.Lock()
	staleSeq := g.turnSeq
	g.mu.Unlock()

	// p1 moves; the turn passes to p2 synchronously (turnSeq becomes 2).
	newLetter := Letter{RowID: 3, ColID: 3, Char: "е"}
	word := []Letter{
		{RowID: 2, ColID: 0, Char: "в"},
		{RowID: 2, ColID: 1, Char: "о"},
		{RowID: 2, ColID: 2, Char: "л"},
		{RowID: 2, ColID: 3, Char: "н"},
		{RowID: 3, ColID: 3, Char: "е"},
	}
	require.NoError(t, g.SubmitWord("p1", &newLetter, word))
	require.Equal(t, "p2", g.CurrentPlayerID())

	// A timeout from p1's turn arrives late — it must be discarded.
	g.eventCh <- fsmEvent{ev: EventTurnTimeout, turnSeq: staleSeq}

	// Give Run ample time to process the event, then verify nothing changed.
	// (Same negative-wait pattern as TestGame_ProposeEnd_PausesTimer.)
	time.Sleep(100 * time.Millisecond)

	g.mu.Lock()
	assert.Equal(t, 0, players[1].ConsecutiveTimeouts, "stale timeout must not penalize the next player")
	assert.Equal(t, StateWaitingForMove, g.state, "stale timeout must not change the game state")
	g.mu.Unlock()

	// Sanity check: a timeout tagged with the current turn does apply.
	g.mu.Lock()
	currentSeq := g.turnSeq
	g.mu.Unlock()
	g.eventCh <- fsmEvent{ev: EventTurnTimeout, turnSeq: currentSeq}

	require.Eventually(t, func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return players[1].ConsecutiveTimeouts == 1
	}, time.Second, 5*time.Millisecond)
}
