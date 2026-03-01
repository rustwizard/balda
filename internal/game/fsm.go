package game

type GameState int
type TurnEvent int

const (
	StateWaitingForMove GameState = iota
	StatePlayerTimedOut           // intermediate: notification sent, awaiting ACK or kick
	StateGameOver
)

const (
	EventMoveSubmitted TurnEvent = iota
	EventTurnSkipped
	EventTurnTimeout
	EventAckTimeout // player (or coordinator) acks the timeout; game continues
	EventKick       // explicit kick decision; game ends
)

/*
## FSM Transition Table

┌─────────────────────┬────────────────────┬─────────────────────┐
│ State               │ Event              │ Next State          │
├─────────────────────┼────────────────────┼─────────────────────┤
│ WaitingForMove      │ MoveSubmitted      │ WaitingForMove      │
│ WaitingForMove      │ TurnSkipped        │ WaitingForMove      │
│ WaitingForMove      │ TurnTimeout        │ PlayerTimedOut      │
├─────────────────────┼────────────────────┼─────────────────────┤
│ PlayerTimedOut      │ AckTimeout         │ WaitingForMove      │
│ PlayerTimedOut      │ Kick               │ GameOver            │
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
	},
	StatePlayerTimedOut: {
		EventAckTimeout: {StateWaitingForMove, (*Game).onTimeoutAck},
		EventKick:       {StateGameOver, (*Game).onKick},
	},
}
