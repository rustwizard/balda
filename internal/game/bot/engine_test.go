package bot

import (
	"testing"
	"unicode/utf8"

	"github.com/rustwizard/balda/internal/game"
)

func makeBoardWithWord(t *testing.T, word string) [5][5]string {
	t.Helper()
	if utf8.RuneCountInString(word) > 5 {
		t.Fatalf("makeBoardWithWord: word %q is longer than 5 runes", word)
	}
	var board [5][5]string
	row := uint8(2)
	idx := 0
	for _, r := range word {
		board[row][idx] = string(r)
		idx++
	}
	return board
}

func TestNewRandomValidStrategy_SharesTrieForGlobalDict(t *testing.T) {
	s1 := NewRandomValidStrategy(game.Dict)
	s2 := NewRandomValidStrategy(game.Dict)
	if s1.trie != s2.trie {
		t.Fatal("expected NewRandomValidStrategy to share the trie for game.Dict")
	}
}

func TestRandomValidStrategy_FindMove(t *testing.T) {
	dict := &game.Dictionary{
		Definition: map[string]string{
			"кот":   "домашнее животное",
			"кошка": "домашнее животное",
			"ток":   "место для скота",
			"тока":  "род падежа",
		},
	}
	strategy := NewRandomValidStrategy(dict)

	board := makeBoardWithWord(t, "котка")
	letter, path, err := strategy.MakeMove(board, nil)
	if err != nil {
		t.Fatalf("expected move, got error: %v", err)
	}
	if letter == nil {
		t.Fatal("expected new letter")
	}
	if len(path) < 3 {
		t.Fatalf("expected word path of length >= 3, got %d", len(path))
	}

	// The new letter must be part of the word path.
	found := false
	for _, l := range path {
		if l.RowID == letter.RowID && l.ColID == letter.ColID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("new letter must be included in the word path")
	}
}

func TestRandomValidStrategy_NoValidMove(t *testing.T) {
	dict := &game.Dictionary{
		Definition: map[string]string{
			"абракадабра": "непонятное слово",
		},
	}
	strategy := NewRandomValidStrategy(dict)

	// Full board without any valid continuation.
	board := [5][5]string{}
	for r := range board {
		for c := range board[r] {
			board[r][c] = "а"
		}
	}

	_, _, err := strategy.MakeMove(board, nil)
	if err == nil {
		t.Fatal("expected no move found")
	}
}

func TestRandomValidStrategy_YoNormalized(t *testing.T) {
	dict := &game.Dictionary{
		Definition: map[string]string{
			"ёлка": "новогоднее дерево",
		},
	}
	strategy := NewRandomValidStrategy(dict)

	// Center row contains "елк"; placing 'а' can form "ёлка" which is
	// normalized to "елка" by the game package.
	board := makeBoardWithWord(t, "елк")
	_, path, err := strategy.MakeMove(board, nil)
	if err != nil {
		t.Fatalf("expected move, got error: %v", err)
	}

	word := game.MakeWord(path)
	if word != "елка" {
		t.Fatalf("expected normalized word 'елка', got %q", word)
	}
}

func TestRandomValidStrategy_RespectsUsedWords(t *testing.T) {
	dict := &game.Dictionary{
		Definition: map[string]string{
			"кот": "домашнее животное",
		},
	}
	strategy := NewRandomValidStrategy(dict)

	board := makeBoardWithWord(t, "котка")
	// If "кот" is already used, the bot should not return it again.
	// There may still be other valid moves; we only check that the returned
	// word is not the used one when a move is found.
	_, path, err := strategy.MakeMove(board, []string{"кот"})
	if err != nil {
		return // no other valid moves on this board
	}
	word := game.MakeWord(path)
	if word == "кот" {
		t.Fatal("bot returned an already used word")
	}
}
