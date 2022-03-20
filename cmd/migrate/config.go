package migrate

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Config struct {
	Addr     string
	User     string
	Password string
	Database string
}

func (c *Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("addr should be non empty")
	}
	if c.Database == "" {
		return fmt.Errorf("database should be non empty")
	}

	return nil
}

func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix = prefix + "."
	}

	f := pflag.NewFlagSet("postgres", pflag.PanicOnError)
	f.StringVar(&c.Addr, prefix+"host", "", "postgres host:port string. example: [127.0.0.1:5432]")
	f.StringVar(&c.Database, prefix+"database", "", "postgres database name")
	f.StringVar(&c.User, prefix+"username", "", "postgres username")
	f.StringVar(&c.Password, prefix+"password", "", "postgres password")

	return f
}

func (c *Config) BuildDSN() string {
	return fmt.Sprintf("pgx://%s:%s@%s/%s?x-multi-statement=true&x-statement-timeout=320000", c.User, c.Password, c.Addr, c.Database)
}
