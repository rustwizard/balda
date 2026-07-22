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

// transition table: (state, event) -> action. The action runs under g.mu and
// returns the next state, so a state-dependent decision (e.g. kick only after
// the third consecutive skip) is committed atomically with the action itself.
type transition struct {
	action func(g *Game) GameState
}

var fsmTable = map[GameState]map[TurnEvent]transition{
	StateWaitingForMove: {
		EventMoveSubmitted: {(*Game).onMoveAccepted},
		EventTurnSkipped:   {(*Game).onSkip},
		EventTurnTimeout:   {(*Game).onTurnTimeout},
		EventKick:          {(*Game).onKick},
		EventGameFinished:  {(*Game).onGameFinished},
		EventEndProposed:   {(*Game).onEndProposed},
	},
	StatePlayerTimedOut: {
		EventAckTimeout: {(*Game).onTimeoutAck},
		EventKick:       {(*Game).onKick},
	},
	StateEndProposed: {
		EventEndAccepted: {(*Game).onEndAccepted},
		EventEndRejected: {(*Game).onEndRejected},
	},
}
