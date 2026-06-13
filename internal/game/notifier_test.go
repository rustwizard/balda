package game

import (
	"testing"
	"time"
)

type recordingNotifier struct {
	turnStarts []string
	timeouts   []string
	skips      []string
}

func (r *recordingNotifier) NotifyTimeout(playerID string, _ int, _ bool) {
	r.timeouts = append(r.timeouts, playerID)
}

func (r *recordingNotifier) NotifySkip(playerID string, _ int, _ bool) {
	r.skips = append(r.skips, playerID)
}

func (r *recordingNotifier) NotifyKick(_ string) {}

func (r *recordingNotifier) NotifyGameFinished() {}

func (r *recordingNotifier) NotifyTurnStart(playerID string) {
	r.turnStarts = append(r.turnStarts, playerID)
}

func (r *recordingNotifier) NotifyEndProposed(_ string) {}

func (r *recordingNotifier) NotifyEndAccepted() {}

func (r *recordingNotifier) NotifyEndRejected(_ time.Duration) {}

func TestCompositeNotifier_ForwardsAllEvents(t *testing.T) {
	a := &recordingNotifier{}
	b := &recordingNotifier{}
	c := NewCompositeNotifier(a, b)

	c.NotifyTurnStart("p1")
	c.NotifyTimeout("p1", 1, false)
	c.NotifySkip("p2", 1, false)
	c.NotifyKick("p1")
	c.NotifyGameFinished()
	c.NotifyEndProposed("p1")
	c.NotifyEndAccepted()
	c.NotifyEndRejected(5 * time.Second)

	if len(a.turnStarts) != 1 || a.turnStarts[0] != "p1" {
		t.Errorf("expected a to receive turn start for p1, got %v", a.turnStarts)
	}
	if len(b.turnStarts) != 1 || b.turnStarts[0] != "p1" {
		t.Errorf("expected b to receive turn start for p1, got %v", b.turnStarts)
	}
	if len(a.timeouts) != 1 || a.timeouts[0] != "p1" {
		t.Errorf("expected a to receive timeout for p1, got %v", a.timeouts)
	}
	if len(b.skips) != 1 || b.skips[0] != "p2" {
		t.Errorf("expected b to receive skip for p2, got %v", b.skips)
	}
}
