package notifier

// EventType identifies the kind of game event.
type EventType string

const (
	EventTimeout   EventType = "timeout"
	EventKick      EventType = "kick"
	EventTurnStart EventType = "turn_start"
)

// Event is the transport-agnostic representation of a game notification.
type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload,omitempty"`
}

// TimeoutPayload carries details for EventTimeout.
type TimeoutPayload struct {
	Consecutive int  `json:"consecutive"`
	WillKick    bool `json:"will_kick"`
}

// Sender delivers events to a player. Implementations are responsible for
// transport (Redis Pub/Sub, WebSocket, in-process channel, …).
type Sender interface {
	Send(playerID string, event Event)
}

// GameNotifier implements game.Notifier using a Sender.
// It is the only place that knows how game events map to Event values.
type GameNotifier struct {
	sender Sender
}

func New(s Sender) *GameNotifier {
	return &GameNotifier{sender: s}
}

func (n *GameNotifier) NotifyTimeout(playerID string, consecutive int, willKick bool) {
	n.sender.Send(playerID, Event{
		Type:    EventTimeout,
		Payload: TimeoutPayload{Consecutive: consecutive, WillKick: willKick},
	})
}

func (n *GameNotifier) NotifyKick(playerID string) {
	n.sender.Send(playerID, Event{Type: EventKick})
}

func (n *GameNotifier) NotifyTurnStart(playerID string) {
	n.sender.Send(playerID, Event{Type: EventTurnStart})
}

// Noop is a no-op implementation of game.Notifier.
type Noop struct{}

func (n *Noop) NotifyTimeout(_ string, _ int, _ bool) {}
func (n *Noop) NotifyKick(_ string)                   {}
func (n *Noop) NotifyTurnStart(_ string)              {}
