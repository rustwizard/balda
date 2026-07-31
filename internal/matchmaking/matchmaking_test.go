package matchmaking_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func player(id string, rating int) *game.Player {
	return &game.Player{ID: id, Rating: rating, Type: game.PlayerTypeHuman}
}

// noopCallback is a MatchCallback that always succeeds without side effects.
func noopCallback(_ []*game.Player) error { return nil }

// testCfg returns a config suitable for fast-running tests.
func testCfg() matchmaking.Config {
	return matchmaking.Config{
		InitialRange:   100,
		ExpandStep:     100,
		ExpandInterval: 10 * time.Second,
		TickInterval:   5 * time.Millisecond,
	}
}

// ─── Enqueue / Dequeue ───────────────────────────────────────────────────────

func TestQueue_Enqueue_AddsPlayer(t *testing.T) {
	q := matchmaking.New(testCfg(), noopCallback)
	require.NoError(t, q.Enqueue(player("p1", 500)))
	assert.Equal(t, 1, q.Len())
}

func TestQueue_Enqueue_AlreadyQueued(t *testing.T) {
	q := matchmaking.New(testCfg(), noopCallback)
	require.NoError(t, q.Enqueue(player("p1", 500)))
	err := q.Enqueue(player("p1", 500))
	assert.ErrorIs(t, err, matchmaking.ErrAlreadyQueued)
}

func TestQueue_Dequeue_RemovesPlayer(t *testing.T) {
	q := matchmaking.New(testCfg(), noopCallback)
	require.NoError(t, q.Enqueue(player("p1", 500)))
	require.NoError(t, q.Dequeue("p1"))
	assert.Equal(t, 0, q.Len())
}

func TestQueue_Dequeue_NotQueued(t *testing.T) {
	q := matchmaking.New(testCfg(), noopCallback)
	err := q.Dequeue("ghost")
	assert.ErrorIs(t, err, matchmaking.ErrNotQueued)
}

// ─── tick-level matching tests ───────────────────────────────────────────────

// TestQueue_Tick_NoMatch_RatingsTooFar verifies that players with ratings
// outside the initial window are not matched at t=0.
func TestQueue_Tick_NoMatch_RatingsTooFar(t *testing.T) {
	called := false
	cb := func(_ []*game.Player) error { called = true; return nil }
	q := matchmaking.New(testCfg(), cb)

	enqAt := time.Now()
	p1 := player("p1", 0)
	p2 := player("p2", 1000)
	require.NoError(t, q.Enqueue(p1))
	require.NoError(t, q.Enqueue(p2))

	q.Tick(enqAt) // window = 100, diff = 1000 → no match

	assert.False(t, called)
	assert.Equal(t, 2, q.Len())
}

// TestQueue_Tick_MatchWithinInitialRange verifies two nearby players match immediately.
func TestQueue_Tick_MatchWithinInitialRange(t *testing.T) {
	var matched []*game.Player
	cb := func(ps []*game.Player) error { matched = ps; return nil }
	q := matchmaking.New(testCfg(), cb)

	require.NoError(t, q.Enqueue(player("p1", 500)))
	require.NoError(t, q.Enqueue(player("p2", 550)))

	q.Tick(time.Now()) // window=100, diff=50 → match

	require.Len(t, matched, 2)
	assert.Equal(t, 0, q.Len())
}

// TestQueue_Tick_MatchAfterExpansion verifies that the window grows over time.
// Rating difference = 300, InitialRange = 100, ExpandStep = 100, ExpandInterval = 10s.
// After 25s: steps = floor(25/10) = 2 → window = 100 + 2×100 = 300 → match.
func TestQueue_Tick_MatchAfterExpansion(t *testing.T) {
	var matched []*game.Player
	cb := func(ps []*game.Player) error { matched = ps; return nil }

	cfg := matchmaking.Config{
		InitialRange:   100,
		ExpandStep:     100,
		ExpandInterval: 10 * time.Second,
		TickInterval:   5 * time.Millisecond,
	}
	q := matchmaking.New(cfg, cb)

	enqAt := time.Now().Add(-25 * time.Second) // simulate 25s of waiting

	p1 := player("p1", 0)
	p2 := player("p2", 300)
	require.NoError(t, q.EnqueueAt(p1, enqAt))
	require.NoError(t, q.EnqueueAt(p2, enqAt))

	q.Tick(time.Now())

	require.Len(t, matched, 2, "players should be matched after window expands to 300")
	assert.Equal(t, 0, q.Len())
}

// TestQueue_Tick_ClosestRatingChosen verifies that among multiple candidates the
// one with the smallest rating difference is preferred.
func TestQueue_Tick_ClosestRatingChosen(t *testing.T) {
	var matched []*game.Player
	cb := func(ps []*game.Player) error { matched = ps; return nil }
	q := matchmaking.New(testCfg(), cb)

	pA := player("pA", 500)
	pB := player("pB", 510) // diff from A = 10
	pC := player("pC", 490) // diff from A = 10 (tied — either is acceptable)
	require.NoError(t, q.Enqueue(pA))
	require.NoError(t, q.Enqueue(pB))
	require.NoError(t, q.Enqueue(pC))

	q.Tick(time.Now())

	// Exactly one pair matched; one player remains.
	require.Len(t, matched, 2)
	assert.Equal(t, 1, q.Len())
	// pA must be in the match.
	ids := []string{matched[0].ID, matched[1].ID}
	assert.Contains(t, ids, "pA")
}

// TestQueue_Tick_CallbackError_PlayersReEnqueued verifies that when onMatch
// returns an error both players are placed back in the queue.
func TestQueue_Tick_CallbackError_PlayersReEnqueued(t *testing.T) {
	cb := func(_ []*game.Player) error { return errors.New("game creation failed") }
	q := matchmaking.New(testCfg(), cb)

	require.NoError(t, q.Enqueue(player("p1", 500)))
	require.NoError(t, q.Enqueue(player("p2", 550)))

	q.Tick(time.Now())

	assert.Equal(t, 2, q.Len(), "players should be re-enqueued after callback error")
}

// ─── integration tests ───────────────────────────────────────────────────────

// TestQueue_Run_Integration verifies that two matching players are paired when
// the background loop is running.
func TestQueue_Run_Integration(t *testing.T) {
	var mu sync.Mutex
	var matched []*game.Player
	cb := func(ps []*game.Player) error {
		mu.Lock()
		matched = ps
		mu.Unlock()
		return nil
	}

	q := matchmaking.New(testCfg(), cb)
	require.NoError(t, q.Enqueue(player("p1", 500)))
	require.NoError(t, q.Enqueue(player("p2", 520)))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(matched) == 2
	}, time.Second, 5*time.Millisecond)

	assert.Equal(t, 0, q.Len())
}

// TestQueue_Run_ContextCancel verifies that Run exits when its context is canceled.
func TestQueue_Run_ContextCancel(t *testing.T) {
	q := matchmaking.New(testCfg(), noopCallback)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		q.Run(ctx)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

// TestQueue_ConcurrentEnqueueDequeue checks for data races under concurrent use.
func TestQueue_ConcurrentEnqueueDequeue(_ *testing.T) {
	q := matchmaking.New(testCfg(), noopCallback)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		id := "p" + string(rune('A'+i%26))
		go func(pid string) {
			defer wg.Done()
			p := player(pid, i*10)
			_ = q.Enqueue(p)
			time.Sleep(time.Millisecond)
			_ = q.Dequeue(pid)
		}(id)
	}
	wg.Wait()
}

// TestQueue_Tick_ExpiresAfterMaxWait verifies that an entry older than MaxWait
// is removed from the queue and handed to the expire callback (bot fallback).
func TestQueue_Tick_ExpiresAfterMaxWait(t *testing.T) {
	cfg := testCfg()
	cfg.MaxWait = time.Second

	var expired []string
	q := matchmaking.New(cfg, noopCallback).WithExpireCallback(func(p *game.Player) error {
		expired = append(expired, p.ID)
		return nil
	})

	require.NoError(t, q.EnqueueAt(player("p1", 500), time.Now().Add(-2*time.Second)))
	require.NoError(t, q.Enqueue(player("p2", 520))) // fresh entry stays

	q.Tick(time.Now())

	assert.Equal(t, []string{"p1"}, expired)
	assert.Equal(t, 1, q.Len(), "only the fresh entry should remain")
}

// TestQueue_Tick_RemovesOffline verifies that entries whose player is offline
// are silently removed (no expire callback).
func TestQueue_Tick_RemovesOffline(t *testing.T) {
	var expired []string
	q := matchmaking.New(testCfg(), noopCallback).
		WithExpireCallback(func(p *game.Player) error {
			expired = append(expired, p.ID)
			return nil
		}).
		WithOnlineChecker(func(playerID string) bool { return playerID != "p1" })

	require.NoError(t, q.Enqueue(player("p1", 500)))
	require.NoError(t, q.Enqueue(player("p2", 520)))

	q.Tick(time.Now())

	assert.Empty(t, expired)
	assert.Equal(t, 1, q.Len())
}

// TestQueue_Tick_ExpiredNotMatched verifies that an expired entry is not
// considered for pairing even if it would match.
func TestQueue_Tick_ExpiredNotMatched(t *testing.T) {
	cfg := testCfg()
	cfg.MaxWait = time.Second

	matched := 0
	var expired []string
	q := matchmaking.New(cfg, func(_ []*game.Player) error {
		matched++
		return nil
	}).WithExpireCallback(func(p *game.Player) error {
		expired = append(expired, p.ID)
		return nil
	})

	require.NoError(t, q.EnqueueAt(player("p1", 500), time.Now().Add(-2*time.Second)))
	require.NoError(t, q.Enqueue(player("p2", 505)))

	q.Tick(time.Now())

	assert.Equal(t, 0, matched, "expired entry must not be matched")
	assert.Equal(t, []string{"p1"}, expired)
	assert.Equal(t, 1, q.Len())
}
