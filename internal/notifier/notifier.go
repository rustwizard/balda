package notifier

// Nooper is a no-op implementation of game.Notifier.
type Noop struct{}

func (n *Noop) NotifyTimeout(_ string, _ int, _ bool) {}
func (n *Noop) NotifyKick(_ string)                   {}
func (n *Noop) NotifyTurnStart(_ string)              {}
