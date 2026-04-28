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
	ErrGameNotFound   = errors.New("lobby: game not found")
	ErrPlayerInGame   = errors.New("lobby: player already in a game")
	ErrGameNotWaiting = errors.New("lobby: game is not in waiting status")
	ErrGameFull       = errors.New("lobby: game already has enough players")
)

type GameStatus string

const (
	GameStatusWaiting    GameStatus = "waiting"
	GameStatusInProgress GameStatus = "in_progress"
	GameStatusFinished   GameStatus = "finished"
)

// GameRecord is the Lobby's view of a running game.
type GameRecord struct {
	ID        string
	Game      *game.Game
	Players   []*game.Player
	StartedAt time.Time
	Status    GameStatus
	cancel    context.CancelFunc
}

// PlayerInfo is a lightweight player descriptor carried in GameSummary.
type PlayerInfo struct {
	ID  string
	Exp int
}

// GameSummary is the read-only snapshot returned by List and FindByPlayer.
type GameSummary struct {
	ID        string
	Players   []PlayerInfo
	StartedAt time.Time
	Status    GameStatus
}

// GameFactory abstracts game construction so the Lobby is testable.
// gameID is the lobby-assigned UUID for the game being created.
type GameFactory func(ctx context.Context, gameID string, players []*game.Player, n game.Notifier) (*game.Game, error)

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

// Create registers a new game in waiting status for a single player (the creator).
// Other players join later via join mechanics.
// Returns ErrPlayerInGame if the player is already in an active game.
func (l *Lobby) Create(p *game.Player) (*GameRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.byPlayer[p.ID]; ok {
		return nil, ErrPlayerInGame
	}

	id := uuid.New().String()
	rec := &GameRecord{
		ID:        id,
		Players:   []*game.Player{p},
		StartedAt: time.Now(),
		Status:    GameStatusWaiting,
	}
	l.games[id] = rec
	l.byPlayer[p.ID] = id
	return rec, nil
}

// Join adds playerID to an existing waiting game. If the joining player brings
// the player count to 2 (quorum), the game is started immediately via the
// factory; the first move belongs to the player who created the game (index 0).
// Returns ErrGameNotFound, ErrGameNotWaiting, ErrGameFull, or ErrPlayerInGame
// on the corresponding error conditions.
func (l *Lobby) Join(ctx context.Context, gameID string, p *game.Player, n game.Notifier) (*GameRecord, error) {
	// Phase 1: read-check without creating anything.
	l.mu.RLock()
	rec, ok := l.games[gameID]
	if !ok {
		l.mu.RUnlock()
		return nil, ErrGameNotFound
	}
	if rec.Status != GameStatusWaiting {
		l.mu.RUnlock()
		if rec.Status == GameStatusInProgress {
			return nil, ErrGameFull
		}
		return nil, ErrGameNotWaiting
	}
	if _, alreadyIn := l.byPlayer[p.ID]; alreadyIn {
		l.mu.RUnlock()
		return nil, ErrPlayerInGame
	}
	// Snapshot creator list so factory can be called outside the lock.
	existing := make([]*game.Player, len(rec.Players))
	copy(existing, rec.Players)
	l.mu.RUnlock()

	// Build the full player list: creator(s) first, joiner last.
	allPlayers := append(existing, p)

	// Use a background context so the game goroutine outlives the HTTP request
	// that triggered the join. The lobby cancels it via rec.cancel when needed.
	gameCtx, cancel := context.WithCancel(context.Background())
	g, err := l.factory(gameCtx, gameID, allPlayers, n)
	if err != nil {
		cancel()
		return nil, err
	}

	// Phase 2: write-lock, re-validate, commit.
	l.mu.Lock()
	defer l.mu.Unlock()

	rec, ok = l.games[gameID]
	if !ok {
		cancel()
		return nil, ErrGameNotFound
	}
	if rec.Status != GameStatusWaiting {
		cancel()
		if rec.Status == GameStatusInProgress {
			return nil, ErrGameFull
		}
		return nil, ErrGameNotWaiting
	}
	if _, alreadyIn := l.byPlayer[p.ID]; alreadyIn {
		cancel()
		return nil, ErrPlayerInGame
	}

	// Commit: update record in-place and register the new player.
	rec.Players = append(rec.Players, p)
	rec.Game = g
	rec.Status = GameStatusInProgress
	rec.cancel = cancel
	l.byPlayer[p.ID] = gameID

	go func() {
		g.Run(gameCtx)
		l.onDone(gameID)
	}()

	return rec, nil
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

	id := uuid.New().String()

	gameCtx, cancel := context.WithCancel(ctx)
	g, err := l.factory(gameCtx, id, players, n)
	if err != nil {
		cancel()
		return nil, err
	}
	rec := &GameRecord{
		ID:        id,
		Game:      g,
		Players:   players,
		StartedAt: time.Now(),
		Status:    GameStatusInProgress,
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
	players := make([]PlayerInfo, len(rec.Players))
	for i, p := range rec.Players {
		players[i] = PlayerInfo{ID: p.ID, Exp: p.Exp}
	}
	return GameSummary{
		ID:        rec.ID,
		Players:   players,
		StartedAt: rec.StartedAt,
		Status:    rec.Status,
	}
}

/*
Пример интеграции в сервер

lby := lobby.New(func(ctx context.Context, gameID string, players []*game.Player, n game.Notifier) (*game.Game, error) {
    return game.NewGame(players, n)
})
queue := matchmaking.New(matchmaking.DefaultConfig(), func(players []*game.Player) error {
    _, err := lby.StartGame(serverCtx, players, myNotifier)
    return err
})
go queue.Run(serverCtx)
*/
