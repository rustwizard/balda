package storage

import (
	"context"
	"fmt"
	"time"
)

type FinishReason string

const (
	FinishReasonBoardFull FinishReason = "board_full"
	FinishReasonKick      FinishReason = "kick"
	FinishReasonAcceptEnd FinishReason = "accept_end"
)

type PlayerResult struct {
	PlayerID   string
	Score      int
	WordsCount int
	ExpGained  int
}

type GameResult struct {
	GameID       string
	WinnerID     string // empty = draw
	FinishReason FinishReason
	FinishedAt   time.Time
	Players      []PlayerResult
}

// ExpGained returns EXP delta for a player: win=10+score, draw=5+score, loss=score.
func ExpGained(score int, isWinner, isDraw bool) int {
	switch {
	case isWinner:
		return 10 + score
	case isDraw:
		return 5 + score
	default:
		return score
	}
}

// SaveGameResult writes the game result and updates each player's EXP atomically.
func (b *Balda) SaveGameResult(ctx context.Context, r GameResult) error {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("save game result: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var winnerID *string
	if r.WinnerID != "" {
		winnerID = &r.WinnerID
	}

	var resultID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO game_results (game_id, winner_id, finish_reason, finished_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		r.GameID, winnerID, string(r.FinishReason), r.FinishedAt,
	).Scan(&resultID)
	if err != nil {
		return fmt.Errorf("save game result: insert game_results: %w", err)
	}

	for _, p := range r.Players {
		_, err = tx.Exec(ctx,
			`INSERT INTO game_result_players (game_result_id, player_id, score, words_count, exp_gained)
			 VALUES ($1, $2, $3, $4, $5)`,
			resultID, p.PlayerID, p.Score, p.WordsCount, p.ExpGained,
		)
		if err != nil {
			return fmt.Errorf("save game result: insert game_result_players for %s: %w", p.PlayerID, err)
		}

		_, err = tx.Exec(ctx,
			`UPDATE player_state SET exp = exp + $1, updated_at = now() WHERE player_id = $2`,
			p.ExpGained, p.PlayerID,
		)
		if err != nil {
			return fmt.Errorf("save game result: update exp for %s: %w", p.PlayerID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("save game result: commit: %w", err)
	}
	return nil
}
