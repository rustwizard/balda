package session

import (
	"time"

	"github.com/spf13/pflag"
)

const defaultExpiration = 30 * time.Second

type Config struct {
	Addr       string
	Expiration time.Duration
	Username   string
	Password   string
	DBNum      int
}

func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix += "."
	}

	f := pflag.NewFlagSet("", pflag.PanicOnError)
	f.DurationVar(&c.Expiration, prefix+"expiration", defaultExpiration, "session expiration time in seconds")
	f.StringVar(&c.Addr, prefix+"addr", "127.0.0.1:6379", "redis host:port")
	f.StringVar(&c.Username, prefix+"username", "", "redis db username")
	f.StringVar(&c.Password, prefix+"password", "", "redis db password")
	f.IntVar(&c.DBNum, prefix+"db_num", 0, "redis db num")
	return f
}
