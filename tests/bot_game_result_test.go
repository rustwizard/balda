package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/game/bot"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSaveGameResult_BotGame verifies that a game against the bot is
// persisted for the human only: ELO/EXP/counters for the human, no rows
// for the bot (which has no player_state by design).
func TestSaveGameResult_BotGame(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	human := seedPlayerWithRating(ctx, t, s, "botgame.human@example.org", 1000)
	gameID := uuid.New()

	require.NoError(t, s.SaveGameResult(ctx, storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     human.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now().UTC(),
		Players: []storage.PlayerResult{
			{PlayerID: human.String(), Score: 12, WordsCount: 3, ExpGained: 22, Words: []string{"кот", "дом", "мир"}},
			{PlayerID: bot.BotPlayerID, Score: 5, WordsCount: 1, ExpGained: 5, Bot: true},
		},
	}))

	// game_results row exists, winner is the human.
	var winnerID string
	require.NoError(t, s.Pool().QueryRow(ctx,
		`SELECT winner_id FROM game_results WHERE game_id = $1`, gameID,
	).Scan(&winnerID))
	assert.Equal(t, human.String(), winnerID)

	// Only the human's row was written to game_result_players.
	var rows int
	require.NoError(t, s.Pool().QueryRow(ctx,
		`SELECT count(*) FROM game_result_players p
		 JOIN game_results r ON r.id = p.game_result_id
		 WHERE r.game_id = $1`, gameID,
	).Scan(&rows))
	assert.Equal(t, 1, rows, "bot must not be persisted")

	// Human got EXP and ELO vs the default bot rating (1000): win vs equal
	// rating is +16.
	var exp, rating, totalGames int64
	require.NoError(t, s.Pool().QueryRow(ctx,
		`SELECT exp, rating, total_games FROM player_state WHERE player_id = $1`, human,
	).Scan(&exp, &rating, &totalGames))
	assert.Equal(t, int64(22), exp)
	assert.Equal(t, int64(1016), rating)
	assert.Equal(t, int64(1), totalGames)

	// The human's words were stored, so stats aggregation sees the game.
	stats, err := s.GetPlayerStats(ctx, human)
	require.NoError(t, err)
	assert.Equal(t, int64(1), stats.GamesPlayed)
	assert.Equal(t, int64(1), stats.Wins)
	assert.Equal(t, "кот", stats.BestWord) // all words are 3 letters, first wins the tie
}
