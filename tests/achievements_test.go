package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/achievements"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveGameResultWithAchievements(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	achSvc := achievements.NewService(s.LoadAchievementDefinitions)
	require.NoError(t, achSvc.Load(ctx))

	p1 := seedPlayerWithRating(ctx, t, s, "ach.winner@example.org", 1000)
	p2 := seedPlayerWithRating(ctx, t, s, "ach.loser@example.org", 1000)

	gameID := uuid.New()
	result := storage.GameResult{
		GameID:       gameID.String(),
		WinnerID:     p1.String(),
		FinishReason: storage.FinishReasonGameFinished,
		FinishedAt:   time.Now(),
		Players: []storage.PlayerResult{
			{
				PlayerID:       p1.String(),
				Score:          55,
				WordsCount:     12,
				ExpGained:      65,
				BestWordLength: 11,
			},
			{
				PlayerID:       p2.String(),
				Score:          30,
				WordsCount:     5,
				ExpGained:      30,
				BestWordLength: 5,
			},
		},
	}

	unlocked, err := s.SaveGameResultWithAchievements(ctx, result, achSvc)
	require.NoError(t, err)
	require.Len(t, unlocked, 2)

	winnerUnlocks := make(map[string][]string)
	for _, u := range unlocked {
		ids := make([]string, len(u.Unlocked))
		for i, a := range u.Unlocked {
			ids[i] = a.ID
		}
		winnerUnlocks[u.PlayerID] = ids
	}

	assert.ElementsMatch(t, []string{"first_game", "first_win", "high_scorer_50", "wordsmith_10", "giant_word"}, winnerUnlocks[p1.String()])
	assert.ElementsMatch(t, []string{"first_game"}, winnerUnlocks[p2.String()])

	ps, err := s.GetPlayerState(ctx, p1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, ps.TotalGames)
	assert.EqualValues(t, 1, ps.ConsecutiveWins)

	list := achSvc.List(ps.Flags)
	unlockedMap := make(map[string]bool)
	for _, a := range list {
		unlockedMap[a.ID] = a.Unlocked
	}
	assert.True(t, unlockedMap["first_game"])
	assert.True(t, unlockedMap["first_win"])
	assert.True(t, unlockedMap["high_scorer_50"])
	assert.True(t, unlockedMap["wordsmith_10"])
	assert.True(t, unlockedMap["giant_word"])
}

func TestGetPlayerAchievementsHTTP(t *testing.T) {
	srv, token, cleanup := setupServer(t)
	defer cleanup()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/balda/api/v1/player/achievements", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body baldaapi.PlayerAchievementsResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Achievements, 7)

	unlockedCount := 0
	for _, a := range body.Achievements {
		if a.Unlocked.Value {
			unlockedCount++
		}
	}
	assert.Zero(t, unlockedCount)
}
