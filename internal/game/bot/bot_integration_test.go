package bot

import (
	"context"
	"testing"
	"time"

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

	// It's the human's turn (index 0). Find and submit a valid first move
	// using the same engine/strategy the bot uses.
	humanEngine := NewEngine(NewRandomValidStrategy(game.Dict))
	moveLetter, movePath, err := humanEngine.MakeMove(g.BoardSnapshot(), g.UsedWords())
	if err != nil {
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
