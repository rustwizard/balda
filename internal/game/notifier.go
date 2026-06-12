package game

import "time"

// CompositeNotifier forwards every notification to multiple notifiers.
// It is used to combine human-facing real-time events (gamecoord.Coordinator)
// with a server-side bot driver.
type CompositeNotifier struct {
	notifiers []Notifier
}

// NewCompositeNotifier creates a notifier that broadcasts to all provided notifiers.
func NewCompositeNotifier(notifiers ...Notifier) *CompositeNotifier {
	return &CompositeNotifier{notifiers: notifiers}
}

func (c *CompositeNotifier) NotifyTimeout(playerID string, consecutive int, willKick bool) {
	for _, n := range c.notifiers {
		n.NotifyTimeout(playerID, consecutive, willKick)
	}
}

func (c *CompositeNotifier) NotifySkip(playerID string, consecutive int, willEnd bool) {
	for _, n := range c.notifiers {
		n.NotifySkip(playerID, consecutive, willEnd)
	}
}

func (c *CompositeNotifier) NotifyKick(playerID string) {
	for _, n := range c.notifiers {
		n.NotifyKick(playerID)
	}
}

func (c *CompositeNotifier) NotifyGameFinished() {
	for _, n := range c.notifiers {
		n.NotifyGameFinished()
	}
}

func (c *CompositeNotifier) NotifyTurnStart(playerID string) {
	for _, n := range c.notifiers {
		n.NotifyTurnStart(playerID)
	}
}

func (c *CompositeNotifier) NotifyEndProposed(proposerID string) {
	for _, n := range c.notifiers {
		n.NotifyEndProposed(proposerID)
	}
}

func (c *CompositeNotifier) NotifyEndAccepted() {
	for _, n := range c.notifiers {
		n.NotifyEndAccepted()
	}
}

func (c *CompositeNotifier) NotifyEndRejected(remainingTurn time.Duration) {
	for _, n := range c.notifiers {
		n.NotifyEndRejected(remainingTurn)
	}
}
