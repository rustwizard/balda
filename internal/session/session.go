// Package session implements user sessions
package session

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"log/slog"
)

const (
	keyPrefix    = "sessions:"
	sidKeyPrefix = "sessions:sid:"
)

var (
	ErrNotFound       = errors.New("session service: not found")
	ErrEmptySessionID = errors.New("session service: empty session id. set X-API-Session")
)

type Service struct {
	cfg     Config
	storage *redis.Client
}

func NewService(cfg Config) *Service {
	return &Service{
		cfg: cfg,
		storage: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DBNum,
			Username: cfg.Username,
		}),
	}
}

type User struct {
	Sid string
	UID int64
}

func (s *Service) Get(uid int64) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	val, err := s.storage.GetEx(ctx, keyPrefix+strconv.FormatInt(uid, 10), s.cfg.Expiration).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &User{Sid: val, UID: uid}, nil
}

// Refresh extends the TTL of the session identified by sid.
// Returns ErrNotFound if the session does not exist or has already expired.
func (s *Service) Refresh(sid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.storage.GetEx(ctx, sidKeyPrefix+sid, s.cfg.Expiration).Result()
	if errors.Is(err, redis.Nil) {
		return ErrNotFound
	}
	return err
}

func (s *Service) Save(u *User) error {
	if u.Sid == "" {
		slog.Error("sessions service: user session id not set", slog.Any("error", ErrEmptySessionID))
		return ErrEmptySessionID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uidStr := strconv.FormatInt(u.UID, 10)
	_, err := s.storage.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Set(ctx, keyPrefix+uidStr, u.Sid, s.cfg.Expiration)
		p.Set(ctx, sidKeyPrefix+u.Sid, uidStr, s.cfg.Expiration)
		return nil
	})
	return err
}

func (s *Service) Create(uid int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sid := uuid.New().String()
	uidStr := strconv.FormatInt(uid, 10)

	_, err := s.storage.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Set(ctx, keyPrefix+uidStr, sid, s.cfg.Expiration)
		p.Set(ctx, sidKeyPrefix+sid, uidStr, s.cfg.Expiration)
		return nil
	})
	if err != nil {
		return "", err
	}
	return sid, nil
}
