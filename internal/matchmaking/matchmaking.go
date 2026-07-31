// Package matchmaking implements a rating-based matchmaking queue.
// Players enter the queue and are paired with opponents of similar ELO rating.
// The longer a player waits, the wider the acceptable rating gap becomes.
package matchmaking

import (
	"context"
	"errors"
	"log/slog"
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
	// InitialRange is the starting acceptable ELO rating difference between players.
	InitialRange int
	// ExpandStep is the number of ELO rating points added per ExpandInterval of waiting.
	ExpandStep int
	// ExpandInterval is how long a player must wait before the range widens by ExpandStep.
	ExpandInterval time.Duration
	// TickInterval is how often the queue is scanned for matches.
	TickInterval time.Duration
	// MaxWait is how long a player may wait before being expired from the
	// queue (the server then falls back to a bot game for them).
	MaxWait time.Duration
}

// DefaultConfig returns production-sensible matchmaking defaults.
func DefaultConfig() Config {
	return Config{
		InitialRange:   100,
		ExpandStep:     50,
		ExpandInterval: 10 * time.Second,
		TickInterval:   2 * time.Second,
		MaxWait:        15 * time.Second,
	}
}

// MatchCallback is called (outside the queue lock) when two players are matched.
// If it returns an error both players are re-enqueued and matching is retried
// on the next tick.
type MatchCallback func(players []*game.Player) error

// ExpireCallback is called (outside the queue lock) for a player who waited
// longer than Config.MaxWait — the entry is removed first, so the callback
// owns the fallback (e.g. starting a bot game).
type ExpireCallback func(p *game.Player) error

type entry struct {
	player     *game.Player
	enqueuedAt time.Time
}

// Queue is the matchmaking queue. Safe for concurrent use.
type Queue struct {
	mu       sync.Mutex
	entries  []*entry          // insertion-ordered (FIFO within same rating bucket)
	indexed  map[string]*entry // playerID → entry for O(1) lookup
	cfg      Config
	onMatch  MatchCallback
	onExpire ExpireCallback
	isOnline func(playerID string) bool
}

// New constructs a Queue with the given config and match callback.
func New(cfg Config, onMatch MatchCallback) *Queue {
	return &Queue{
		indexed: make(map[string]*entry),
		cfg:     cfg,
		onMatch: onMatch,
	}
}

// WithExpireCallback sets the fallback invoked for entries that outwait MaxWait.
func (q *Queue) WithExpireCallback(cb ExpireCallback) *Queue {
	q.onExpire = cb
	return q
}

// WithOnlineChecker sets an optional presence check: entries whose player is
// offline at tick time are silently removed from the queue.
func (q *Queue) WithOnlineChecker(f func(playerID string) bool) *Queue {
	q.isOnline = f
	return q
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
	// Matchmaking is only for human players. Normalize an unset type so callers
	// do not need to know the player taxonomy.
	if p.Type == game.PlayerTypeUnknown {
		p.Type = game.PlayerTypeHuman
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

// Run starts the matchmaking loop. It blocks until ctx is canceled.
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

	// Drop expired entries (waited longer than MaxWait) and offline players.
	var expired []*game.Player
	if q.cfg.MaxWait > 0 || q.isOnline != nil {
		keep := q.entries[:0]
		for _, e := range q.entries {
			switch {
			case q.cfg.MaxWait > 0 && now.Sub(e.enqueuedAt) > q.cfg.MaxWait:
				expired = append(expired, e.player)
				delete(q.indexed, e.player.ID)
			case q.isOnline != nil && !q.isOnline(e.player.ID):
				delete(q.indexed, e.player.ID)
			default:
				keep = append(keep, e)
			}
		}
		q.entries = keep
	}

	n := len(q.entries)
	if n < 2 {
		q.mu.Unlock()
		q.fireExpire(expired)
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
			diff := abs(a.player.Rating - b.player.Rating)
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

	// Call callbacks outside the lock so they can safely call Enqueue/Dequeue.
	for _, p := range pairs {
		if err := q.onMatch([]*game.Player{p.a, p.b}); err != nil {
			// Re-enqueue both players so they can be matched on the next tick.
			_ = q.Enqueue(p.a)
			_ = q.Enqueue(p.b)
		}
	}
	q.fireExpire(expired)
}

// fireExpire invokes the expire callback for each expired player, if set.
func (q *Queue) fireExpire(players []*game.Player) {
	if q.onExpire == nil {
		return
	}
	for _, p := range players {
		// The player is already out of the queue; if the fallback fails there
		// is nothing to retry — the client times out on its own.
		if err := q.onExpire(p); err != nil {
			slog.Error("matchmaking: expire fallback failed", slog.String("playerID", p.ID), slog.Any("error", err))
		}
	}
}

// window computes the current acceptable ELO rating difference for entry e at time now.
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
