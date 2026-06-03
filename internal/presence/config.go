package presence

import (
	"time"

	"github.com/spf13/pflag"
)

const defaultTTL = 30 * time.Second

type Config struct {
	TTL time.Duration
}

func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix += "."
	}
	f := pflag.NewFlagSet("", pflag.PanicOnError)
	f.DurationVar(&c.TTL, prefix+"ttl", defaultTTL, "game presence TTL — player considered absent after this duration without a ping")
	return f
}
