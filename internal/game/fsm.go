package game

type GameState int
type TurnEvent int

const (
	StateWaitingForMove GameState = iota
	StatePlayerTimedOut           // intermediate: notification sent, awaiting ACK or kick
	StateEndProposed              // current player proposed to end; timer paused, awaiting opponent
	StateGameOver
)

const (
	EventMoveSubmitted TurnEvent = iota
	EventTurnSkipped
	EventTurnTimeout
	EventAckTimeout    // player (or coordinator) acks the timeout; game continues
	EventKick          // explicit kick decision; game ends
	EventBoardFull     // board has no empty cells; game ends
	EventEndProposed   // current player proposes to end the game
	EventEndAccepted   // opponent accepts the end proposal
	EventEndRejected   // opponent rejects the end proposal; timer resumes
)

/*
## FSM Transition Table

┌─────────────────────┬────────────────────┬─────────────────────┐
│ State               │ Event              │ Next State          │
├─────────────────────┼────────────────────┼─────────────────────┤
│ WaitingForMove      │ MoveSubmitted      │ WaitingForMove      │
│ WaitingForMove      │ TurnSkipped        │ WaitingForMove      │
│ WaitingForMove      │ TurnTimeout        │ PlayerTimedOut      │
│ WaitingForMove      │ EndProposed        │ EndProposed         │
├─────────────────────┼────────────────────┼─────────────────────┤
│ PlayerTimedOut      │ AckTimeout         │ WaitingForMove      │
│ PlayerTimedOut      │ Kick               │ GameOver            │
├─────────────────────┼────────────────────┼─────────────────────┤
│ EndProposed         │ EndAccepted        │ GameOver            │
│ EndProposed         │ EndRejected        │ WaitingForMove      │
└─────────────────────┴────────────────────┴─────────────────────┘
*/

// transition table: (state, event) -> (nextState, action)
type transition struct {
	next   GameState
	action func(g *Game)
}

var fsmTable = map[GameState]map[TurnEvent]transition{
	StateWaitingForMove: {
		EventMoveSubmitted: {StateWaitingForMove, (*Game).onMoveAccepted},
		EventTurnSkipped:   {StateWaitingForMove, (*Game).onSkip},
		EventTurnTimeout:   {StatePlayerTimedOut, (*Game).onTurnTimeout},
		EventKick:          {StateGameOver, (*Game).onKick},
		EventBoardFull:     {StateGameOver, (*Game).onBoardFull},
		EventEndProposed:   {StateEndProposed, (*Game).onEndProposed},
	},
	StatePlayerTimedOut: {
		EventAckTimeout: {StateWaitingForMove, (*Game).onTimeoutAck},
		EventKick:       {StateGameOver, (*Game).onKick},
	},
	StateEndProposed: {
		EventEndAccepted: {StateGameOver, (*Game).onEndAccepted},
		EventEndRejected: {StateWaitingForMove, (*Game).onEndRejected},
	},
}
