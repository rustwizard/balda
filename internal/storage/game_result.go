package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/rustwizard/balda/internal/achievements"
)

// KFactor is the ELO sensitivity constant used for rating updates.
const KFactor = 32

type FinishReason string

const (
	FinishReasonGameFinished FinishReason = "game_finished"
	FinishReasonBoardFull    FinishReason = "board_full"
	FinishReasonKick         FinishReason = "kick"
	FinishReasonAcceptEnd    FinishReason = "accept_end"
)

type PlayerResult struct {
	PlayerID       string
	Score          int
	WordsCount     int
	ExpGained      int
	BestWordLength int
	Words          []string
}

type GameResult struct {
	GameID       string
	WinnerID     string // empty = draw
	FinishReason FinishReason
	FinishedAt   time.Time
	Players      []PlayerResult
}

// PlayerAchievementUnlock groups newly unlocked achievements for a player.
type PlayerAchievementUnlock struct {
	PlayerID string
	Unlocked []achievements.Achievement
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

// EloDelta returns the rating change for a player with the given current rating
// against an opponent with opponentRating. Score is 1 for a win, 0.5 for a draw,
// 0 for a loss. KFactor is used as the maximum rating swing per game.
func EloDelta(rating, opponentRating int, score float64) int {
	expected := 1.0 / (1.0 + math.Pow(10.0, float64(opponentRating-rating)/400.0))
	return int(math.Round(KFactor * (score - expected)))
}

// SaveGameResult writes the game result and updates each player's EXP atomically.
// It is a convenience wrapper that discards unlocked achievements.
func (b *Balda) SaveGameResult(ctx context.Context, r GameResult) error {
	_, err := b.SaveGameResultWithAchievements(ctx, r, nil)
	return err
}

// SaveGameResultWithAchievements writes the game result, updates player counters
// and flags, and returns any achievements unlocked during this game.
// If achSvc is nil, achievements are not evaluated but counters are still updated.
func (b *Balda) SaveGameResultWithAchievements(ctx context.Context, r GameResult, achSvc *achievements.Service) ([]PlayerAchievementUnlock, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("save game result: begin tx: %w", err)
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
		return nil, fmt.Errorf("save game result: insert game_results: %w", err)
	}

	// Load current ratings to compute ELO updates. The game is expected to have
	// exactly two players; for any other count we leave ratings unchanged.
	ratingDeltas := make(map[string]int, len(r.Players))
	if len(r.Players) == 2 {
		p0, p1 := r.Players[0], r.Players[1]
		var ratings [2]int
		for i, pid := range []string{p0.PlayerID, p1.PlayerID} {
			var rating int
			err := tx.QueryRow(ctx,
				`SELECT COALESCE(rating, $1) FROM player_state WHERE player_id = $2`,
				DefaultRating, pid,
			).Scan(&rating)
			if err != nil {
				return nil, fmt.Errorf("save game result: load rating for %s: %w", pid, err)
			}
			ratings[i] = rating
		}

		var score0, score1 float64
		switch r.WinnerID {
		case "":
			score0, score1 = 0.5, 0.5
		case p0.PlayerID:
			score0, score1 = 1.0, 0.0
		default:
			score0, score1 = 0.0, 1.0
		}

		ratingDeltas[p0.PlayerID] = EloDelta(ratings[0], ratings[1], score0)
		ratingDeltas[p1.PlayerID] = EloDelta(ratings[1], ratings[0], score1)
	}

	var unlocks []PlayerAchievementUnlock

	for _, p := range r.Players {
		wordsJSON, err := json.Marshal(p.Words)
		if err != nil {
			return nil, fmt.Errorf("save game result: marshal words for %s: %w", p.PlayerID, err)
		}
		if p.Words == nil {
			wordsJSON = []byte("[]")
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO game_result_players (game_result_id, player_id, score, words_count, exp_gained, best_word_length, words)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			resultID, p.PlayerID, p.Score, p.WordsCount, p.ExpGained, p.BestWordLength, wordsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("save game result: insert game_result_players for %s: %w", p.PlayerID, err)
		}

		isWinner := r.WinnerID != "" && r.WinnerID == p.PlayerID

		var oldTotal, oldStreak int
		var oldFlags int64
		err = tx.QueryRow(ctx,
			`SELECT total_games, consecutive_wins, flags
			 FROM player_state
			 WHERE player_id = $1
			 FOR UPDATE`,
			p.PlayerID,
		).Scan(&oldTotal, &oldStreak, &oldFlags)
		if err != nil {
			return nil, fmt.Errorf("save game result: load counters for %s: %w", p.PlayerID, err)
		}

		newTotal := oldTotal + 1
		newStreak := 0
		if isWinner {
			newStreak = oldStreak + 1
		}

		newBits := oldFlags
		var newlyUnlocked []achievements.Achievement
		if achSvc != nil {
			newBits, newlyUnlocked = achSvc.Calculate(oldFlags, achievements.PlayerGameStats{
				TotalGames:      newTotal,
				ConsecutiveWins: newStreak,
				Score:           p.Score,
				WordsCount:      p.WordsCount,
				BestWordLength:  p.BestWordLength,
				IsWinner:        isWinner,
			})
		}

		_, err = tx.Exec(ctx,
			`UPDATE player_state
			 SET exp = exp + $1,
			     rating = rating + $2,
			     total_games = $3,
			     consecutive_wins = $4,
			     flags = flags | $5,
			     updated_at = now()
			 WHERE player_id = $6`,
			p.ExpGained, ratingDeltas[p.PlayerID], newTotal, newStreak, newBits, p.PlayerID,
		)
		if err != nil {
			return nil, fmt.Errorf("save game result: update player_state for %s: %w", p.PlayerID, err)
		}

		if len(newlyUnlocked) > 0 {
			unlocks = append(unlocks, PlayerAchievementUnlock{
				PlayerID: p.PlayerID,
				Unlocked: newlyUnlocked,
			})
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("save game result: commit: %w", err)
	}
	return unlocks, nil
}
