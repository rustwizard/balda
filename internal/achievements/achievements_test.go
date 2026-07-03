package achievements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculate(t *testing.T) {
	t.Run("first game unlocked", func(t *testing.T) {
		flags, unlocked := Calculate(0, PlayerGameStats{TotalGames: 1})
		assert.NotZero(t, flags&FlagFirstGame)
		assert.Len(t, unlocked, 1)
		assert.Equal(t, "first_game", unlocked[0].ID)
	})

	t.Run("first win unlocked", func(t *testing.T) {
		flags, unlocked := Calculate(0, PlayerGameStats{TotalGames: 1, IsWinner: true})
		assert.NotZero(t, flags&FlagFirstGame)
		assert.NotZero(t, flags&FlagFirstWin)
		assert.Len(t, unlocked, 2)
	})

	t.Run("high scorer and wordsmith", func(t *testing.T) {
		flags, _ := Calculate(0, PlayerGameStats{
			TotalGames: 1,
			IsWinner:   true,
			Score:      55,
			WordsCount: 12,
		})
		assert.NotZero(t, flags&FlagHighScorer50)
		assert.NotZero(t, flags&FlagWordsmith10)
	})

	t.Run("giant word", func(t *testing.T) {
		flags, unlocked := Calculate(0, PlayerGameStats{
			TotalGames:     1,
			BestWordLength: 11,
		})
		assert.NotZero(t, flags&FlagGiantWord)
		assert.Contains(t, ids(unlocked), "giant_word")
	})

	t.Run("winning streak 3", func(t *testing.T) {
		flags, unlocked := Calculate(FlagFirstGame|FlagFirstWin, PlayerGameStats{
			TotalGames:      3,
			IsWinner:        true,
			ConsecutiveWins: 3,
		})
		assert.NotZero(t, flags&FlagWinningStreak3)
		assert.Len(t, unlocked, 1)
		assert.Equal(t, "winning_streak_3", unlocked[0].ID)
	})

	t.Run("veteran 10", func(t *testing.T) {
		flags, unlocked := Calculate(FlagFirstGame, PlayerGameStats{
			TotalGames: 10,
		})
		assert.NotZero(t, flags&FlagVeteran10)
		assert.Len(t, unlocked, 1)
		assert.Equal(t, "veteran_10", unlocked[0].ID)
	})

	t.Run("already unlocked not duplicated", func(t *testing.T) {
		flags, unlocked := Calculate(FlagFirstGame, PlayerGameStats{TotalGames: 1})
		assert.Zero(t, flags&^FlagFirstGame)
		assert.Empty(t, unlocked)
	})

	t.Run("loss resets streak", func(t *testing.T) {
		_, unlocked := Calculate(FlagFirstGame|FlagWinningStreak3, PlayerGameStats{
			TotalGames:      4,
			IsWinner:        false,
			ConsecutiveWins: 0,
		})
		assert.Empty(t, unlocked)
	})
}

func TestList(t *testing.T) {
	flags := FlagFirstGame | FlagVeteran10
	list := List(flags)

	assert.Len(t, list, len(registry))

	unlocked := make(map[string]bool)
	for _, a := range list {
		unlocked[a.ID] = a.Unlocked
	}

	assert.True(t, unlocked["first_game"])
	assert.True(t, unlocked["veteran_10"])
	assert.False(t, unlocked["first_win"])
	assert.False(t, unlocked["high_scorer_50"])
}

func TestWordLength(t *testing.T) {
	assert.Equal(t, 5, WordLength("масло"))
	assert.Equal(t, 10, WordLength("достаточно"))
	assert.Equal(t, 0, WordLength(""))
}

func ids(a []Achievement) []string {
	out := make([]string, len(a))
	for i, x := range a {
		out[i] = x.ID
	}
	return out
}
