package bot

import (
	"errors"
	"math/rand"
	"unicode/utf8"

	"github.com/rustwizard/balda/internal/game"
)

// ErrNoMoveFound is returned when the bot cannot find any valid move.
var ErrNoMoveFound = errors.New("bot: no valid move found")

// russianAlphabet contains normalized lowercase letters used by the dictionary.
// ё is normalized to е by the game package, so it is omitted here.
var russianAlphabet = []rune("абвгдежзийклмнопрстуфхцчшщъыьэюя")

// Engine wraps a strategy and the dictionary trie.
type Engine struct {
	strategy Strategy
}

// NewEngine creates a bot engine with the given dictionary and strategy.
func NewEngine(strategy Strategy) *Engine {
	return &Engine{strategy: strategy}
}

// MakeMove delegates to the configured strategy.
func (e *Engine) MakeMove(board [5][5]string, usedWords []string) (*game.Letter, []game.Letter, error) {
	return e.strategy.MakeMove(board, usedWords)
}

// RandomValidStrategy finds any valid word using DFS with trie prefix pruning.
type RandomValidStrategy struct {
	trie *Trie
}

// NewRandomValidStrategy builds a strategy backed by a trie of all dictionary words.
func NewRandomValidStrategy(dict *game.Dictionary) *RandomValidStrategy {
	words := make([]string, 0, len(dict.Definition))
	for w := range dict.Definition {
		words = append(words, w)
	}
	return &RandomValidStrategy{trie: NewTrie(words)}
}

// MakeMove searches for a valid move. It shuffles empty cells and letters for variety.
func (s *RandomValidStrategy) MakeMove(board [5][5]string, usedWords []string) (*game.Letter, []game.Letter, error) {
	used := make(map[string]struct{}, len(usedWords))
	for _, w := range usedWords {
		used[w] = struct{}{}
	}

	emptyCells := findEmptyCells(board)
	if len(emptyCells) == 0 {
		return nil, nil, ErrNoMoveFound
	}

	rand.Shuffle(len(emptyCells), func(i, j int) { emptyCells[i], emptyCells[j] = emptyCells[j], emptyCells[i] })
	letters := make([]rune, len(russianAlphabet))
	copy(letters, russianAlphabet)
	rand.Shuffle(len(letters), func(i, j int) { letters[i], letters[j] = letters[j], letters[i] })

	for _, cell := range emptyCells {
		for _, r := range letters {
			letter := game.Letter{
				RowID: cell.row,
				ColID: cell.col,
				Char:  string(r),
			}
			wordPath := s.findWordFrom(board, letter, used)
			if wordPath != nil {
				return &letter, wordPath, nil
			}
		}
	}

	return nil, nil, ErrNoMoveFound
}

// cell is a helper coordinate pair.
type cell struct {
	row uint8
	col uint8
}

func findEmptyCells(board [5][5]string) []cell {
	var cells []cell
	for r := range board {
		for c := range board[r] {
			if board[r][c] == "" {
				cells = append(cells, cell{row: uint8(r), col: uint8(c)})
			}
		}
	}
	return cells
}

func (s *RandomValidStrategy) findWordFrom(board [5][5]string, letter game.Letter, used map[string]struct{}) []game.Letter {
	temp := board
	temp[letter.RowID][letter.ColID] = letter.Char

	visited := [5][5]bool{}
	visited[letter.RowID][letter.ColID] = true

	start := []game.Letter{letter}
	prefix := letter.Char

	if !s.trie.IsPrefix(prefix) {
		return nil
	}
	if utf8.RuneCountInString(prefix) >= 3 && s.trie.IsWord(prefix) {
		if _, ok := used[prefix]; !ok {
			return start
		}
	}

	var result []game.Letter
	var dfs func(row, col uint8, path []game.Letter, word string)
	dfs = func(row, col uint8, path []game.Letter, word string) {
		if result != nil {
			return
		}
		// Balda allows only horizontal and vertical adjacency (Manhattan distance == 1).
		directions := [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
		for _, d := range directions {
			nr := int(row) + d[0]
			nc := int(col) + d[1]
			if nr < 0 || nr >= 5 || nc < 0 || nc >= 5 {
				continue
			}
			if visited[nr][nc] {
				continue
			}
			ch := temp[nr][nc]
			if ch == "" {
				continue
			}
			nextWord := word + ch
			if !s.trie.IsPrefix(nextWord) {
				continue
			}
			nextPath := make([]game.Letter, len(path)+1)
			copy(nextPath, path)
			nextPath[len(path)] = game.Letter{RowID: uint8(nr), ColID: uint8(nc), Char: ch}
			if utf8.RuneCountInString(nextWord) >= 3 && s.trie.IsWord(nextWord) {
				if _, ok := used[nextWord]; !ok {
					result = nextPath
					return
				}
			}
			visited[nr][nc] = true
			dfs(uint8(nr), uint8(nc), nextPath, nextWord)
			visited[nr][nc] = false
		}
	}

	dfs(letter.RowID, letter.ColID, start, prefix)
	return result
}
