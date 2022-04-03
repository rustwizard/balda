package session

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/rs/zerolog/log"

	"github.com/go-redis/redis/v8"
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
		// TODO: move to config
		storage: redis.NewClient(&redis.Options{
			Addr:     "127.0.0.1:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
	}
}

type User struct {
	Sid string
	UID int64
}

func (s *Service) Get(uid int64) (*User, error) {
	var user *User
	ctx, cacnel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cacnel()

	val, err := s.storage.Get(ctx, strconv.FormatInt(uid, 10)).Result()
	if err != nil {
		return user, err
	}
	user.UID = uid
	user.Sid = val
	return user, nil
}

func (s *Service) Save(u *User) error {
	if u.Sid == "" {
		log.Error().Msg("sessions service: user session id not set")
		return ErrEmptySessionID
	}
	ctx, cacnel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cacnel()

	err := s.storage.Set(ctx, strconv.FormatInt(u.UID, 10), u.Sid, s.cfg.Expiration).Err()
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) Create(uid int64) (string, error) {
	ctx, cacnel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cacnel()
	sid := uuid.New().String()
	err := s.storage.Set(ctx, strconv.FormatInt(uid, 10), sid, s.cfg.Expiration).Err()
	if err != nil {
		return sid, err
	}

	return sid, nil
}
