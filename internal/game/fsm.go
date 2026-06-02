package game

type GameState int
type TurnEvent int

func (e TurnEvent) String() string {
	switch e {
	case EventMoveSubmitted:
		return "MoveSubmitted"
	case EventTurnSkipped:
		return "TurnSkipped"
	case EventTurnTimeout:
		return "TurnTimeout"
	case EventAckTimeout:
		return "AckTimeout"
	case EventKick:
		return "Kick"
	case EventGameFinished:
		return "GameFinished"
	case EventEndProposed:
		return "EndProposed"
	case EventEndAccepted:
		return "EndAccepted"
	case EventEndRejected:
		return "EndRejected"
	default:
		return "Unknown"
	}
}

func (s GameState) String() string {
	switch s {
	case StateWaitingForMove:
		return "WaitingForMove"
	case StatePlayerTimedOut:
		return "PlayerTimedOut"
	case StateEndProposed:
		return "EndProposed"
	case StateGameOver:
		return "GameOver"
	default:
		return "Unknown"
	}
}

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
	EventAckTimeout   // player (or coordinator) acks the timeout; game continues
	EventKick         // explicit kick decision; game ends
	EventGameFinished // board full or no moves left; game ends naturally
	EventEndProposed  // current player proposes to end the game
	EventEndAccepted  // opponent accepts the end proposal
	EventEndRejected  // opponent rejects the end proposal; timer resumes
)

/*
## FSM Transition Table

┌─────────────────────┬────────────────────┬─────────────────────┐
│ State               │ Event              │ Next State          │
├─────────────────────┼────────────────────┼─────────────────────┤
│ WaitingForMove      │ MoveSubmitted      │ WaitingForMove      │
│ WaitingForMove      │ TurnSkipped        │ WaitingForMove      │
│ WaitingForMove      │ TurnTimeout        │ PlayerTimedOut      │
│ WaitingForMove      │ GameFinished       │ GameOver            │
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
		EventGameFinished:  {StateGameOver, (*Game).onGameFinished},
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
