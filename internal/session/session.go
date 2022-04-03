package session

import (
	"errors"

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

//func (s *Service) Get(uid int64) (*User, error) {
//	s.lock.RLock()
//	defer s.lock.RUnlock()
//
//	item, ok := s.storage.Get(strconv.FormatInt(uid, 10)).(*session)
//	if !ok {
//		return nil, ErrNotFound
//	}
//
//	return &User{
//		Sid: item.sessionID,
//		UID: item.data.Get("uid").(int64),
//	}, nil
//}
//
//func (s *Service) Save(u *User) error {
//	if u.Sid == "" {
//		log.Error().Msg("sessions service: user session id not set")
//		return ErrEmptySessionID
//	}
//	ss := acquireSession()
//	ss.sessionID = u.Sid
//	ss.lastAccessTime = time.Now().UnixNano()
//	ss.expiration = s.cfg.Expiration
//	ss.data = dictpool.AcquireDict()
//	ss.data.Set("uid", u.UID)
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	s.storage.Set(strconv.FormatInt(u.UID, 10), ss)
//
//	return nil
//}
//
//func (s *Service) Create(uid int64) string {
//	ss := acquireSession()
//	ss.sessionID = uuid.New().String()
//	ss.lastAccessTime = time.Now().UnixNano()
//	ss.expiration = s.cfg.Expiration
//	ss.data = dictpool.AcquireDict()
//	ss.data.Set("uid", uid)
//	s.lock.Lock()
//	defer s.lock.Unlock()
//
//	s.storage.Set(ss.sessionID, ss)
//
//	return ss.sessionID
//}
