package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// PlayerState holds the profile fields stored in player_state.
type PlayerState struct {
	Nickname string
	Exp      int64
	Flags    int64
	Lives    int64
}

// PlayerForGame holds the minimum player data needed to participate in a game.
type PlayerForGame struct {
	PlayerID uuid.UUID
	Exp      int
}

// GetPlayerState returns profile fields for the given player UUID.
func (b *Balda) GetPlayerState(ctx context.Context, playerID uuid.UUID) (PlayerState, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var ps PlayerState
	err := b.db.QueryRow(ctx,
		`SELECT nickname, COALESCE(exp, 0), COALESCE(flags, 0), COALESCE(lives, 0)
		 FROM player_state WHERE player_id = $1`,
		playerID,
	).Scan(&ps.Nickname, &ps.Exp, &ps.Flags, &ps.Lives)
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
		`SELECT player_id, COALESCE(exp, 0) FROM player_state WHERE user_id = $1`, uid,
	).Scan(&p.PlayerID, &p.Exp)
	if err != nil {
		return PlayerForGame{}, fmt.Errorf("get player by uid: %w", err)
	}
	return p, nil
}
