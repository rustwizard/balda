package bot

import "github.com/rustwizard/balda/internal/game"

// Strategy chooses a move for the bot given the current board and used words.
type Strategy interface {
	// MakeMove returns the new letter to place and the word path that uses it.
	// If no valid move exists, it returns a non-nil error.
	MakeMove(board [5][5]string, usedWords []string) (*game.Letter, []game.Letter, error)
}
