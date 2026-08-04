package achievements

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(defs []Definition) *Service {
	return NewService(func(_ context.Context) ([]Definition, error) {
		return defs, nil
	})
}

func TestServiceLoad(t *testing.T) {
	svc := newTestService([]Definition{
		{ID: "first_game", Name: "First Game", ConditionType: ConditionTotalGames, Operator: OperatorGTE, Threshold: 1, BitPosition: 0},
		{ID: "first_win", Name: "First Win", ConditionType: ConditionWin, Operator: OperatorGTE, Threshold: 1, BitPosition: 1},
	})
	require.NoError(t, svc.Load(context.Background()))

	list := svc.List(0)
	require.Len(t, list, 2)
	assert.Equal(t, "first_game", list[0].ID)
}

func TestCalculate(t *testing.T) {
	svc := newTestService([]Definition{
		{ID: "first_game", Name: "First Game", ConditionType: ConditionTotalGames, Operator: OperatorGTE, Threshold: 1, BitPosition: 0},
		{ID: "first_win", Name: "First Win", ConditionType: ConditionWin, Operator: OperatorGTE, Threshold: 1, BitPosition: 1},
		{ID: "high_scorer_50", Name: "High Scorer", ConditionType: ConditionScore, Operator: OperatorGTE, Threshold: 50, BitPosition: 2},
		{ID: "wordsmith_10", Name: "Wordsmith", ConditionType: ConditionWordsCount, Operator: OperatorGTE, Threshold: 10, BitPosition: 3},
		{ID: "giant_word", Name: "Giant Word", ConditionType: ConditionBestWordLength, Operator: OperatorGTE, Threshold: 10, BitPosition: 4},
		{ID: "winning_streak_3", Name: "Streak", ConditionType: ConditionConsecutiveWins, Operator: OperatorGTE, Threshold: 3, BitPosition: 5},
		{ID: "veteran_10", Name: "Veteran", ConditionType: ConditionTotalGames, Operator: OperatorGTE, Threshold: 10, BitPosition: 6},
	})
	require.NoError(t, svc.Load(context.Background()))

	t.Run("first game unlocked", func(t *testing.T) {
		flags, unlocked := svc.Calculate(0, PlayerGameStats{TotalGames: 1})
		assert.NotZero(t, flags&(1<<0))
		assert.Len(t, unlocked, 1)
		assert.Equal(t, "first_game", unlocked[0].ID)
	})

	t.Run("first win unlocked", func(t *testing.T) {
		flags, unlocked := svc.Calculate(0, PlayerGameStats{TotalGames: 1, IsWinner: true})
		assert.NotZero(t, flags&(1<<0))
		assert.NotZero(t, flags&(1<<1))
		assert.Len(t, unlocked, 2)
	})

	t.Run("high scorer and wordsmith", func(t *testing.T) {
		flags, _ := svc.Calculate(0, PlayerGameStats{
			TotalGames: 1,
			IsWinner:   true,
			Score:      55,
			WordsCount: 12,
		})
		assert.NotZero(t, flags&(1<<2))
		assert.NotZero(t, flags&(1<<3))
	})

	t.Run("giant word", func(t *testing.T) {
		flags, unlocked := svc.Calculate(0, PlayerGameStats{
			TotalGames:     1,
			BestWordLength: 11,
		})
		assert.NotZero(t, flags&(1<<4))
		assert.Contains(t, ids(unlocked), "giant_word")
	})

	t.Run("winning streak 3", func(t *testing.T) {
		flags, unlocked := svc.Calculate((1<<0)|(1<<1), PlayerGameStats{
			TotalGames:      3,
			IsWinner:        true,
			ConsecutiveWins: 3,
		})
		assert.NotZero(t, flags&(1<<5))
		assert.Len(t, unlocked, 1)
		assert.Equal(t, "winning_streak_3", unlocked[0].ID)
	})

	t.Run("veteran 10", func(t *testing.T) {
		flags, unlocked := svc.Calculate((1 << 0), PlayerGameStats{
			TotalGames: 10,
		})
		assert.NotZero(t, flags&(1<<6))
		assert.Len(t, unlocked, 1)
		assert.Equal(t, "veteran_10", unlocked[0].ID)
	})

	t.Run("already unlocked not duplicated", func(t *testing.T) {
		flags, unlocked := svc.Calculate((1 << 0), PlayerGameStats{TotalGames: 1})
		assert.Zero(t, flags&^(1<<0))
		assert.Empty(t, unlocked)
	})

	t.Run("loss resets streak", func(t *testing.T) {
		_, unlocked := svc.Calculate((1<<0)|(1<<5), PlayerGameStats{
			TotalGames:      4,
			IsWinner:        false,
			ConsecutiveWins: 0,
		})
		assert.Empty(t, unlocked)
	})

	t.Run("reload picks up new achievement", func(t *testing.T) {
		updated := []Definition{
			{ID: "first_game", Name: "First Game", ConditionType: ConditionTotalGames, Operator: OperatorGTE, Threshold: 1, BitPosition: 0},
			{ID: "new_one", Name: "New One", ConditionType: ConditionScore, Operator: OperatorGTE, Threshold: 100, BitPosition: 7},
		}
		svc := NewService(func(_ context.Context) ([]Definition, error) {
			return updated, nil
		})
		require.NoError(t, svc.Load(context.Background()))

		flags, unlocked := svc.Calculate(0, PlayerGameStats{TotalGames: 1, Score: 100})
		assert.NotZero(t, flags&(1<<0))
		assert.NotZero(t, flags&(1<<7))
		assert.Len(t, unlocked, 2)
	})
}

func TestList(t *testing.T) {
	svc := newTestService([]Definition{
		{ID: "first_game", Name: "First Game", ConditionType: ConditionTotalGames, Operator: OperatorGTE, Threshold: 1, BitPosition: 0},
		{ID: "veteran_10", Name: "Veteran", ConditionType: ConditionTotalGames, Operator: OperatorGTE, Threshold: 10, BitPosition: 6},
	})
	require.NoError(t, svc.Load(context.Background()))

	flags := int64((1 << 0) | (1 << 6))
	list := svc.List(flags)
	require.Len(t, list, 2)

	unlocked := make(map[string]bool)
	for _, a := range list {
		unlocked[a.ID] = a.Unlocked
	}

	assert.True(t, unlocked["first_game"])
	assert.True(t, unlocked["veteran_10"])
}

func TestWordLength(t *testing.T) {
	assert.Equal(t, 5, wordLength("масло"))
	assert.Equal(t, 10, wordLength("достаточно"))
	assert.Equal(t, 0, wordLength(""))
}

func ids(a []Achievement) []string {
	out := make([]string, len(a))
	for i, x := range a {
		out[i] = x.ID
	}
	return out
}
