package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	leaderboardSortRating = "rating"
	leaderboardSortExp    = "exp"
)

// LeaderboardEntry is one row in a leaderboard.
type LeaderboardEntry struct {
	PlayerID  uuid.UUID
	Nickname  string
	Rating    int64
	Exp       int64
	UpdatedAt time.Time
}

// GetLeaderboard returns the top players active since periodStart, ordered by sortBy.
// sortBy must be validated by the caller; only "rating" or "exp" are accepted.
func (b *Balda) GetLeaderboard(ctx context.Context, periodStart time.Time, sortBy string, limit int) ([]LeaderboardEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	orderCol := leaderboardSortRating
	tieCol := leaderboardSortExp
	if sortBy == leaderboardSortExp {
		orderCol = leaderboardSortExp
		tieCol = leaderboardSortRating
	}

	query := fmt.Sprintf(`
		SELECT player_id, nickname, rating, exp, updated_at
		FROM player_state
		WHERE updated_at >= $1
		ORDER BY %s DESC, %s DESC, player_id ASC
		LIMIT $2
	`, orderCol, tieCol)

	rows, err := b.db.Query(ctx, query, periodStart, limit)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}
	defer rows.Close()

	var out []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.PlayerID, &e.Nickname, &e.Rating, &e.Exp, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("get leaderboard scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get leaderboard rows: %w", err)
	}
	return out, nil
}
