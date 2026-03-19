package tests

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rustwizard/balda/internal/notifier"
)

// ---- recording Sender (unit-test double) ----

type notifierCall struct {
	playerID string
	event    notifier.Event
}

type recordingSender struct {
	mu    sync.Mutex
	calls []notifierCall
}

func (r *recordingSender) Send(playerID string, event notifier.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, notifierCall{playerID, event})
}

func (r *recordingSender) last() notifierCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[len(r.calls)-1]
}

// ---- GameNotifier unit tests ----

func TestGameNotifier_NotifyTurnStart(t *testing.T) {
	s := &recordingSender{}
	n := notifier.New(s)

	n.NotifyTurnStart("p1")

	got := s.last()
	assert.Equal(t, "p1", got.playerID)
	assert.Equal(t, notifier.EventTurnStart, got.event.Type)
	assert.Nil(t, got.event.Payload)
}

func TestGameNotifier_NotifyKick(t *testing.T) {
	s := &recordingSender{}
	n := notifier.New(s)

	n.NotifyKick("p2")

	got := s.last()
	assert.Equal(t, "p2", got.playerID)
	assert.Equal(t, notifier.EventKick, got.event.Type)
	assert.Nil(t, got.event.Payload)
}

func TestGameNotifier_NotifyTimeout(t *testing.T) {
	tests := []struct {
		name        string
		consecutive int
		willKick    bool
	}{
		{"warning", 2, false},
		{"will kick", 3, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &recordingSender{}
			n := notifier.New(s)

			n.NotifyTimeout("p3", tc.consecutive, tc.willKick)

			got := s.last()
			assert.Equal(t, "p3", got.playerID)
			assert.Equal(t, notifier.EventTimeout, got.event.Type)

			p, ok := got.event.Payload.(notifier.TimeoutPayload)
			require.True(t, ok)
			assert.Equal(t, tc.consecutive, p.Consecutive)
			assert.Equal(t, tc.willKick, p.WillKick)
		})
	}
}

// ---- Redis integration tests ----

func newNotifierSender(t *testing.T) *notifier.RedisSender {
	t.Helper()
	ctx := context.Background()
	addr, cleanup := startRedis(ctx, t)
	t.Cleanup(cleanup)
	client := redis.NewClient(&redis.Options{Addr: addr})
	return notifier.NewRedisSender(client)
}

// subscribeReady subscribes and waits for the Redis confirmation before
// returning, preventing a race between Subscribe and Publish.
func subscribeReady(ctx context.Context, t *testing.T, sender *notifier.RedisSender, playerID string) *redis.PubSub {
	t.Helper()
	sub := sender.Subscribe(ctx, playerID)
	_, err := sub.Receive(ctx) // *redis.Subscription confirmation
	require.NoError(t, err)
	return sub
}

func receiveEvent(ctx context.Context, t *testing.T, sub *redis.PubSub) notifier.Event {
	t.Helper()
	msg, err := sub.ReceiveMessage(ctx)
	require.NoError(t, err)
	var event notifier.Event
	require.NoError(t, json.Unmarshal([]byte(msg.Payload), &event))
	return event
}

func TestRedisSender_NotifyTurnStart(t *testing.T) {
	ctx := context.Background()
	sender := newNotifierSender(t)
	n := notifier.New(sender)

	sub := subscribeReady(ctx, t, sender, "p1")
	defer sub.Close()

	n.NotifyTurnStart("p1")

	event := receiveEvent(ctx, t, sub)
	assert.Equal(t, notifier.EventTurnStart, event.Type)
	assert.Nil(t, event.Payload)
}

func TestRedisSender_NotifyKick(t *testing.T) {
	ctx := context.Background()
	sender := newNotifierSender(t)
	n := notifier.New(sender)

	sub := subscribeReady(ctx, t, sender, "p1")
	defer sub.Close()

	n.NotifyKick("p1")

	event := receiveEvent(ctx, t, sub)
	assert.Equal(t, notifier.EventKick, event.Type)
	assert.Nil(t, event.Payload)
}

func TestRedisSender_NotifyTimeout(t *testing.T) {
	ctx := context.Background()
	sender := newNotifierSender(t)
	n := notifier.New(sender)

	sub := subscribeReady(ctx, t, sender, "p1")
	defer sub.Close()

	n.NotifyTimeout("p1", 2, true)

	event := receiveEvent(ctx, t, sub)
	assert.Equal(t, notifier.EventTimeout, event.Type)

	// Payload arrives as map[string]any after JSON round-trip; re-decode it.
	b, err := json.Marshal(event.Payload)
	require.NoError(t, err)
	var p notifier.TimeoutPayload
	require.NoError(t, json.Unmarshal(b, &p))
	assert.Equal(t, 2, p.Consecutive)
	assert.True(t, p.WillKick)
}

// TestRedisSender_IsolatedChannels verifies that a message for player B
// is not delivered to player A.
func TestRedisSender_IsolatedChannels(t *testing.T) {
	ctx := context.Background()
	sender := newNotifierSender(t)
	n := notifier.New(sender)

	subA := subscribeReady(ctx, t, sender, "playerA")
	defer subA.Close()
	subB := subscribeReady(ctx, t, sender, "playerB")
	defer subB.Close()

	n.NotifyKick("playerB")

	eventB := receiveEvent(ctx, t, subB)
	assert.Equal(t, notifier.EventKick, eventB.Type)

	ctxTimeout, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, err := subA.ReceiveMessage(ctxTimeout)
	assert.Error(t, err, "playerA should not receive a message sent to playerB")
}

// TestRedisSender_OfflinePlayer verifies that publishing to a channel with
// no subscribers does not panic or block.
func TestRedisSender_OfflinePlayer(t *testing.T) {
	sender := newNotifierSender(t)
	n := notifier.New(sender)

	n.NotifyKick("offline")
}
