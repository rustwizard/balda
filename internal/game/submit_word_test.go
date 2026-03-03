package game

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ────────────────────────────────────────────────────────────────

// makeGameWithBoard builds a Game (not running) with a controlled initial word.
func makeGameWithBoard(t testing.TB, n *mockNotifier, initWord string, ids ...string) *Game {
	t.Helper()
	players := makePlayers(ids...)
	board, err := NewLettersTable(initWord)
	require.NoError(t, err)
	g := &Game{
		players:  players,
		board:    board,
		eventCh:  make(chan TurnEvent, 4),
		done:     make(chan struct{}),
		notifier: n,
		state:    StateWaitingForMove,
	}
	g.current = 0
	g.turn = &Turn{
		PlayerID: players[0].ID,
		timer:    time.AfterFunc(time.Hour, func() {}),
	}
	return g
}

// addTestWord injects a word into the global dictionary for the duration of the test.
func addTestWord(t testing.TB, word string) {
	t.Helper()
	Dict.Definition[word] = "test-definition"
	t.Cleanup(func() { delete(Dict.Definition, word) })
}

// ─── fixed test fixtures ─────────────────────────────────────────────────────
//
// Board initial word: "волна" → в(2,0) о(2,1) л(2,2) н(2,3) а(2,4)
// New letter:         "е" at (3,3), adjacent to н(2,3) below it.
// Word path:          в→о→л→н→е = "волне" (5 letters, each adjacent).

const testBoardWord = "волна"

var testNewLetter = Letter{RowID: 3, ColID: 3, Char: "е"}

// testWord spells "волне":  в(2,0)→о(2,1)→л(2,2)→н(2,3)→е(3,3)
var testWord = []Letter{
	{RowID: 2, ColID: 0, Char: "в"},
	{RowID: 2, ColID: 1, Char: "о"},
	{RowID: 2, ColID: 2, Char: "л"},
	{RowID: 2, ColID: 3, Char: "н"},
	{RowID: 3, ColID: 3, Char: "е"},
}

const testWordStr = "волне"

// ─── error cases ─────────────────────────────────────────────────────────────

func TestSubmitWord_WrongState(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	g.state = StatePlayerTimedOut

	err := g.SubmitWord("p1", &testNewLetter, testWord)
	assert.ErrorIs(t, err, ErrWrongState)
}

func TestSubmitWord_WrongPlayer(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()

	err := g.SubmitWord("p2", &testNewLetter, testWord)
	assert.ErrorIs(t, err, ErrNotYourTurn)
}

func TestSubmitWord_WordHasGaps(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()

	// в(2,0) to е(3,3): Manhattan distance = 1+3 = 4 ≠ 1 → gap.
	gapped := []Letter{
		{RowID: 2, ColID: 0, Char: "в"},
		{RowID: 3, ColID: 3, Char: "е"},
	}
	err := g.SubmitWord("p1", &testNewLetter, gapped)
	assert.ErrorIs(t, err, ErrWordHasGaps)
}

func TestSubmitWord_NewLetterNotInWord(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	addTestWord(t, "вол")

	// Word uses only existing board letters; new letter е(3,3) is absent.
	withoutNew := []Letter{
		{RowID: 2, ColID: 0, Char: "в"},
		{RowID: 2, ColID: 1, Char: "о"},
		{RowID: 2, ColID: 2, Char: "л"},
	}
	err := g.SubmitWord("p1", &testNewLetter, withoutNew)
	assert.ErrorIs(t, err, ErrNewLetterNotInWord)
}

func TestSubmitWord_WordAlreadyUsed(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	addTestWord(t, testWordStr)
	g.players[0].Words = append(g.players[0].Words, testWordStr)

	err := g.SubmitWord("p1", &testNewLetter, testWord)
	assert.ErrorIs(t, err, ErrWordAlreadyUsed)
}

func TestSubmitWord_WordAlreadyUsed_ByOtherPlayer(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	addTestWord(t, testWordStr)
	g.players[1].Words = append(g.players[1].Words, testWordStr) // p2 already used it

	err := g.SubmitWord("p1", &testNewLetter, testWord)
	assert.ErrorIs(t, err, ErrWordAlreadyUsed)
}

func TestSubmitWord_WordNotInDictionary(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()

	// "не" (negation particle) is not a Russian noun — not in the dictionary.
	// Path: н(2,3)→е(3,3), adjacent ✓, includes new letter е(3,3) ✓.
	notNoun := []Letter{
		{RowID: 2, ColID: 3, Char: "н"},
		{RowID: 3, ColID: 3, Char: "е"},
	}
	err := g.SubmitWord("p1", &testNewLetter, notNoun)
	assert.ErrorIs(t, err, ErrWordNotInDictionary)
}

func TestSubmitWord_LetterPlaceTaken(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	// "хол": word path х(2,0)→о(2,1)→л(2,2), all adjacent ✓
	// new letter is at (2,0) which is already occupied by "в".
	addTestWord(t, "хол")
	takenPos := Letter{RowID: 2, ColID: 0, Char: "х"}
	word := []Letter{
		{RowID: 2, ColID: 0, Char: "х"},
		{RowID: 2, ColID: 1, Char: "о"},
		{RowID: 2, ColID: 2, Char: "л"},
	}
	err := g.SubmitWord("p1", &takenPos, word)
	assert.ErrorIs(t, err, ErrLetterPlaceTaken)
}

func TestSubmitWord_LetterWrongPlace(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	// Row 0, col 2: needs letter at (1,2) below it, but row 1 is empty.
	addTestWord(t, "еф")
	badPos := Letter{RowID: 0, ColID: 2, Char: "е"}
	word := []Letter{
		{RowID: 0, ColID: 2, Char: "е"},
		{RowID: 1, ColID: 2, Char: "ф"},
	}
	err := g.SubmitWord("p1", &badPos, word)
	assert.ErrorIs(t, err, ErrWrongLetterPlace)
}

// ─── success cases ───────────────────────────────────────────────────────────

func TestSubmitWord_Success_UpdatesPlayerState(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	addTestWord(t, testWordStr)

	err := g.SubmitWord("p1", &testNewLetter, testWord)
	require.NoError(t, err)

	p1 := g.players[0]
	assert.Equal(t, len(testWord), p1.Score)
	assert.Equal(t, []string{testWordStr}, p1.Words)
}

func TestSubmitWord_Success_PlacesLetterOnBoard(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	addTestWord(t, testWordStr)

	require.NoError(t, g.SubmitWord("p1", &testNewLetter, testWord))

	placed := g.board.Table[testNewLetter.RowID][testNewLetter.ColID]
	require.NotNil(t, placed)
	assert.Equal(t, testNewLetter.Char, placed.Char)
}

func TestSubmitWord_Success_SendsMoveSubmittedEvent(t *testing.T) {
	n := &mockNotifier{}
	g := makeGameWithBoard(t, n, testBoardWord, "p1", "p2")
	defer g.cancelTimer()
	addTestWord(t, testWordStr)

	require.NoError(t, g.SubmitWord("p1", &testNewLetter, testWord))

	select {
	case ev := <-g.eventCh:
		assert.Equal(t, EventMoveSubmitted, ev)
	default:
		t.Fatal("expected EventMoveSubmitted in eventCh")
	}
}

// TestSubmitWord_Integration_TurnAdvances runs the full game loop and verifies
// that after a successful SubmitWord the turn passes to the next player.
func TestSubmitWord_Integration_TurnAdvances(t *testing.T) {
	n := &mockNotifier{}
	board, err := NewLettersTable(testBoardWord)
	require.NoError(t, err)
	players := makePlayers("p1", "p2")
	g := &Game{
		players:  players,
		board:    board,
		eventCh:  make(chan TurnEvent, 4),
		done:     make(chan struct{}),
		notifier: n,
		state:    StateWaitingForMove,
	}
	addTestWord(t, testWordStr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)
	assert.Equal(t, "p1", n.lastTurnStart())

	// Copy letter so each test run uses a fresh struct.
	nl := testNewLetter
	require.NoError(t, g.SubmitWord("p1", &nl, testWord))

	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)
}

// TestSubmitWord_Integration_SecondSubmitFailsWordAlreadyUsed verifies that
// the same word cannot be submitted in a subsequent turn.
func TestSubmitWord_Integration_SecondSubmitFailsWordAlreadyUsed(t *testing.T) {
	n := &mockNotifier{}
	board, err := NewLettersTable(testBoardWord)
	require.NoError(t, err)
	players := makePlayers("p1", "p2")
	g := &Game{
		players:  players,
		board:    board,
		eventCh:  make(chan TurnEvent, 4),
		done:     make(chan struct{}),
		notifier: n,
		state:    StateWaitingForMove,
	}
	addTestWord(t, testWordStr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.Run(ctx)

	require.Eventually(t, func() bool {
		return n.turnStartCount() >= 1
	}, time.Second, 5*time.Millisecond)

	// p1 submits the word.
	nl := testNewLetter
	require.NoError(t, g.SubmitWord("p1", &nl, testWord))

	// Wait for p2's turn.
	require.Eventually(t, func() bool {
		return n.lastTurnStart() == "p2"
	}, time.Second, 5*time.Millisecond)

	// p2 tries the same word — must be rejected.
	err = g.SubmitWord("p2", &nl, testWord)
	assert.ErrorIs(t, err, ErrWordAlreadyUsed)
}
