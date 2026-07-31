package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// DefaultRating is the starting ELO rating for new players.
const DefaultRating = 1000

// PlayerState holds the profile fields stored in player_state.
type PlayerState struct {
	Nickname        string
	Exp             int64
	Rating          int64
	Flags           int64
	Lives           int64
	TotalGames      int64
	ConsecutiveWins int64
}

// PlayerForGame holds the minimum player data needed to participate in a game.
type PlayerForGame struct {
	PlayerID uuid.UUID
	Exp      int
	Rating   int
}

// GetPlayerState returns profile fields for the given player UUID.
func (b *Balda) GetPlayerState(ctx context.Context, playerID uuid.UUID) (PlayerState, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var ps PlayerState
	err := b.db.QueryRow(ctx,
		`SELECT nickname, COALESCE(exp, 0), COALESCE(rating, $1), COALESCE(flags, 0), COALESCE(lives, 0),
		        COALESCE(total_games, 0), COALESCE(consecutive_wins, 0)
		 FROM player_state WHERE player_id = $2`,
		DefaultRating, playerID,
	).Scan(&ps.Nickname, &ps.Exp, &ps.Rating, &ps.Flags, &ps.Lives, &ps.TotalGames, &ps.ConsecutiveWins)
	if err != nil {
		return PlayerState{}, fmt.Errorf("get player state: %w", err)
	}
	return ps, nil
}

// GetPlayerByUID returns the player_id and exp for the given internal user ID.
func (b *Balda) GetPlayerByUID(ctx context.Context, uid int64) (PlayerForGame, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var p PlayerForGame
	err := b.db.QueryRow(ctx,
		`SELECT player_id, COALESCE(exp, 0), COALESCE(rating, $1)
		 FROM player_state WHERE user_id = $2`,
		DefaultRating, uid,
	).Scan(&p.PlayerID, &p.Exp, &p.Rating)
	if err != nil {
		return PlayerForGame{}, fmt.Errorf("get player by uid: %w", err)
	}
	return p, nil
}

// GetUIDByPlayerID returns the internal user ID for the given player UUID.
// Needed to mint per-user Centrifugo tokens for matchmaking callbacks.
func (b *Balda) GetUIDByPlayerID(ctx context.Context, playerID uuid.UUID) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var uid int64
	err := b.db.QueryRow(ctx,
		`SELECT user_id FROM player_state WHERE player_id = $1`,
		playerID,
	).Scan(&uid)
	if err != nil {
		return 0, fmt.Errorf("get uid by player id: %w", err)
	}
	return uid, nil
}
