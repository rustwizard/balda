package notifier

import "github.com/redis/go-redis/v9"

// EventType identifies the kind of game event.
type EventType string

const (
	EventTimeout   EventType = "timeout"
	EventKick      EventType = "kick"
	EventTurnStart EventType = "turn_start"
	EventSkip      EventType = "skip"
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

// SkipPayload carries details for EventSkip.
type SkipPayload struct {
	Consecutive int  `json:"consecutive"`
	WillEnd     bool `json:"will_end"`
}

// Sender delivers events to a player. Implementations are responsible for
// transport (Redis Pub/Sub, WebSocket, in-process channel, …).
type Sender interface {
	Send(playerID string, event Event)
}

// Option configures a GameNotifier.
type Option func(*GameNotifier)

// WithSender sets an arbitrary Sender implementation.
func WithSender(s Sender) Option {
	return func(n *GameNotifier) { n.sender = s }
}

// WithRedisSender creates a RedisSender from client and sets it.
func WithRedisSender(client *redis.Client) Option {
	return func(n *GameNotifier) { n.sender = NewRedisSender(client) }
}

// GameNotifier implements game.Notifier using a Sender.
// It is the only place that knows how game events map to Event values.
type GameNotifier struct {
	sender Sender
}

// New returns a GameNotifier. Without options it uses a no-op Sender.
func New(opts ...Option) *GameNotifier {
	n := &GameNotifier{sender: noopSender{}}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

type noopSender struct{}

func (noopSender) Send(_ string, _ Event) {}

func (n *GameNotifier) NotifyTimeout(playerID string, consecutive int, willKick bool) {
	n.sender.Send(playerID, Event{
		Type:    EventTimeout,
		Payload: TimeoutPayload{Consecutive: consecutive, WillKick: willKick},
	})
}

func (n *GameNotifier) NotifySkip(playerID string, consecutive int, willEnd bool) {
	n.sender.Send(playerID, Event{
		Type:    EventSkip,
		Payload: SkipPayload{Consecutive: consecutive, WillEnd: willEnd},
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
func (n *Noop) NotifySkip(_ string, _ int, _ bool)    {}
func (n *Noop) NotifyKick(_ string)                   {}
func (n *Noop) NotifyTurnStart(_ string)              {}
