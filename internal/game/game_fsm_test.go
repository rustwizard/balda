package game

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNotifier captures all notifications for assertion in tests.
type mockNotifier struct {
	mu         sync.Mutex
	timeouts   []timeoutCall
	kicks      []string
	turnStarts []string
}

type timeoutCall struct {
	playerID    string
	consecutive int
	willKick    bool
}

func (m *mockNotifier) NotifyTimeout(playerID string, consecutive int, willKick bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeouts = append(m.timeouts, timeoutCall{playerID, consecutive, willKick})
}

func (m *mockNotifier) NotifyKick(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.kicks = append(m.kicks, playerID)
}

func (m *mockNotifier) NotifyTurnStart(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turnStarts = append(m.turnStarts, playerID)
}

func (m *mockNotifier) lastTurnStart() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.turnStarts) == 0 {
		return ""
	}
	return m.turnStarts[len(m.turnStarts)-1]
}

func (m *mockNotifier) turnStartCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.turnStarts)
}

func (m *mockNotifier) timeoutCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.timeouts)
}

// makePlayers constructs a slice of players with the given IDs.
func makePlayers(ids ...string) []*Player {
	players := make([]*Player, len(ids))
	for i, id := range ids {
		players[i] = &Player{ID: id}
	}
	return players
}

// makeTestGame creates a Game in WaitingForMove state with a long-lived timer
// so dispatch tests don't accidentally fire the real turn timer.
// Callers must defer g.cancelTimer() to prevent goroutine leaks.
func makeTestGame(t testing.TB, n *mockNotifier, ids ...string) *Game {
	t.Helper()
	players := makePlayers(ids...)
	g, err := NewGame(players, n)
	require.NoError(t, err)
	g.state = StateWaitingForMove
	g.turn = &Turn{
		PlayerID: players[0].ID,
		timer:    time.AfterFunc(time.Hour, func() {}),
	}
	return g
}

// ─── dispatch unit tests ────────────────────────────────────────────────────

func TestDispatch_MoveSubmitted_AdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	defer g.cancelTimer()

	g.dispatch(EventMoveSubmitted)

	assert.Equal(t, StateWaitingForMove, g.state)
	assert.Equal(t, 1, g.current)
	assert.Equal(t, "p2", n.lastTurnStart())
}

func TestDispatch_MoveSubmitted_ResetsConsecutiveTimeouts(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	g.players[0].ConsecutiveTimeouts = 2
	defer g.cancelTimer()

	g.dispatch(EventMoveSubmitted)

	assert.Equal(t, 0, g.players[0].ConsecutiveTimeouts)
}

func TestDispatch_TurnSkipped_AdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	defer g.cancelTimer()

	g.dispatch(EventTurnSkipped)

	assert.Equal(t, StateWaitingForMove, g.state)
	assert.Equal(t, 1, g.current)
	assert.Equal(t, "p2", n.lastTurnStart())
}

func TestDispatch_TurnSkipped_ResetsConsecutiveTimeouts(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	g.players[0].ConsecutiveTimeouts = 1
	defer g.cancelTimer()

	g.dispatch(EventTurnSkipped)

	assert.Equal(t, 0, g.players[0].ConsecutiveTimeouts)
}

func TestDispatch_TurnTimeout_TransitionsToPlayerTimedOut(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	defer g.cancelTimer()

	g.dispatch(EventTurnTimeout)

	assert.Equal(t, StatePlayerTimedOut, g.state)
	assert.Equal(t, 0, g.current) // turn did NOT advance
	assert.Equal(t, 1, g.players[0].ConsecutiveTimeouts)
	require.Len(t, n.timeouts, 1)
	assert.Equal(t, "p1", n.timeouts[0].playerID)
	assert.Equal(t, 1, n.timeouts[0].consecutive)
	assert.False(t, n.timeouts[0].willKick)
}

func TestDispatch_TurnTimeout_ThirdTimeout_WillKick(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	g.players[0].ConsecutiveTimeouts = MaxConsecutiveTimeouts - 1
	defer g.cancelTimer()

	g.dispatch(EventTurnTimeout)

	assert.Equal(t, StatePlayerTimedOut, g.state)
	assert.Equal(t, MaxConsecutiveTimeouts, g.players[0].ConsecutiveTimeouts)
	require.Len(t, n.timeouts, 1)
	assert.True(t, n.timeouts[0].willKick)

	// EventKick must have been queued into the buffered channel.
	select {
	case ev := <-g.eventCh:
		assert.Equal(t, EventKick, ev)
	default:
		t.Fatal("expected EventKick to be queued in eventCh")
	}
}

func TestDispatch_AckTimeout_AdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	g.state = StatePlayerTimedOut
	defer g.cancelTimer()

	g.dispatch(EventAckTimeout)

	assert.Equal(t, StateWaitingForMove, g.state)
	assert.Equal(t, 1, g.current)
	assert.Equal(t, "p2", n.lastTurnStart())
}

func TestDispatch_Kick_TransitionsToGameOver(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	g.state = StatePlayerTimedOut
	defer g.cancelTimer()

	g.dispatch(EventKick)

	assert.Equal(t, StateGameOver, g.state)
	assert.True(t, g.players[0].Kicked)
	require.Len(t, n.kicks, 1)
	assert.Equal(t, "p1", n.kicks[0])
}

func TestDispatch_IgnoredEvent_DoesNotChangeState(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	defer g.cancelTimer()

	// EventAckTimeout is not valid in StateWaitingForMove.
	g.dispatch(EventAckTimeout)

	assert.Equal(t, StateWaitingForMove, g.state)
	assert.Equal(t, 0, g.current)
	assert.Empty(t, n.turnStarts)
}

func TestDispatch_TurnWraparound(t *testing.T) {
	n := &mockNotifier{}
	g := makeTestGame(t, n, "p1", "p2")
	g.current = 1
	g.turn.PlayerID = "p2"
	defer g.cancelTimer()

	g.dispatch(EventMoveSubmitted)

	assert.Equal(t, 0, g.current)
	assert.Equal(t, "p1", n.lastTurnStart())
}

// ─── Run integration tests ──────────────────────────────────────────────────

func TestGame_Run_StartsFirstTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := NewGame(makePlayers("p1", "p2"), n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	assert.Equal(t, "p1", n.lastTurnStart())
}

func TestGame_Run_MoveSubmittedAdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := NewGame(makePlayers("p1", "p2"), n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.eventCh <- EventMoveSubmitted

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

func TestGame_Run_TurnSkippedAdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := NewGame(makePlayers("p1", "p2"), n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.eventCh <- EventTurnSkipped

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

func TestGame_Run_TimeoutThenAckAdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := NewGame(makePlayers("p1", "p2"), n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.eventCh <- EventTurnTimeout

	require.Eventually(t, func() bool {
		return n.timeoutCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.eventCh <- EventAckTimeout

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

// TestGame_Run_ThreeTimeoutsAutoKick uses a single player so each timeout
// hits the same player, reaching MaxConsecutiveTimeouts quickly.
func TestGame_Run_ThreeTimeoutsAutoKick(t *testing.T) {
	n := &mockNotifier{}
	players := makePlayers("p1")
	g, err := NewGame(players, n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	// First two timeouts: below kick threshold, ack to continue.
	for i := 1; i < MaxConsecutiveTimeouts; i++ {
		g.eventCh <- EventTurnTimeout
		require.Eventually(t, func() bool {
			return n.timeoutCount() >= i
		}, time.Second, 5*time.Millisecond)
		g.eventCh <- EventAckTimeout
	}

	// Third timeout triggers auto-kick and game over.
	g.eventCh <- EventTurnTimeout

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("game did not end after third consecutive timeout")
	}

	assert.True(t, players[0].Kicked)
	n.mu.Lock()
	defer n.mu.Unlock()
	require.Len(t, n.kicks, 1)
	assert.Equal(t, "p1", n.kicks[0])
}

func TestGame_Run_ExplicitKickEndsGame(t *testing.T) {
	n := &mockNotifier{}
	players := makePlayers("p1", "p2")
	g, err := NewGame(players, n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	// Trigger timeout to enter PlayerTimedOut state, then kick.
	g.eventCh <- EventTurnTimeout
	require.Eventually(t, func() bool {
		return n.timeoutCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.eventCh <- EventKick

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("game did not end after explicit kick")
	}

	assert.True(t, players[0].Kicked)
}

func TestGame_Run_ContextCancellationStopsGame(t *testing.T) {
	n := &mockNotifier{}
	g, err := NewGame(makePlayers("p1", "p2"), n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.Run(ctx)
	}()

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("game did not stop after context cancellation")
	}
}
