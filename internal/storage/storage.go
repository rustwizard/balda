package storage

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Balda struct {
	db *pgxpool.Pool
	t  time.Duration
}

func New(db *pgxpool.Pool, t time.Duration) *Balda {
	return &Balda{db: db, t: t}
}

func (b Balda) DB() *pgxpool.Pool {
	return b.db
}
