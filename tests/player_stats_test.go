package tests

import (
	"context"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlayerStats(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayer(ctx, t, s, "stats.hero@example.org")
	p2 := seedPlayer(ctx, t, s, "stats.rival@example.org")

	// Game 1: p1 wins, submits "кот" and "дом".
	require.NoError(t, s.SaveGameResult(ctx, storage.GameResult{
		GameID:       "11111111-1111-1111-1111-111111111111",
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now().UTC(),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 6, WordsCount: 2, ExpGained: 16, Words: []string{"кот", "дом"}},
			{PlayerID: p2.String(), Score: 3, WordsCount: 1, ExpGained: 3, Words: []string{"мир"}},
		},
	}))

	// Game 2: draw, p1 submits "автостоп".
	require.NoError(t, s.SaveGameResult(ctx, storage.GameResult{
		GameID:       "22222222-2222-2222-2222-222222222222",
		WinnerID:     "",
		FinishReason: storage.FinishReasonAcceptEnd,
		FinishedAt:   time.Now().UTC(),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 8, WordsCount: 1, ExpGained: 13, Words: []string{"автостоп"}},
			{PlayerID: p2.String(), Score: 8, WordsCount: 1, ExpGained: 13, Words: []string{"провода"}},
		},
	}))

	stats, err := s.GetPlayerStats(ctx, p1)
	require.NoError(t, err)

	assert.Equal(t, int64(2), stats.GamesPlayed)
	assert.Equal(t, int64(1), stats.Wins)
	assert.Equal(t, int64(0), stats.Losses)
	assert.Equal(t, int64(1), stats.Draws)
	assert.InDelta(t, 0.5, stats.WinRate, 1e-9)
	// total score 6+8=14 over 3 words.
	assert.InDelta(t, 14.0/3.0, stats.AvgWordLength, 1e-9)
	assert.Equal(t, "автостоп", stats.BestWord)
	assert.Equal(t, "о", stats.FavoriteLetter)

	// The rival's perspective: one loss, one draw.
	rival, err := s.GetPlayerStats(ctx, p2)
	require.NoError(t, err)
	assert.Equal(t, int64(2), rival.GamesPlayed)
	assert.Equal(t, int64(0), rival.Wins)
	assert.Equal(t, int64(1), rival.Losses)
	assert.Equal(t, int64(1), rival.Draws)
	assert.Equal(t, "провода", rival.BestWord)
}

func TestGetPlayerStats_NoGames(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p := seedPlayer(ctx, t, s, "stats.newbie@example.org")

	stats, err := s.GetPlayerStats(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.GamesPlayed)
	assert.Equal(t, 0.0, stats.WinRate)
	assert.Equal(t, 0.0, stats.AvgWordLength)
	assert.Equal(t, "", stats.BestWord)
	assert.Equal(t, "", stats.FavoriteLetter)
}

func TestSaveGameResult_PersistsWords(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayer(ctx, t, s, "stats.words1@example.org")
	p2 := seedPlayer(ctx, t, s, "stats.words2@example.org")

	require.NoError(t, s.SaveGameResult(ctx, storage.GameResult{
		GameID:       "33333333-3333-3333-3333-333333333333",
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now().UTC(),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 6, WordsCount: 2, ExpGained: 16, Words: []string{"кот", "дом"}},
			{PlayerID: p2.String(), Score: 0, WordsCount: 0, ExpGained: 0},
		},
	}))

	var words string
	require.NoError(t, s.Pool().QueryRow(ctx,
		`SELECT words::text FROM game_result_players p
		 JOIN game_results r ON r.id = p.game_result_id
		 WHERE r.game_id = $1 AND p.player_id = $2`,
		"33333333-3333-3333-3333-333333333333", p1,
	).Scan(&words))
	assert.JSONEq(t, `["кот","дом"]`, words)

	// A player without words gets an empty array, not NULL.
	require.NoError(t, s.Pool().QueryRow(ctx,
		`SELECT words::text FROM game_result_players p
		 JOIN game_results r ON r.id = p.game_result_id
		 WHERE r.game_id = $1 AND p.player_id = $2`,
		"33333333-3333-3333-3333-333333333333", p2,
	).Scan(&words))
	assert.JSONEq(t, `[]`, words)
}
