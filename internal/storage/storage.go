package storage

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Balda struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}
