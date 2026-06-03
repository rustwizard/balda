// Package presence tracks whether a player is currently online.
// A player is considered online as long as they send pings within the configured TTL.
// This is a game mechanic (absence detection), independent of authentication.
package presence

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "presence:"

type Service struct {
	cfg Config
	rdb *redis.Client
}

func NewService(cfg Config, rdb *redis.Client) *Service {
	return &Service{cfg: cfg, rdb: rdb}
}

// Refresh marks the player as online, resetting the absence timer.
func (s *Service) Refresh(ctx context.Context, uid int64) error {
	return s.rdb.Set(ctx, s.key(uid), 1, s.cfg.TTL).Err()
}

// IsOnline reports whether the player has pinged within the TTL window.
func (s *Service) IsOnline(ctx context.Context, uid int64) bool {
	n, err := s.rdb.Exists(ctx, s.key(uid)).Result()
	return err == nil && n > 0
}

// Remove deletes the presence key immediately (e.g. on game end or logout).
func (s *Service) Remove(ctx context.Context, uid int64) error {
	return s.rdb.Del(ctx, s.key(uid)).Err()
}

// TTL returns the configured absence window.
func (s *Service) TTL() time.Duration {
	return s.cfg.TTL
}

func (s *Service) key(uid int64) string {
	return keyPrefix + strconv.FormatInt(uid, 10)
}
