// Package lobby tracks all currently active games.
package lobby

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/game"
)

var (
	ErrGameNotFound = errors.New("lobby: game not found")
	ErrPlayerInGame = errors.New("lobby: player already in a game")
)

// GameRecord is the Lobby's view of a running game.
type GameRecord struct {
	ID        string
	Game      *game.Game
	Players   []*game.Player
	StartedAt time.Time
	cancel    context.CancelFunc
}

// GameSummary is the read-only snapshot returned by List and FindByPlayer.
type GameSummary struct {
	ID        string
	PlayerIDs []string
	StartedAt time.Time
}

// GameFactory abstracts game construction so the Lobby is testable.
type GameFactory func(ctx context.Context, players []*game.Player, n game.Notifier) (*game.Game, error)

// Lobby tracks all currently active games. Safe for concurrent use.
type Lobby struct {
	mu       sync.RWMutex
	games    map[string]*GameRecord // gameID → record
	byPlayer map[string]string      // playerID → gameID
	factory  GameFactory
}

// New creates an empty Lobby that uses factory to construct new games.
func New(factory GameFactory) *Lobby {
	return &Lobby{
		games:    make(map[string]*GameRecord),
		byPlayer: make(map[string]string),
		factory:  factory,
	}
}

// StartGame creates a new game for players, registers it in the lobby, and
// launches game.Run in a background goroutine. The game is automatically
// removed from the lobby when Run returns.
// Returns ErrPlayerInGame if any player is already in an active game.
func (l *Lobby) StartGame(ctx context.Context, players []*game.Player, n game.Notifier) (*GameRecord, error) {
	l.mu.Lock()
	for _, p := range players {
		if _, ok := l.byPlayer[p.ID]; ok {
			l.mu.Unlock()
			return nil, ErrPlayerInGame
		}
	}
	l.mu.Unlock()

	gameCtx, cancel := context.WithCancel(ctx)
	g, err := l.factory(gameCtx, players, n)
	if err != nil {
		cancel()
		return nil, err
	}

	id := uuid.New().String()
	rec := &GameRecord{
		ID:        id,
		Game:      g,
		Players:   players,
		StartedAt: time.Now(),
		cancel:    cancel,
	}

	l.mu.Lock()
	// Re-check under write lock in case of a concurrent StartGame.
	for _, p := range players {
		if _, ok := l.byPlayer[p.ID]; ok {
			l.mu.Unlock()
			cancel()
			return nil, ErrPlayerInGame
		}
	}
	l.games[id] = rec
	for _, p := range players {
		l.byPlayer[p.ID] = id
	}
	l.mu.Unlock()

	go func() {
		g.Run(gameCtx)
		l.onDone(id)
	}()

	return rec, nil
}

// Remove stops and deregisters a game by its ID.
// Returns ErrGameNotFound if the ID is unknown.
func (l *Lobby) Remove(id string) error {
	l.mu.Lock()
	rec, ok := l.games[id]
	if !ok {
		l.mu.Unlock()
		return ErrGameNotFound
	}
	l.removeRecordLocked(rec)
	l.mu.Unlock()

	rec.cancel()
	return nil
}

// Get returns the GameRecord for the given game ID.
// Returns ErrGameNotFound if the game does not exist.
func (l *Lobby) Get(id string) (*GameRecord, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	rec, ok := l.games[id]
	if !ok {
		return nil, ErrGameNotFound
	}
	return rec, nil
}

// FindByPlayer returns the GameSummary for the game the given player is in.
// Returns ErrGameNotFound if the player is not in any active game.
func (l *Lobby) FindByPlayer(playerID string) (GameSummary, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	gid, ok := l.byPlayer[playerID]
	if !ok {
		return GameSummary{}, ErrGameNotFound
	}
	return summaryOf(l.games[gid]), nil
}

// List returns a snapshot of all active games as GameSummary values.
func (l *Lobby) List() []GameSummary {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]GameSummary, 0, len(l.games))
	for _, rec := range l.games {
		out = append(out, summaryOf(rec))
	}
	return out
}

// onDone is called from the game's goroutine after Run returns.
func (l *Lobby) onDone(id string) {
	l.mu.Lock()
	rec, ok := l.games[id]
	if ok {
		l.removeRecordLocked(rec)
	}
	l.mu.Unlock()
}

// removeRecordLocked removes rec from both maps. Caller must hold l.mu (write).
func (l *Lobby) removeRecordLocked(rec *GameRecord) {
	delete(l.games, rec.ID)
	for _, p := range rec.Players {
		delete(l.byPlayer, p.ID)
	}
}

func summaryOf(rec *GameRecord) GameSummary {
	ids := make([]string, len(rec.Players))
	for i, p := range rec.Players {
		ids[i] = p.ID
	}
	return GameSummary{
		ID:        rec.ID,
		PlayerIDs: ids,
		StartedAt: rec.StartedAt,
	}
}
