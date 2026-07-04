package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEloDelta(t *testing.T) {
	cases := []struct {
		name           string
		rating         int
		opponentRating int
		score          float64
		want           int
	}{
		{"equal rated win", 1000, 1000, 1.0, 16},
		{"equal rated loss", 1000, 1000, 0.0, -16},
		{"equal rated draw", 1000, 1000, 0.5, 0},
		{"underdog win", 1200, 1400, 1.0, 24},
		{"underdog loss", 1200, 1400, 0.0, -8},
		{"favorite win", 1400, 1200, 1.0, 8},
		{"favorite loss", 1400, 1200, 0.0, -24},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, storage.EloDelta(tc.rating, tc.opponentRating, tc.score))
		})
	}
}

func TestExpGained(t *testing.T) {
	cases := []struct {
		name     string
		score    int
		isWinner bool
		isDraw   bool
		want     int
	}{
		{"winner", 7, true, false, 17},
		{"draw", 3, false, true, 8},
		{"loser", 5, false, false, 5},
		{"zero score winner", 0, true, false, 10},
		{"zero score draw", 0, false, true, 5},
		{"zero score loser", 0, false, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, storage.ExpGained(tc.score, tc.isWinner, tc.isDraw))
		})
	}
}

// seedPlayer inserts a user + player_state row and returns the player_id UUID.
func seedPlayer(ctx context.Context, t *testing.T, s *storage.Balda, email string) uuid.UUID {
	return seedPlayerWithRating(ctx, t, s, email, storage.DefaultRating)
}

// seedPlayerWithRating inserts a user + player_state row with a specific rating.
func seedPlayerWithRating(ctx context.Context, t *testing.T, s *storage.Balda, email string, rating int) uuid.UUID {
	t.Helper()
	playerID := uuid.New()
	var userID int64
	err := s.Pool().QueryRow(ctx,
		`INSERT INTO users (first_name, last_name, email, hash_password) VALUES ('Test', 'User', $1, 'x') RETURNING user_id`,
		email,
	).Scan(&userID)
	require.NoError(t, err)

	_, err = s.Pool().Exec(ctx,
		`INSERT INTO player_state (user_id, player_id, nickname, exp, rating, flags, lives) VALUES ($1, $2, $3, 0, $4, 0, 5)`,
		userID, playerID, email, rating,
	)
	require.NoError(t, err)
	return playerID
}

func TestSaveGameResult_Winner(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayer(ctx, t, s, "gr.winner@example.org")
	p2 := seedPlayer(ctx, t, s, "gr.loser@example.org")
	gameID := uuid.New()

	result := storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now().UTC().Truncate(time.Second),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 8, WordsCount: 3, ExpGained: storage.ExpGained(8, true, false)},
			{PlayerID: p2.String(), Score: 5, WordsCount: 2, ExpGained: storage.ExpGained(5, false, false)},
		},
	}

	require.NoError(t, s.SaveGameResult(ctx, result))

	// game_results row
	var winnerID string
	var finishReason string
	err := s.Pool().QueryRow(ctx,
		`SELECT winner_id, finish_reason FROM game_results WHERE game_id = $1`, gameID,
	).Scan(&winnerID, &finishReason)
	require.NoError(t, err)
	assert.Equal(t, p1.String(), winnerID)
	assert.Equal(t, "game_finished", finishReason)

	// game_result_players rows
	rows, err := s.Pool().Query(ctx,
		`SELECT player_id, score, words_count, exp_gained FROM game_result_players
		 JOIN game_results ON game_results.id = game_result_players.game_result_id
		 WHERE game_results.game_id = $1`, gameID,
	)
	require.NoError(t, err)
	defer rows.Close()

	type playerRow struct {
		playerID   string
		score      int
		wordsCount int
		expGained  int
	}
	var got []playerRow
	for rows.Next() {
		var r playerRow
		require.NoError(t, rows.Scan(&r.playerID, &r.score, &r.wordsCount, &r.expGained))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2)

	byID := make(map[string]playerRow, 2)
	for _, r := range got {
		byID[r.playerID] = r
	}

	assert.Equal(t, 8, byID[p1.String()].score)
	assert.Equal(t, 3, byID[p1.String()].wordsCount)
	assert.Equal(t, 18, byID[p1.String()].expGained) // 10+8

	assert.Equal(t, 5, byID[p2.String()].score)
	assert.Equal(t, 2, byID[p2.String()].wordsCount)
	assert.Equal(t, 5, byID[p2.String()].expGained) // score=5 (loser gets raw score)

	// player_state.exp updated
	checkExp := func(playerID uuid.UUID, want int64) {
		t.Helper()
		var exp int64
		require.NoError(t, s.Pool().QueryRow(ctx,
			`SELECT exp FROM player_state WHERE player_id = $1`, playerID,
		).Scan(&exp))
		assert.Equal(t, want, exp)
	}
	checkExp(p1, 18)
	checkExp(p2, 5)
}

func TestSaveGameResult_UpdatesRating(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayerWithRating(ctx, t, s, "gr.rating.winner@example.org", 1200)
	p2 := seedPlayerWithRating(ctx, t, s, "gr.rating.loser@example.org", 1400)
	gameID := uuid.New()

	result := storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now().UTC().Truncate(time.Second),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 8, WordsCount: 3, ExpGained: storage.ExpGained(8, true, false)},
			{PlayerID: p2.String(), Score: 5, WordsCount: 2, ExpGained: storage.ExpGained(5, false, false)},
		},
	}

	require.NoError(t, s.SaveGameResult(ctx, result))

	checkRating := func(playerID uuid.UUID, want int) {
		t.Helper()
		var rating int
		require.NoError(t, s.Pool().QueryRow(ctx,
			`SELECT rating FROM player_state WHERE player_id = $1`, playerID,
		).Scan(&rating))
		assert.Equal(t, want, rating)
	}

	// Underdog (1200) beats favorite (1400): delta = +24.
	checkRating(p1, 1224)
	// Favorite loses: delta = -24.
	checkRating(p2, 1376)
}

func TestSaveGameResult_Draw(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayer(ctx, t, s, "gr.draw1@example.org")
	p2 := seedPlayer(ctx, t, s, "gr.draw2@example.org")
	gameID := uuid.New()

	result := storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     "", // draw
		FinishReason: storage.FinishReasonAcceptEnd,
		FinishedAt:   time.Now().UTC().Truncate(time.Second),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 4, WordsCount: 2, ExpGained: storage.ExpGained(4, false, true)},
			{PlayerID: p2.String(), Score: 4, WordsCount: 2, ExpGained: storage.ExpGained(4, false, true)},
		},
	}

	require.NoError(t, s.SaveGameResult(ctx, result))

	var winnerID *string
	var finishReason string
	err := s.Pool().QueryRow(ctx,
		`SELECT winner_id, finish_reason FROM game_results WHERE game_id = $1`, gameID,
	).Scan(&winnerID, &finishReason)
	require.NoError(t, err)
	assert.Nil(t, winnerID, "draw: winner_id must be NULL")
	assert.Equal(t, "accept_end", finishReason)

	checkExp := func(playerID uuid.UUID, want int64) {
		t.Helper()
		var exp int64
		require.NoError(t, s.Pool().QueryRow(ctx,
			`SELECT exp FROM player_state WHERE player_id = $1`, playerID,
		).Scan(&exp))
		assert.Equal(t, want, exp)
	}
	checkExp(p1, 9) // 5+4
	checkExp(p2, 9)
}

func TestSaveGameResult_Kick(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayer(ctx, t, s, "gr.kicker@example.org")
	p2 := seedPlayer(ctx, t, s, "gr.kicked@example.org")
	gameID := uuid.New()

	result := storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonKick,
		FinishedAt:   time.Now().UTC().Truncate(time.Second),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 2, WordsCount: 1, ExpGained: storage.ExpGained(2, true, false)},
			{PlayerID: p2.String(), Score: 0, WordsCount: 0, ExpGained: storage.ExpGained(0, false, false)},
		},
	}

	require.NoError(t, s.SaveGameResult(ctx, result))

	var finishReason string
	err := s.Pool().QueryRow(ctx,
		`SELECT finish_reason FROM game_results WHERE game_id = $1`, gameID,
	).Scan(&finishReason)
	require.NoError(t, err)
	assert.Equal(t, "kick", finishReason)
}

func TestSaveGameResult_DuplicateGameID(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	p1 := seedPlayer(ctx, t, s, "gr.dup1@example.org")
	p2 := seedPlayer(ctx, t, s, "gr.dup2@example.org")
	gameID := uuid.New()

	result := storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now().UTC(),
		Players: []storage.PlayerResult{
			{PlayerID: p1.String(), Score: 1, WordsCount: 1, ExpGained: 11},
			{PlayerID: p2.String(), Score: 0, WordsCount: 0, ExpGained: 0},
		},
	}

	require.NoError(t, s.SaveGameResult(ctx, result))
	// saving the same game_id again must fail (unique constraint)
	err := s.SaveGameResult(ctx, result)
	require.Error(t, err, "expected error on duplicate game_id")
}
