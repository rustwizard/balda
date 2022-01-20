package rnd

import (
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func Int(n int) int {
	return rand.Intn(n)
}
