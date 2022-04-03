package session

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rs/zerolog/log"

	"github.com/savsgio/dictpool"
)

var (
	ErrNotFound       = errors.New("session service: not found")
	ErrEmptySessionID = errors.New("session service: empty session id. set X-API-Session")
)

type Service struct {
	cfg     Config
	storage *dictpool.Dict
	lock    sync.RWMutex
}

func NewService(cfg Config) *Service {
	return &Service{
		cfg:     cfg,
		storage: new(dictpool.Dict),
	}
}

type session struct {
	sessionID      string
	data           *dictpool.Dict
	expiration     time.Duration
	lastAccessTime int64
}

var sessionPool = &sync.Pool{
	New: func() interface{} {
		return new(session)
	},
}

func acquireSession() *session {
	return sessionPool.Get().(*session)
}

func releaseSession(item *session) {
	item.data.Reset()
	item.lastAccessTime = 0
	item.expiration = 0

	sessionPool.Put(item)
}

type User struct {
	Sid string
	UID int64
}

func (s *Service) Get(r *http.Request) (*User, error) {
	sid := r.Header.Get("X-API-Session")
	if sid == "" {
		log.Error().Msg("sessions service: x-api-session not set")
		return nil, ErrEmptySessionID
	}

	s.lock.RLock()
	defer s.lock.RUnlock()

	item, ok := s.storage.Get(sid).(*session)
	if !ok {
		return nil, ErrNotFound
	}

	return &User{
		Sid: item.sessionID,
		UID: item.data.Get("uid").(int64),
	}, nil
}

func (s *Service) Save(u *User) error {
	if u.Sid == "" {
		log.Error().Msg("sessions service: user session id not set")
		return ErrEmptySessionID
	}
	ss := acquireSession()
	ss.sessionID = u.Sid
	ss.lastAccessTime = time.Now().UnixNano()
	ss.expiration = s.cfg.Expiration
	ss.data = dictpool.AcquireDict()
	ss.data.Set("uid", u.UID)
	s.lock.Lock()
	defer s.lock.Unlock()

	s.storage.Set(u.Sid, ss)

	return nil
}

func (s *Service) Create(uid int64) string {
	ss := acquireSession()
	ss.sessionID = uuid.New().String()
	ss.lastAccessTime = time.Now().UnixNano()
	ss.expiration = s.cfg.Expiration
	ss.data = dictpool.AcquireDict()
	ss.data.Set("uid", uid)
	s.lock.Lock()
	defer s.lock.Unlock()

	s.storage.Set(ss.sessionID, ss)

	return ss.sessionID
}
