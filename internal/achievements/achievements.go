package achievements

import "unicode/utf8"

// Achievement is a single achievement definition.
type Achievement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Unlocked    bool   `json:"unlocked"`
}

// Bit flags for player_state.flags.
const (
	FlagFirstGame int64 = 1 << iota
	FlagFirstWin
	FlagHighScorer50
	FlagWordsmith10
	FlagGiantWord
	FlagWinningStreak3
	FlagVeteran10
)

// PlayerGameStats holds per-game data needed to evaluate achievements.
type PlayerGameStats struct {
	TotalGames      int
	ConsecutiveWins int
	Score           int
	WordsCount      int
	BestWordLength  int
	IsWinner        bool
}

var registry = []Achievement{
	{ID: "first_game", Name: "Дебютант", Description: "Сыграть первую партию"},
	{ID: "first_win", Name: "Первая победа", Description: "Одержать первую победу"},
	{ID: "high_scorer_50", Name: "Рекордсмен", Description: "Набрать 50+ очков за партию"},
	{ID: "wordsmith_10", Name: "Словесный мастер", Description: "Составить 10+ слов за партию"},
	{ID: "giant_word", Name: "Гигант", Description: "Составить слово из 10+ букв"},
	{ID: "winning_streak_3", Name: "Победная серия", Description: "3 победы подряд"},
	{ID: "veteran_10", Name: "Ветеран", Description: "Сыграть 10 партий"},
}

var flagByID = map[string]int64{
	registry[0].ID: FlagFirstGame,
	registry[1].ID: FlagFirstWin,
	registry[2].ID: FlagHighScorer50,
	registry[3].ID: FlagWordsmith10,
	registry[4].ID: FlagGiantWord,
	registry[5].ID: FlagWinningStreak3,
	registry[6].ID: FlagVeteran10,
}

// Calculate evaluates achievements for a single player after a game.
// It returns the updated bitmask and the list of newly unlocked achievements.
func Calculate(oldFlags int64, s PlayerGameStats) (newFlags int64, unlocked []Achievement) {
	flags := oldFlags

	check := func(bit int64, cond bool, a Achievement) {
		if flags&bit == 0 && cond {
			flags |= bit
			unlocked = append(unlocked, a)
		}
	}

	check(FlagFirstGame, s.TotalGames >= 1, registry[0])
	check(FlagFirstWin, s.IsWinner, registry[1])
	check(FlagHighScorer50, s.Score >= 50, registry[2])
	check(FlagWordsmith10, s.WordsCount >= 10, registry[3])
	check(FlagGiantWord, s.BestWordLength >= 10, registry[4])
	check(FlagWinningStreak3, s.ConsecutiveWins >= 3, registry[5])
	check(FlagVeteran10, s.TotalGames >= 10, registry[6])

	return flags, unlocked
}

// List returns the full registry with Unlocked status for the given flags.
func List(flags int64) []Achievement {
	out := make([]Achievement, len(registry))
	for i, a := range registry {
		out[i] = a
		out[i].Unlocked = flags&flagByID[a.ID] != 0
	}
	return out
}

// WordLength returns the number of runes in a word.
func WordLength(w string) int {
	return utf8.RuneCountInString(w)
}
