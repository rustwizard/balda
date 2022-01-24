package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rustwizard/cleargo/db/pg"
)

func TestConnect(t *testing.T) {
	db := pg.NewDB()
	st := &Balda{db: db}
	err := st.db.Connect(&pg.Config{
		Host:         "127.0.0.1",
		Port:         5432,
		User:         "balda",
		Password:     "password",
		DatabaseName: "balda",
		MaxPoolSize:  100,
	})
	assert.NoError(t, err)
}
