package rnd

import (
	"errors"
	"math/rand"
)

func Int(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("argument shouldn't be negative or zero")
	}
	return rand.Intn(n), nil
}
