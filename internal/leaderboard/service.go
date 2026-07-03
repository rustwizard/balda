package leaderboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rustwizard/balda/internal/storage"
)

const defaultLimit = 100

// Service builds leaderboards with Redis caching.
type Service struct {
	st  *storage.Balda
	rdb *redis.Client
	ttl time.Duration
}

// NewService creates a leaderboard service. rdb may be nil to disable caching.
func NewService(st *storage.Balda, rdb *redis.Client, ttl time.Duration) *Service {
	return &Service{st: st, rdb: rdb, ttl: ttl}
}

// Entry aliases storage.LeaderboardEntry to avoid leaking the storage type upward.
type Entry = storage.LeaderboardEntry

// Request holds validated leaderboard parameters.
type Request struct {
	Period string // week | month
	Sort   string // rating | exp
	Limit  int
}

// GetLeaderboard returns the top players for the requested period and sort order.
// It caches the result in Redis for the configured TTL.
func (s *Service) GetLeaderboard(ctx context.Context, req Request) ([]Entry, time.Time, error) {
	if req.Period != "week" && req.Period != "month" {
		return nil, time.Time{}, fmt.Errorf("invalid period %q", req.Period)
	}
	if req.Sort != "rating" && req.Sort != "exp" {
		req.Sort = "rating"
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = defaultLimit
	}

	now := time.Now().UTC()
	var periodStart time.Time
	switch req.Period {
	case "week":
		periodStart = now.AddDate(0, 0, -7)
	case "month":
		periodStart = now.AddDate(0, 0, -30)
	}

	key := cacheKey(req.Period, req.Sort, req.Limit)

	// Try cache first.
	if s.rdb != nil {
		cached, err := s.rdb.Get(ctx, key).Result()
		if err == nil {
			var entries []Entry
			if err := json.Unmarshal([]byte(cached), &entries); err == nil {
				return entries, now, nil
			}
			slog.Warn("leaderboard cache unmarshal failed", slog.Any("error", err))
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("leaderboard cache read failed", slog.Any("error", err))
		}
	}

	// Cache miss or Redis unavailable — query the database.
	entries, err := s.st.GetLeaderboard(ctx, periodStart, req.Sort, req.Limit)
	if err != nil {
		return nil, now, err
	}

	// Best-effort cache write.
	if s.rdb != nil {
		data, err := json.Marshal(entries)
		if err == nil {
			if err := s.rdb.Set(ctx, key, data, s.ttl).Err(); err != nil {
				slog.Error("leaderboard cache write failed", slog.Any("error", err))
			}
		}
	}

	return entries, now, nil
}

func cacheKey(period, sort string, limit int) string {
	return fmt.Sprintf("leaderboard:%s:%s:%d", period, sort, limit)
}
