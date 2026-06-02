package game

// TestBoardSnapshot_NoRace verifies that BoardSnapshot() is safe to call
// concurrently with SubmitWord.
//
// Before this fix, Board() returned a *LettersTable whose Table array could be
// read (via AsStrings) without holding g.mu while SubmitWord wrote to it under
// g.mu — a classic data race. BoardSnapshot() copies the board under g.mu so
// callers never hold a raw pointer to the board.
//
// Run: go test -race -run TestBoardSnapshot_NoRace ./internal/game/...

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/require"
)

// snapshotNotifier mirrors what gamecoord.Coordinator does: on each
// NotifyTurnStart it reads the board state in a goroutine.
// Uses BoardSnapshot() (the fixed API) so there must be no race.
type snapshotNotifier struct {
	g          *game.Game
	startCount atomic.Int32
}

func (n *snapshotNotifier) NotifyTurnStart(_ string) {
	n.startCount.Add(1)
	// Called from within dispatch which may hold g.mu; must not call
	// BoardSnapshot() synchronously here — launch a goroutine exactly as
	// Coordinator does.
	go func() {
		_ = n.g.BoardSnapshot() // copies board under g.mu — safe
		runtime.Gosched()
	}()
}
func (n *snapshotNotifier) NotifyTimeout(_ string, _ int, _ bool) {}
func (n *snapshotNotifier) NotifySkip(_ string, _ int, _ bool)    {}
func (n *snapshotNotifier) NotifyKick(_ string)                   {}

func TestBoardSnapshot_NoRace(t *testing.T) {
	addTestWord(t, testWordStr)

	notif := &snapshotNotifier{}
	players := makePlayers("p1", "p2")

	g, err := game.NewGameWithWord(players, testBoardWord, notif)
	require.NoError(t, err)
	notif.g = g

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return notif.startCount.Load() > 0
	}, time.Second, time.Millisecond)

	// Concurrently submit words from multiple goroutines while the notifier
	// reads the board via BoardSnapshot(). The race detector must stay silent.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = g.SubmitWord("p1", &testNewLetter, testWord)
		}()
	}
	wg.Wait()

	time.Sleep(50 * time.Millisecond)
}
