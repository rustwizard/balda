package bot

import (
	"context"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rustwizard/balda/internal/game"
)

// noopHumanNotifier discards all human-facing events for the integration test.
type noopHumanNotifier struct{}

func (noopHumanNotifier) NotifyTimeout(_ string, _ int, _ bool) {}
func (noopHumanNotifier) NotifySkip(_ string, _ int, _ bool)    {}
func (noopHumanNotifier) NotifyKick(_ string)                   {}
func (noopHumanNotifier) NotifyGameFinished()                   {}
func (noopHumanNotifier) NotifyTurnStart(_ string)              {}
func (noopHumanNotifier) NotifyEndProposed(_ string)            {}
func (noopHumanNotifier) NotifyEndAccepted()                    {}
func (noopHumanNotifier) NotifyEndRejected(_ time.Duration)     {}

func TestBotVsHuman_GameProgresses(t *testing.T) {
	human := &game.Player{ID: "human", Type: game.PlayerTypeHuman}
	botPlayer := &game.Player{ID: "bot", Type: game.PlayerTypeBot}

	coord := noopHumanNotifier{}
	engine := NewEngine(NewRandomValidStrategy(game.Dict))
	botNotifier := NewNotifier(engine, botPlayer.ID)
	composite := game.NewCompositeNotifier(coord, botNotifier)

	g, err := game.NewGame([]*game.Player{human, botPlayer}, composite)
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	botNotifier.SetGame(g)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	// Wait for the initial turn_change to be processed.
	time.Sleep(100 * time.Millisecond)

	// It's the human's turn (index 0). Make a valid first move.
	board := g.BoardSnapshot()
	var moveLetter *game.Letter
	var movePath []game.Letter
	used := g.UsedWords()

	for r := range board {
		for c := range board[r] {
			if board[r][c] != "" {
				continue
			}
			for _, letterRune := range russianAlphabet {
				letter := game.Letter{
					RowID: uint8(r),
					ColID: uint8(c),
					Char:  string(letterRune),
				}
				path := findWordForTest(board, letter, used)
				if path != nil {
					moveLetter = &letter
					movePath = path
					break
				}
			}
			if moveLetter != nil {
				break
			}
		}
		if moveLetter != nil {
			break
		}
	}

	if moveLetter == nil {
		t.Skip("could not find a valid human move for this board word")
	}

	if err := g.SubmitWord(human.ID, moveLetter, movePath); err != nil {
		t.Fatalf("human submit: %v", err)
	}

	// Wait for the bot to respond.
	time.Sleep(3 * time.Second)

	if g.MoveNumber() < 2 {
		t.Fatalf("expected bot to have moved, move number is %d", g.MoveNumber())
	}
}

// findWordForTest is a minimal helper that finds any valid word starting with the
// placed letter. It builds a local trie from the production dictionary.
func findWordForTest(board [5][5]string, letter game.Letter, used []string) []game.Letter {
	words := make([]string, 0, len(game.Dict.Definition))
	for w := range game.Dict.Definition {
		words = append(words, w)
	}
	t := NewTrie(words)

	usedMap := make(map[string]struct{}, len(used))
	for _, w := range used {
		usedMap[w] = struct{}{}
	}

	temp := board
	temp[letter.RowID][letter.ColID] = letter.Char

	visited := [5][5]bool{}
	visited[letter.RowID][letter.ColID] = true

	start := []game.Letter{letter}
	prefix := letter.Char

	if !t.IsPrefix(prefix) {
		return nil
	}
	if utf8.RuneCountInString(prefix) >= 3 && t.IsWord(prefix) {
		if _, ok := usedMap[prefix]; !ok {
			return start
		}
	}

	var result []game.Letter
	var dfs func(row, col uint8, path []game.Letter, word string)
	dfs = func(row, col uint8, path []game.Letter, word string) {
		if result != nil {
			return
		}
		// Balda allows only horizontal and vertical adjacency.
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
			if !t.IsPrefix(nextWord) {
				continue
			}
			nextPath := make([]game.Letter, len(path)+1)
			copy(nextPath, path)
			nextPath[len(path)] = game.Letter{RowID: uint8(nr), ColID: uint8(nc), Char: ch}
			if utf8.RuneCountInString(nextWord) >= 3 && t.IsWord(nextWord) {
				if _, ok := usedMap[nextWord]; !ok {
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
