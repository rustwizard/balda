package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/achievements"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/game/bot"
	"github.com/rustwizard/balda/internal/leaderboard"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/rustwizard/balda/internal/storage"
)

// ErrNotParticipant is returned by game actions when the caller is authenticated
// but is not a participant of the target game. Handlers map it to 403 Forbidden.
var ErrNotParticipant = errors.New("service: player is not a participant of this game")

type Balda struct {
	lby *lobby.Lobby
	mm  *matchmaking.Queue
	s   *storage.Balda
	lb  *leaderboard.Service
	ach *achievements.Service
	// botFallback starts a bot game for a player who found no human opponent.
	botFallback func(p *game.Player) error
}

func New(lby *lobby.Lobby, mm *matchmaking.Queue, s *storage.Balda, lb *leaderboard.Service, ach *achievements.Service) *Balda {
	return &Balda{lby: lby, mm: mm, s: s, lb: lb, ach: ach}
}

// WithBotFallback sets the function used to start a bot game when no human
// opponent is available. It is invoked synchronously by QuickMatchJoin when
// the matchmaking queue is empty.
func (s *Balda) WithBotFallback(f func(p *game.Player) error) *Balda {
	s.botFallback = f
	return s
}

func (s *Balda) GameSummary(playerID string) *lobby.GameSummary {
	gs, err := s.lby.FindByPlayer(playerID)
	if errors.Is(err, lobby.ErrGameNotFound) {
		return nil
	}
	return &gs
}

// ActiveGameRecord returns the full game record for the given player, or nil if not in a game.
func (s *Balda) ActiveGameRecord(playerID string) *lobby.GameRecord {
	gs, err := s.lby.FindByPlayer(playerID)
	if err != nil {
		return nil
	}
	rec, err := s.lby.Get(gs.ID)
	if err != nil {
		return nil
	}
	return rec
}

func (s *Balda) ListGames() []lobby.GameSummary {
	return s.lby.List()
}

func (s *Balda) Lobby() *lobby.Lobby {
	return s.lby
}

// AuthUser verifies credentials and returns the user's identity.
func (s *Balda) AuthUser(ctx context.Context, email, password string) (storage.UserAuth, error) {
	return s.s.AuthUser(ctx, email, password)
}

// CreateUser registers a new user with their player profile in one transaction.
// CreateUser creates a new email/password user with their player state.
func (s *Balda) CreateUser(ctx context.Context, p storage.CreateUserParams) (storage.UserCreated, error) {
	return s.s.CreateUser(ctx, p)
}

// GetPlayerState returns the profile fields for the given player UUID.
func (s *Balda) GetPlayerState(ctx context.Context, playerID uuid.UUID) (storage.PlayerState, error) {
	return s.s.GetPlayerState(ctx, playerID)
}

// GetLeaderboard returns the leaderboard for the requested period and sort order.
func (s *Balda) GetLeaderboard(ctx context.Context, req leaderboard.Request) ([]leaderboard.Entry, time.Time, error) {
	return s.lb.GetLeaderboard(ctx, req)
}

// GetPlayerAchievements returns the full list of achievements for the player.
func (s *Balda) GetPlayerAchievements(ctx context.Context, playerID uuid.UUID) ([]achievements.Achievement, error) {
	ps, err := s.s.GetPlayerState(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player achievements: %w", err)
	}
	return s.ach.List(ps.Flags), nil
}

// GetPlayerStats returns lifetime aggregated statistics for the player.
func (s *Balda) GetPlayerStats(ctx context.Context, playerID uuid.UUID) (storage.PlayerStats, error) {
	return s.s.GetPlayerStats(ctx, playerID)
}

// GetUserForToken returns the player UUID and role needed to mint an access token.
func (s *Balda) GetUserForToken(ctx context.Context, uid int64) (storage.UserForToken, error) {
	return s.s.GetUserForToken(ctx, uid)
}

// GetUserByTelegramID returns the user's identity for the given Telegram user ID.
func (s *Balda) GetUserByTelegramID(ctx context.Context, telegramID int64) (storage.UserAuth, error) {
	return s.s.GetUserByTelegramID(ctx, telegramID)
}

// CreateTelegramUser creates a new user registered via Telegram Mini App auth.
func (s *Balda) CreateTelegramUser(ctx context.Context, p storage.CreateTelegramUserParams) (storage.UserCreated, error) {
	return s.s.CreateTelegramUser(ctx, p)
}

// SaveRefreshToken persists the HMAC hash of a refresh token for the user.
func (s *Balda) SaveRefreshToken(ctx context.Context, rt storage.RefreshToken) error {
	return s.s.SaveRefreshToken(ctx, rt)
}

// GetRefreshToken fetches a refresh token row by its hash.
func (s *Balda) GetRefreshToken(ctx context.Context, tokenHash string) (storage.RefreshToken, error) {
	return s.s.GetRefreshToken(ctx, tokenHash)
}

// RevokeRefreshToken marks a single refresh token as revoked.
func (s *Balda) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return s.s.RevokeRefreshToken(ctx, tokenHash)
}

// RevokeAllUserTokens marks all of a user's refresh tokens as revoked.
func (s *Balda) RevokeAllUserTokens(ctx context.Context, uid int64) error {
	return s.s.RevokeAllUserTokens(ctx, uid)
}

// ErrPlayerInGame is returned when a player tries to join matchmaking or
// create a game while already participating in one.
var ErrPlayerInGame = errors.New("service: player already in a game")

// QuickMatchJoin enqueues the user for quick matchmaking. When nobody else is
// waiting, a bot game is started immediately via the bot fallback instead of
// queuing. Returns ErrPlayerInGame if they are already in a game, or
// matchmaking.ErrAlreadyQueued if they are already waiting.
func (s *Balda) QuickMatchJoin(ctx context.Context, uid int64) error {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("quick match join: %w", err)
	}
	if rec := s.ActiveGameRecord(p.PlayerID.String()); rec != nil && rec.Status != lobby.GameStatusFinished {
		return ErrPlayerInGame
	}
	player := &game.Player{ID: p.PlayerID.String(), Exp: p.Exp, Rating: p.Rating, Type: game.PlayerTypeHuman}
	if s.mm.Len() == 0 && s.botFallback != nil {
		return s.botFallback(player)
	}
	return s.mm.Enqueue(player)
}

// QuickMatchLeave removes the user from the matchmaking queue. It is
// idempotent: leaving without being queued is not an error.
func (s *Balda) QuickMatchLeave(ctx context.Context, uid int64) error {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("quick match leave: %w", err)
	}
	_ = s.mm.Dequeue(p.PlayerID.String())
	return nil
}

// LeaveGame removes the user from a waiting game, deleting the game
// entirely when it becomes empty.
func (s *Balda) LeaveGame(ctx context.Context, uid int64, gameID string) error {
	pid, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	return s.lby.Leave(gameID, pid)
}

// CreateGame creates a new game in waiting status for the given user.
func (s *Balda) CreateGame(ctx context.Context, uid int64) (*lobby.GameRecord, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	_ = s.mm.Dequeue(p.PlayerID.String())
	return s.lby.Create(&game.Player{ID: p.PlayerID.String(), Exp: p.Exp, Rating: p.Rating, Type: game.PlayerTypeHuman})
}

// JoinGame adds the user identified by uid to the waiting game with the given gameID.
func (s *Balda) JoinGame(ctx context.Context, uid int64, gameID string) (*lobby.GameRecord, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("join game: %w", err)
	}
	_ = s.mm.Dequeue(p.PlayerID.String())
	return s.lby.Join(ctx, gameID, &game.Player{ID: p.PlayerID.String(), Exp: p.Exp, Rating: p.Rating, Type: game.PlayerTypeHuman}, &game.NoopNotifier{})
}

// CreateGameWithBot creates and immediately starts a game between the user and a bot.
func (s *Balda) CreateGameWithBot(ctx context.Context, uid int64) (*lobby.GameRecord, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("create game with bot: %w", err)
	}
	_ = s.mm.Dequeue(p.PlayerID.String())
	human := &game.Player{ID: p.PlayerID.String(), Exp: p.Exp, Rating: p.Rating, Type: game.PlayerTypeHuman}
	botPlayer := &game.Player{
		ID:     bot.BotPlayerID,
		Exp:    0,
		Rating: storage.DefaultRating,
		Type:   game.PlayerTypeBot,
	}
	// Use a background context so the bot game outlives the HTTP request.
	return s.lby.StartGame(context.Background(), []*game.Player{human, botPlayer}, &game.NoopNotifier{})
}

func (s *Balda) playerIDByUID(ctx context.Context, uid int64) (string, error) {
	p, err := s.s.GetPlayerByUID(ctx, uid)
	if err != nil {
		return "", fmt.Errorf("fetch player: %w", err)
	}
	return p.PlayerID.String(), nil
}

func (s *Balda) isPlayerInGame(rec *lobby.GameRecord, playerID string) bool {
	for _, p := range rec.Players {
		if p.ID == playerID {
			return true
		}
	}
	return false
}

// SubmitMove validates and applies a player's move.
func (s *Balda) SubmitMove(ctx context.Context, uid int64, gameID string, newLetter game.Letter, wordPath []game.Letter) (*lobby.GameRecord, string, error) {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return nil, "", err
	}

	rec, err := s.lby.Get(gameID)
	if err != nil {
		return nil, "", err
	}

	if !s.isPlayerInGame(rec, playerID) {
		return nil, "", ErrNotParticipant
	}

	// Resolve characters for the word path from the current board state.
	board := rec.Game.BoardSnapshot()
	for i := range wordPath {
		if wordPath[i].RowID == newLetter.RowID && wordPath[i].ColID == newLetter.ColID {
			wordPath[i].Char = newLetter.Char
		} else {
			wordPath[i].Char = board[wordPath[i].RowID][wordPath[i].ColID]
		}
	}

	if err := rec.Game.SubmitWord(playerID, &newLetter, wordPath); err != nil {
		return nil, "", err
	}

	return rec, playerID, nil
}

// ProposeEnd proposes to end the game. Only the current player may call this.
func (s *Balda) ProposeEnd(ctx context.Context, uid int64, gameID string) error {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	rec, err := s.lby.Get(gameID)
	if err != nil {
		return err
	}
	if !s.isPlayerInGame(rec, playerID) {
		return ErrNotParticipant
	}
	return rec.Game.ProposeEnd(playerID)
}

// AcceptEnd accepts the opponent's end-game proposal.
func (s *Balda) AcceptEnd(ctx context.Context, uid int64, gameID string) error {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	rec, err := s.lby.Get(gameID)
	if err != nil {
		return err
	}
	if !s.isPlayerInGame(rec, playerID) {
		return ErrNotParticipant
	}
	return rec.Game.AcceptEnd(playerID)
}

// RejectEnd rejects the opponent's end-game proposal.
func (s *Balda) RejectEnd(ctx context.Context, uid int64, gameID string) error {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return err
	}
	rec, err := s.lby.Get(gameID)
	if err != nil {
		return err
	}
	if !s.isPlayerInGame(rec, playerID) {
		return ErrNotParticipant
	}
	return rec.Game.RejectEnd(playerID)
}

// SkipTurn ends the current turn without a move.
func (s *Balda) SkipTurn(ctx context.Context, uid int64, gameID string) (*lobby.GameRecord, string, error) {
	playerID, err := s.playerIDByUID(ctx, uid)
	if err != nil {
		return nil, "", err
	}

	rec, err := s.lby.Get(gameID)
	if err != nil {
		return nil, "", err
	}

	if !s.isPlayerInGame(rec, playerID) {
		return nil, "", ErrNotParticipant
	}

	if err := rec.Game.Skip(playerID); err != nil {
		return nil, "", err
	}

	return rec, playerID, nil
}
