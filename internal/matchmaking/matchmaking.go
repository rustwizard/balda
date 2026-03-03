// Package matchmaking implements a rating-based matchmaking queue.
// Players enter the queue and are paired with opponents of similar Exp rating.
// The longer a player waits, the wider the acceptable rating gap becomes.
package matchmaking

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/rustwizard/balda/internal/game"
)

var (
	ErrAlreadyQueued = errors.New("matchmaking: player already in queue")
	ErrNotQueued     = errors.New("matchmaking: player not in queue")
)

// Config controls all tunable parameters of the matchmaker.
type Config struct {
	// InitialRange is the starting acceptable Exp difference between players.
	InitialRange int
	// ExpandStep is the number of Exp units added per ExpandInterval of waiting.
	ExpandStep int
	// ExpandInterval is how long a player must wait before the range widens by ExpandStep.
	ExpandInterval time.Duration
	// TickInterval is how often the queue is scanned for matches.
	TickInterval time.Duration
}

// DefaultConfig returns production-sensible matchmaking defaults.
func DefaultConfig() Config {
	return Config{
		InitialRange:   100,
		ExpandStep:     50,
		ExpandInterval: 10 * time.Second,
		TickInterval:   2 * time.Second,
	}
}

// MatchCallback is called (outside the queue lock) when two players are matched.
// If it returns an error both players are re-enqueued and matching is retried
// on the next tick.
type MatchCallback func(players []*game.Player) error

type entry struct {
	player     *game.Player
	enqueuedAt time.Time
}

// Queue is the matchmaking queue. Safe for concurrent use.
type Queue struct {
	mu      sync.Mutex
	entries []*entry          // insertion-ordered (FIFO within same Exp bucket)
	indexed map[string]*entry // playerID → entry for O(1) lookup
	cfg     Config
	onMatch MatchCallback
}

// New constructs a Queue with the given config and match callback.
func New(cfg Config, onMatch MatchCallback) *Queue {
	return &Queue{
		indexed: make(map[string]*entry),
		cfg:     cfg,
		onMatch: onMatch,
	}
}

// Enqueue adds a player to the matchmaking queue.
// Returns ErrAlreadyQueued if the player is already waiting.
func (q *Queue) Enqueue(p *game.Player) error {
	return q.EnqueueAt(p, time.Now())
}

// EnqueueAt adds a player with a custom enqueue timestamp.
// Intended for testing the window-expansion logic without sleeping.
func (q *Queue) EnqueueAt(p *game.Player, at time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.indexed[p.ID]; ok {
		return ErrAlreadyQueued
	}
	e := &entry{player: p, enqueuedAt: at}
	q.entries = append(q.entries, e)
	q.indexed[p.ID] = e
	return nil
}

// Dequeue removes a player from the queue (e.g. they disconnected).
// Returns ErrNotQueued if the player is not currently waiting.
func (q *Queue) Dequeue(playerID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.indexed[playerID]; !ok {
		return ErrNotQueued
	}
	delete(q.indexed, playerID)
	for i, e := range q.entries {
		if e.player.ID == playerID {
			q.entries = append(q.entries[:i], q.entries[i+1:]...)
			return nil
		}
	}
	return ErrNotQueued
}

// Len returns the number of players currently in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.entries)
}

// Run starts the matchmaking loop. It blocks until ctx is cancelled.
// Call this in a dedicated goroutine.
func (q *Queue) Run(ctx context.Context) {
	ticker := time.NewTicker(q.cfg.TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			q.Tick(t)
		}
	}
}

type matchedPair struct {
	a, b *game.Player
}

// Tick runs one pass of the greedy matching algorithm at the given time.
// Exported so tests can call it directly with a synthetic timestamp instead
// of waiting for the real ticker.
func (q *Queue) Tick(now time.Time) {
	q.mu.Lock()
	n := len(q.entries)
	if n < 2 {
		q.mu.Unlock()
		return
	}

	used := make([]bool, n)
	var pairs []matchedPair

	for i := 0; i < n; i++ {
		if used[i] {
			continue
		}
		a := q.entries[i]
		winA := q.window(a, now)

		bestJ := -1
		bestDiff := math.MaxInt

		for j := i + 1; j < n; j++ {
			if used[j] {
				continue
			}
			b := q.entries[j]
			diff := abs(a.player.Exp - b.player.Exp)
			if diff > winA {
				continue
			}
			winB := q.window(b, now)
			effectiveWindow := min(winA, winB)
			if diff <= effectiveWindow && diff < bestDiff {
				bestJ = j
				bestDiff = diff
			}
		}

		if bestJ >= 0 {
			pairs = append(pairs, matchedPair{a.player, q.entries[bestJ].player})
			used[i] = true
			used[bestJ] = true
		}
	}

	// Rebuild entries without matched players.
	keep := q.entries[:0]
	for i, e := range q.entries {
		if !used[i] {
			keep = append(keep, e)
		} else {
			delete(q.indexed, e.player.ID)
		}
	}
	q.entries = keep

	q.mu.Unlock()

	// Call onMatch outside the lock so it can safely call Enqueue/Dequeue.
	for _, p := range pairs {
		if err := q.onMatch([]*game.Player{p.a, p.b}); err != nil {
			// Re-enqueue both players so they can be matched on the next tick.
			_ = q.Enqueue(p.a)
			_ = q.Enqueue(p.b)
		}
	}
}

// window computes the current acceptable Exp difference for entry e at time now.
func (q *Queue) window(e *entry, now time.Time) int {
	waited := max(now.Sub(e.enqueuedAt), 0)
	steps := int(waited / q.cfg.ExpandInterval)
	return q.cfg.InitialRange + steps*q.cfg.ExpandStep
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
