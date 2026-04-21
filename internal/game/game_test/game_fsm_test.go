package game

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNotifier captures all notifications for assertion in tests.
type mockNotifier struct {
	mu          sync.Mutex
	timeouts    []timeoutCall
	kicks       []string
	turnStarts  []string
	boardFulls  int
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

func (m *mockNotifier) NotifySkip(_ string, _ int, _ bool) {}

func (m *mockNotifier) NotifyBoardFull() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.boardFulls++
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

func (m *mockNotifier) boardFullCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.boardFulls
}

func (m *mockNotifier) timeoutCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.timeouts)
}

// makePlayers constructs a slice of players with the given IDs.
func makePlayers(ids ...string) []*game.Player {
	players := make([]*game.Player, len(ids))
	for i, id := range ids {
		players[i] = &game.Player{ID: id}
	}
	return players
}

const fastTurn = 50 * time.Millisecond

// ─── Run integration tests ──────────────────────────────────────────────────

func TestGame_Run_StartsFirstTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGame(makePlayers("p1", "p2"), n)
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

	nl := testNewLetter
	require.NoError(t, g.SubmitWord("p1", &nl, testWord))

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

func TestGame_Run_TurnSkippedAdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGame(makePlayers("p1", "p2"), n)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	require.NoError(t, g.Skip("p1"))

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

func TestGame_Run_TimeoutThenAckAdvancesTurn(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGame(makePlayers("p1", "p2"), n, game.WithTurnDuration(fastTurn))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.timeoutCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.AckTimeout()

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

// TestGame_Run_ThreeTimeoutsAutoKick uses a single player so each timeout
// hits the same player, reaching MaxConsecutiveTimeouts quickly.
func TestGame_Run_ThreeTimeoutsAutoKick(t *testing.T) {
	n := &mockNotifier{}
	players := makePlayers("p1")
	g, err := game.NewGame(players, n, game.WithTurnDuration(fastTurn))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.Run(ctx)
	}()

	// First two timeouts: below kick threshold, ack to continue.
	for i := 1; i < game.MaxConsecutiveTimeouts; i++ {
		require.Eventually(t, func() bool {
			return n.timeoutCount() >= i
		}, time.Second, 5*time.Millisecond)
		g.AckTimeout()
	}

	// Third timeout triggers auto-kick and game over.
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
	g, err := game.NewGame(players, n, game.WithTurnDuration(fastTurn))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.Run(ctx)
	}()

	// Wait for timeout to put game in PlayerTimedOut state.
	require.Eventually(t, func() bool {
		return n.timeoutCount() >= 1
	}, time.Second, 5*time.Millisecond)

	g.Kick()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("game did not end after explicit kick")
	}

	assert.True(t, players[0].Kicked)
}

// fillBoardExcept fills every cell of the board with "я" except the given cell.
// Must be called before g.Run to avoid data races.
func fillBoardExcept(g *game.Game, skipRow, skipCol uint8) {
	board := g.Board()
	for r := range board.Table {
		for c := range board.Table[r] {
			if board.Table[r][c] == nil && !(uint8(r) == skipRow && uint8(c) == skipCol) {
				board.Table[r][c] = &game.Letter{RowID: uint8(r), ColID: uint8(c), Char: "я"}
			}
		}
	}
}

// submitLastMove places "б" at (3,3) adjacent to н(2,3), forming word "нб".
func submitLastMove(t *testing.T, g *game.Game, playerID string) {
	t.Helper()
	addTestWord(t, "нб")
	nl := game.Letter{RowID: 3, ColID: 3, Char: "б"}
	path := []game.Letter{
		{RowID: 2, ColID: 3, Char: "н"},
		{RowID: 3, ColID: 3, Char: "б"},
	}
	require.NoError(t, g.SubmitWord(playerID, &nl, path))
}

// waitRunDone is a helper that asserts game.Run exits within 2 s.
func waitRunDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("game did not stop")
	}
}

func TestGame_Run_BoardFull_StopsGame(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGameWithWord(makePlayers("p1", "p2"), testBoardWord, n)
	require.NoError(t, err)
	fillBoardExcept(g, 3, 3)

	done := make(chan struct{})
	go func() { defer close(done); g.Run(t.Context()) }()

	require.Eventually(t, func() bool { return n.turnStartCount() >= 1 }, time.Second, 5*time.Millisecond)
	submitLastMove(t, g, "p1")

	waitRunDone(t, done)
}

func TestGame_Run_BoardFull_CallsNotifyBoardFull(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGameWithWord(makePlayers("p1", "p2"), testBoardWord, n)
	require.NoError(t, err)
	fillBoardExcept(g, 3, 3)

	done := make(chan struct{})
	go func() { defer close(done); g.Run(t.Context()) }()

	require.Eventually(t, func() bool { return n.turnStartCount() >= 1 }, time.Second, 5*time.Millisecond)
	submitLastMove(t, g, "p1")

	waitRunDone(t, done)
	assert.Equal(t, 1, n.boardFullCount(), "NotifyBoardFull must be called exactly once")
}

func TestGame_Run_BoardFull_TurnDoesNotAdvance(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGameWithWord(makePlayers("p1", "p2"), testBoardWord, n)
	require.NoError(t, err)
	fillBoardExcept(g, 3, 3)

	done := make(chan struct{})
	go func() { defer close(done); g.Run(t.Context()) }()

	require.Eventually(t, func() bool { return n.turnStartCount() >= 1 }, time.Second, 5*time.Millisecond)
	turnsBeforeMove := n.turnStartCount()
	submitLastMove(t, g, "p1")

	waitRunDone(t, done)
	assert.Equal(t, turnsBeforeMove, n.turnStartCount(), "turn must not advance after board is full")
}

func TestGame_Run_ContextCancellationStopsGame(t *testing.T) {
	n := &mockNotifier{}
	g, err := game.NewGame(makePlayers("p1", "p2"), n)
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
