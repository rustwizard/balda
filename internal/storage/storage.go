package storage

import (
	"github.com/redis/go-redis/v9"
	"github.com/rustwizard/cleargo/db/pg"
)

type Balda struct {
	db  *pg.DB
	rdb *redis.Client
}
