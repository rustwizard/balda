package session

import (
	"time"

	"github.com/spf13/pflag"
)

const defaultExpiration = 30 * time.Second

type Config struct {
	Expiration time.Duration
}

func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix += "."
	}

	f := pflag.NewFlagSet("", pflag.PanicOnError)
	f.DurationVar(&c.Expiration, "expiration", defaultExpiration, "session expiration time in seconds")
	return f
}
