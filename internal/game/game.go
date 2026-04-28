// Package game implement Balda game logic
package game

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotYourTurn         = errors.New("game: not your turn")
	ErrWrongState          = errors.New("game: wrong state for this action")
	ErrWordHasGaps         = errors.New("game: word path has gaps between letters")
	ErrDuplicateCell       = errors.New("game: word path uses the same cell twice")
	ErrNewLetterNotInWord  = errors.New("game: new letter must be included in the word")
	ErrWordAlreadyUsed     = errors.New("game: word already used")
	ErrWordNotInDictionary = errors.New("game: word not found in dictionary")
	ErrWordIsInitialWord   = errors.New("game: word is the initial board word")
	ErrNotOpponent         = errors.New("game: only the opponent can respond to an end proposal")
)

const (
	TurnDuration           = 60 * time.Second
	MaxConsecutiveTimeouts = 3
	MaxConsecutiveSkips    = 3
)

// Option configures a Game at construction time.
type Option func(*Game)

// WithTurnDuration overrides the per-turn timer duration.
func WithTurnDuration(d time.Duration) Option {
	return func(g *Game) { g.turnDuration = d }
}

type Turn struct {
	PlayerID  string
	StartedAt time.Time
	timer     *time.Timer
}

type Notifier interface {
	NotifyTimeout(playerID string, consecutive int, willKick bool)
	NotifySkip(playerID string, consecutive int, willEnd bool)
	NotifyKick(playerID string)
	NotifyBoardFull()
	NotifyTurnStart(playerID string)
	NotifyEndProposed(proposerID string)
	NotifyEndAccepted()
	NotifyEndRejected(remainingTurn time.Duration)
}

type Player struct {
	ID                  string
	Exp                 int
	Score               int
	Words               []string
	ConsecutiveTimeouts int
	ConsecutiveSkips    int
	Kicked              bool
}

type Game struct {
	mu                  sync.Mutex
	state               GameState
	players             []*Player
	board               *LettersTable
	current             int
	turn                *Turn
	eventCh             chan TurnEvent
	done                chan struct{}
	notifier            Notifier
	turnDuration        time.Duration // 0 means use TurnDuration constant
	pausedTurnRemaining time.Duration // remaining turn time when paused for end proposal
}

func (g *Game) СheckWordExistence(word string) bool {
	if _, ok := Dict.Definition[normalizeWord(word)]; !ok {
		return false
	}
	return true
}

func MakeWord(word []Letter) string {
	var w string
	for _, v := range word {
		w += v.Char
	}
	return normalizeWord(w)
}

// normalizeWord replaces ё/Ё with е/Е so that words differing only by
// this letter are treated as identical.
func normalizeWord(word string) string {
	word = strings.ReplaceAll(word, "ё", "е")
	word = strings.ReplaceAll(word, "Ё", "Е")
	return word
}

// HasDuplicateCells reports whether any cell position appears more than once in the path.
func HasDuplicateCells(word []Letter) bool {
	seen := make(map[[2]uint8]struct{}, len(word))
	for _, l := range word {
		key := [2]uint8{l.RowID, l.ColID}
		if _, ok := seen[key]; ok {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

// GapsBetweenLetters reports whether there are gaps between consecutive letters
// in the word path. Each consecutive pair must occupy board-adjacent cells
// (sharing an edge horizontally or vertically; Manhattan distance == 1).
// Returns true when the path is discontinuous (invalid word placement).
func GapsBetweenLetters(word []Letter) bool {
	if len(word) < 2 {
		return true
	}
	for i := 1; i < len(word); i++ {
		prev, curr := word[i-1], word[i]
		rowDiff := int(curr.RowID) - int(prev.RowID)
		colDiff := int(curr.ColID) - int(prev.ColID)
		if rowDiff < 0 {
			rowDiff = -rowDiff
		}
		if colDiff < 0 {
			colDiff = -colDiff
		}
		if rowDiff+colDiff != 1 {
			return true
		}
	}
	return false
}

func NewGame(players []*Player, n Notifier, opts ...Option) (*Game, error) {
	board, err := NewLettersTable(Dict.RandomFiveLetterWord())
	if err != nil {
		return nil, err
	}
	g := &Game{
		players:  players,
		eventCh:  make(chan TurnEvent, 4), // buffered: timer + auto-kick can queue simultaneously
		done:     make(chan struct{}),
		board:    board,
		notifier: n,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

func (g *Game) Run(ctx context.Context) {
	g.startTurn()
	for {
		select {
		case <-ctx.Done():
			g.shutdown()
			return
		case ev, ok := <-g.eventCh:
			if !ok {
				return
			}
			g.mu.Lock()
			g.dispatch(ev)
			terminal := g.state == StateGameOver
			g.mu.Unlock()

			if terminal {
				g.shutdown()
				return
			}
		}
	}
}

func (g *Game) dispatch(ev TurnEvent) {
	t, ok := fsmTable[g.state][ev]
	if !ok {
		slog.Info("game dispatch", slog.Any("ignored event", ev), "state", g.state)
		return
	}
	t.action(g)      // action runs first; may queue follow-up events
	g.state = t.next // then commit the state
}

// --- WaitingForMove actions ---

func (g *Game) onMoveAccepted() {
	p := g.currentPlayer()
	p.ConsecutiveTimeouts = 0
	p.ConsecutiveSkips = 0
	g.cancelTimer()
	g.advanceTurn()
}

func (g *Game) onSkip() {
	p := g.currentPlayer()
	p.ConsecutiveTimeouts = 0
	p.ConsecutiveSkips++
	willEnd := p.ConsecutiveSkips >= MaxConsecutiveSkips
	g.notifier.NotifySkip(p.ID, p.ConsecutiveSkips, willEnd)
	g.cancelTimer()
	if willEnd {
		g.eventCh <- EventKick
		return
	}
	g.advanceTurn()
}

func (g *Game) onTurnTimeout() {
	p := g.currentPlayer()
	p.ConsecutiveTimeouts++

	willKick := p.ConsecutiveTimeouts >= MaxConsecutiveTimeouts

	// Phase 1: notify. Non-blocking — notifier must not call back into Game synchronously.
	g.notifier.NotifyTimeout(p.ID, p.ConsecutiveTimeouts, willKick)

	if willKick {
		// Auto-queue the kick; coordinator can also send it explicitly.
		// Sending into buffered channel from within the run loop is safe.
		g.eventCh <- EventKick
	}
	// If not kicking, we wait for an external EventAckTimeout before advancing.
}

// --- PlayerTimedOut actions ---

func (g *Game) onTimeoutAck() {
	// Player acknowledged the timeout notification; resume play.
	g.advanceTurn()
}

func (g *Game) onKick() {
	p := g.currentPlayer()
	p.Kicked = true
	g.notifier.NotifyKick(p.ID)
	// StateGameOver committed by dispatch; shutdown follows in Run.
}

func (g *Game) onBoardFull() {
	g.notifier.NotifyBoardFull()
}

// --- EndProposed actions ---

func (g *Game) onEndProposed() {
	g.cancelTimer()
	elapsed := time.Since(g.turn.StartedAt)
	d := g.turnDuration
	if d == 0 {
		d = TurnDuration
	}
	remaining := d - elapsed
	// Guarantee the proposer gets at least 1/6 of a full turn after rejection
	// (≥10 s on a standard 60 s turn) so the game never resumes with a near-zero timer.
	if minRemaining := d / 6; remaining < minRemaining {
		remaining = minRemaining
	}
	g.pausedTurnRemaining = remaining
	g.notifier.NotifyEndProposed(g.currentPlayer().ID)
}

func (g *Game) onEndAccepted() {
	g.notifier.NotifyEndAccepted()
}

func (g *Game) onEndRejected() {
	remaining := g.pausedTurnRemaining
	g.pausedTurnRemaining = 0
	p := g.currentPlayer()
	g.turn = &Turn{
		PlayerID:  p.ID,
		StartedAt: time.Now(),
		timer: time.AfterFunc(remaining, func() {
			select {
			case g.eventCh <- EventTurnTimeout:
			case <-g.done:
			}
		}),
	}
	g.notifier.NotifyEndRejected(remaining)
}

func (g *Game) currentPlayer() *Player { return g.players[g.current] }

func (g *Game) advanceTurn() {
	g.current = (g.current + 1) % len(g.players)
	g.startTurn()
}

func (g *Game) startTurn() {
	d := g.turnDuration
	if d == 0 {
		d = TurnDuration
	}
	p := g.currentPlayer()
	g.turn = &Turn{
		PlayerID:  p.ID,
		StartedAt: time.Now(),
		timer: time.AfterFunc(d, func() {
			select {
			case g.eventCh <- EventTurnTimeout:
			case <-g.done:
			}
		}),
	}
	g.notifier.NotifyTurnStart(p.ID)
}

func (g *Game) cancelTimer() {
	if g.turn != nil && g.turn.timer != nil {
		g.turn.timer.Stop()
	}
}

// SubmitWord validates and applies a player's move: places newLetter on the
// board and records the word formed by the given sequence of board letters.
// The sequence must include the new letter, be connected (adjacent cells),
// be present in the dictionary, and not have been played before.
// On success the turn passes to the next player.
func (g *Game) SubmitWord(playerID string, newLetter *Letter, word []Letter) error {
	g.mu.Lock()

	if g.state != StateWaitingForMove {
		g.mu.Unlock()
		return ErrWrongState
	}
	if g.currentPlayer().ID != playerID {
		g.mu.Unlock()
		return ErrNotYourTurn
	}
	if HasDuplicateCells(word) {
		g.mu.Unlock()
		return ErrDuplicateCell
	}
	if GapsBetweenLetters(word) {
		g.mu.Unlock()
		return ErrWordHasGaps
	}

	newLetterUsed := false
	for _, l := range word {
		if l.RowID == newLetter.RowID && l.ColID == newLetter.ColID {
			newLetterUsed = true
			break
		}
	}
	if !newLetterUsed {
		g.mu.Unlock()
		return ErrNewLetterNotInWord
	}

	wordStr := MakeWord(word)
	for _, p := range g.players {
		if slices.Contains(p.Words, wordStr) {
			g.mu.Unlock()
			return ErrWordAlreadyUsed
		}
	}
	if wordStr == g.board.InitialWord() {
		g.mu.Unlock()
		return ErrWordIsInitialWord
	}
	if !g.СheckWordExistence(wordStr) {
		g.mu.Unlock()
		return ErrWordNotInDictionary
	}
	normalizedLetter := *newLetter
	normalizedLetter.Char = normalizeWord(normalizedLetter.Char)
	if err := g.board.PutLetterOnTable(&normalizedLetter); err != nil {
		g.mu.Unlock()
		return err
	}

	p := g.currentPlayer()
	p.Words = append(p.Words, wordStr)
	p.Score += len(word)

	boardFull := g.board.IsFull()
	g.mu.Unlock()

	ev := EventMoveSubmitted
	if boardFull {
		ev = EventBoardFull
	}
	select {
	case g.eventCh <- ev:
	case <-g.done:
	}
	return nil
}

// NewGameWithWord creates a Game with a specific initial board word instead of a random one.
func NewGameWithWord(players []*Player, initWord string, n Notifier, opts ...Option) (*Game, error) {
	board, err := NewLettersTable(initWord)
	if err != nil {
		return nil, err
	}
	g := &Game{
		players:  players,
		eventCh:  make(chan TurnEvent, 4),
		done:     make(chan struct{}),
		board:    board,
		notifier: n,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

// Board returns the game's letter board.
func (g *Game) Board() *LettersTable {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.board
}

// PlayerState holds a player's UID and current score for external consumers.
type PlayerState struct {
	UID        string
	Exp        int
	Score      int
	WordsCount int
	Words      []string
}

// PlayerScores returns a snapshot of each player's score and word count.
func (g *Game) PlayerScores() []PlayerState {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]PlayerState, len(g.players))
	for i, p := range g.players {
		words := make([]string, len(p.Words))
		copy(words, p.Words)
		out[i] = PlayerState{UID: p.ID, Exp: p.Exp, Score: p.Score, WordsCount: len(p.Words), Words: words}
	}
	return out
}

// CurrentPlayerID returns the ID of the player whose turn it currently is.
func (g *Game) CurrentPlayerID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.currentPlayer().ID
}

// MoveNumber returns the total number of accepted moves across all players.
func (g *Game) MoveNumber() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	total := 0
	for _, p := range g.players {
		total += len(p.Words)
	}
	return total
}

// Skip signals that playerID passes their turn without placing a letter.
func (g *Game) Skip(playerID string) error {
	g.mu.Lock()
	if g.state != StateWaitingForMove {
		g.mu.Unlock()
		return ErrWrongState
	}
	if g.currentPlayer().ID != playerID {
		g.mu.Unlock()
		return ErrNotYourTurn
	}
	g.mu.Unlock()
	select {
	case g.eventCh <- EventTurnSkipped:
	case <-g.done:
	}
	return nil
}

// AckTimeout acknowledges a timeout notification; the game resumes with the next player's turn.
func (g *Game) AckTimeout() {
	select {
	case g.eventCh <- EventAckTimeout:
	case <-g.done:
	}
}

// Kick forcibly removes the current player and ends the game.
func (g *Game) Kick() {
	select {
	case g.eventCh <- EventKick:
	case <-g.done:
	}
}

func (g *Game) shutdown() {
	g.cancelTimer()
	// Safe to call multiple times: sync.Once or closed-channel idiom
	select {
	case <-g.done:
	default:
		close(g.done)
	}
}

func (g *Game) AddWordToCurrentPlayer(word string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.currentPlayer().Words = append(g.currentPlayer().Words, word)
}

func (g *Game) IsTakenWord(word string) bool {
	word = normalizeWord(word)
	for _, player := range g.players {
		for _, pword := range player.Words {
			if normalizeWord(pword) == word {
				return true
			}
		}
	}
	return false
}

// ProposeEnd signals that playerID wants to end the game (e.g. no valid moves).
// Only the current player may call this when the game is in WaitingForMove state.
func (g *Game) ProposeEnd(playerID string) error {
	g.mu.Lock()
	if g.state != StateWaitingForMove {
		g.mu.Unlock()
		return ErrWrongState
	}
	if g.currentPlayer().ID != playerID {
		g.mu.Unlock()
		return ErrNotYourTurn
	}
	g.mu.Unlock()
	select {
	case g.eventCh <- EventEndProposed:
	case <-g.done:
	}
	return nil
}

// AcceptEnd signals that playerID (the opponent) accepts the end-game proposal.
func (g *Game) AcceptEnd(playerID string) error {
	g.mu.Lock()
	if g.state != StateEndProposed {
		g.mu.Unlock()
		return ErrWrongState
	}
	if g.currentPlayer().ID == playerID {
		g.mu.Unlock()
		return ErrNotOpponent
	}
	g.mu.Unlock()
	select {
	case g.eventCh <- EventEndAccepted:
	case <-g.done:
	}
	return nil
}

// RejectEnd signals that playerID (the opponent) rejects the end-game proposal.
func (g *Game) RejectEnd(playerID string) error {
	g.mu.Lock()
	if g.state != StateEndProposed {
		g.mu.Unlock()
		return ErrWrongState
	}
	if g.currentPlayer().ID == playerID {
		g.mu.Unlock()
		return ErrNotOpponent
	}
	g.mu.Unlock()
	select {
	case g.eventCh <- EventEndRejected:
	case <-g.done:
	}
	return nil
}

// Done returns a channel that is closed when the game has finished running.
// Safe to call concurrently. Follows the same idiom as context.Context.Done().
func (g *Game) Done() <-chan struct{} {
	return g.done
}
