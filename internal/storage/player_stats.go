package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// PlayerStats holds aggregated lifetime statistics for a player.
type PlayerStats struct {
	GamesPlayed    int64
	Wins           int64
	Losses         int64
	Draws          int64
	WinRate        float64 // 0..1
	AvgWordLength  float64
	BestWord       string
	FavoriteLetter string
}

// GetPlayerStats aggregates lifetime statistics for the given player from
// game_results/game_result_players. Average word length is derived from
// score (= total letters of all words) over words_count, so it also covers
// games saved before words were persisted. Best word and favorite letter
// only consider games where words were recorded.
func (b *Balda) GetPlayerStats(ctx context.Context, playerID uuid.UUID) (PlayerStats, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	rows, err := b.db.Query(ctx,
		`SELECT p.score, p.words_count, p.words, r.winner_id
		 FROM game_result_players p
		 JOIN game_results r ON r.id = p.game_result_id
		 WHERE p.player_id = $1`,
		playerID,
	)
	if err != nil {
		return PlayerStats{}, fmt.Errorf("get player stats: %w", err)
	}
	defer rows.Close()

	var stats PlayerStats
	var totalScore, totalWords int64
	var allWords []string

	for rows.Next() {
		var score, wordsCount int64
		var wordsJSON []byte
		var winnerID *uuid.UUID
		if err := rows.Scan(&score, &wordsCount, &wordsJSON, &winnerID); err != nil {
			return PlayerStats{}, fmt.Errorf("get player stats scan: %w", err)
		}

		stats.GamesPlayed++
		totalScore += score
		totalWords += wordsCount

		switch {
		case winnerID == nil:
			stats.Draws++
		case *winnerID == playerID:
			stats.Wins++
		default:
			stats.Losses++
		}

		if len(wordsJSON) > 0 {
			var words []string
			if err := json.Unmarshal(wordsJSON, &words); err != nil {
				return PlayerStats{}, fmt.Errorf("get player stats: decode words: %w", err)
			}
			allWords = append(allWords, words...)
		}
	}
	if err := rows.Err(); err != nil {
		return PlayerStats{}, fmt.Errorf("get player stats rows: %w", err)
	}

	if stats.GamesPlayed > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed)
	}
	if totalWords > 0 {
		stats.AvgWordLength = float64(totalScore) / float64(totalWords)
	}
	stats.BestWord = bestWord(allWords)
	stats.FavoriteLetter = favoriteLetter(allWords)

	return stats, nil
}

// bestWord returns the longest word by rune count; ties keep the first one.
func bestWord(words []string) string {
	var best string
	var bestLen int
	for _, w := range words {
		if l := utf8.RuneCountInString(w); l > bestLen {
			best, bestLen = w, l
		}
	}
	return best
}

// favoriteLetter returns the most frequent Cyrillic letter across all words.
// Letters are lowercased and ё is folded into е, matching the game rules.
// Ties keep the letter that reached its maximum first.
func favoriteLetter(words []string) string {
	counts := make(map[rune]int)
	var best rune
	var bestCount int
	for _, w := range words {
		for _, r := range strings.ToLower(w) {
			if r == 'ё' {
				r = 'е'
			}
			counts[r]++
			if counts[r] > bestCount {
				best, bestCount = r, counts[r]
			}
		}
	}
	if best == 0 {
		return ""
	}
	return string(best)
}
