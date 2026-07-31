package bot

import (
	"errors"
	"math/rand"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/rustwizard/balda/internal/game"
)

// ErrNoMoveFound is returned when the bot cannot find any valid move.
var ErrNoMoveFound = errors.New("bot: no valid move found")

// BotPlayerID is the fixed player UUID used for the bot in every game.
// The bot deliberately has no player_state row: results of bot games are
// persisted (the human gets ELO/EXP/stats), but the bot itself exists only
// in memory. A stable ID keeps game_result_players rows attributable.
const BotPlayerID = "00000000-0000-0000-0000-000000000b07"

// normalizeWord replaces ё/Ё with е/Е so that words differing only by this
// letter are treated as identical, matching the game package behavior.
func normalizeWord(word string) string {
	word = strings.ReplaceAll(word, "ё", "е")
	word = strings.ReplaceAll(word, "Ё", "Е")
	return word
}

// russianAlphabet contains normalized lowercase letters used by the dictionary.
// ё is normalized to е by the game package, so it is omitted here.
var russianAlphabet = []rune("абвгдежзийклмнопрстуфхцчшщъыьэюя")

// boardDirections are the four orthogonal moves allowed on the 5×5 board.
var boardDirections = [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}

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

var (
	globalTrie     *Trie
	globalTrieOnce sync.Once
)

// globalBotTrie returns a singleton trie built from the production dictionary.
func globalBotTrie() *Trie {
	globalTrieOnce.Do(func() {
		words := make([]string, 0, len(game.Dict.Definition))
		for w := range game.Dict.Definition {
			words = append(words, normalizeWord(w))
		}
		globalTrie = NewTrie(words)
	})
	return globalTrie
}

// NewRandomValidStrategy builds a strategy backed by a trie of all dictionary words.
// When the production dictionary is used, the underlying trie is shared across
// all strategy instances to avoid rebuilding it for every bot.
func NewRandomValidStrategy(dict *game.Dictionary) *RandomValidStrategy {
	if dict == game.Dict {
		return &RandomValidStrategy{trie: globalBotTrie()}
	}

	words := make([]string, 0, len(dict.Definition))
	for w := range dict.Definition {
		words = append(words, normalizeWord(w))
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
	// Copy the 5x5 array by value so we can place the candidate letter
	// without mutating the caller's board snapshot.
	temp := board
	temp[letter.RowID][letter.ColID] = letter.Char

	var result []game.Letter
	// visited tracks cells in the current DFS path. It is shared by recursive
	// calls for a single start cell and reset when the outer loop moves to the
	// next start cell. Arrays are value types in Go, so `visited = [5][5]bool{}`
	// copies the whole matrix rather than creating a shared reference.
	var visited [5][5]bool
	var dfs func(row, col uint8, path []game.Letter, word string, usedNew bool)
	dfs = func(row, col uint8, path []game.Letter, word string, usedNew bool) {
		if result != nil {
			return
		}
		if usedNew && utf8.RuneCountInString(word) >= 3 && s.trie.IsWord(word) {
			if _, ok := used[word]; !ok {
				result = make([]game.Letter, len(path))
				copy(result, path)
				return
			}
		}
		// Balda allows only horizontal and vertical adjacency (Manhattan distance == 1).
		for _, d := range boardDirections {
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
			nextUsedNew := usedNew || (nr == int(letter.RowID) && nc == int(letter.ColID))
			nextPath := make([]game.Letter, len(path)+1)
			copy(nextPath, path)
			nextPath[len(path)] = game.Letter{RowID: uint8(nr), ColID: uint8(nc), Char: ch}
			visited[nr][nc] = true
			dfs(uint8(nr), uint8(nc), nextPath, nextWord, nextUsedNew)
			visited[nr][nc] = false
		}
	}

	// Try every occupied cell as a starting point; the path must include the new letter.
	for r := range temp {
		for c := range temp[r] {
			ch := temp[r][c]
			if ch == "" {
				continue
			}
			if !s.trie.IsPrefix(ch) {
				continue
			}
			visited = [5][5]bool{}
			visited[r][c] = true
			startUsedNew := r == int(letter.RowID) && c == int(letter.ColID)
			dfs(uint8(r), uint8(c), []game.Letter{{RowID: uint8(r), ColID: uint8(c), Char: ch}}, ch, startUsedNew)
			if result != nil {
				return result
			}
		}
	}
	return nil
}
