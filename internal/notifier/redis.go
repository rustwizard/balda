package notifier

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	channelPrefix  = "notify:"
	publishTimeout = 2 * time.Second
)

// RedisSender implements Sender via Redis Pub/Sub.
// Each player is assigned a dedicated channel "notify:{playerID}".
type RedisSender struct {
	client *redis.Client
}

func NewRedisSender(client *redis.Client) *RedisSender {
	return &RedisSender{client: client}
}

// Send publishes event to the player's channel. Non-blocking — uses a short
// timeout so the game loop is never delayed by a slow Redis connection.
func (r *RedisSender) Send(playerID string, event Event) {
	b, err := json.Marshal(event)
	if err != nil {
		slog.Error("notifier: marshal event", slog.Any("error", err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	if err := r.client.Publish(ctx, channelPrefix+playerID, b).Err(); err != nil {
		slog.Error("notifier: publish event",
			slog.String("playerID", playerID),
			slog.Any("error", err),
		)
	}
}

// Subscribe returns a Pub/Sub handle for the player's channel.
// The caller is responsible for closing it when done.
func (r *RedisSender) Subscribe(ctx context.Context, playerID string) *redis.PubSub {
	return r.client.Subscribe(ctx, channelPrefix+playerID)
}
