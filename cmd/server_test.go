package cmd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
)

type fakeGameResultSaver struct {
	calls int
	failN int // number of initial calls that should fail
	err   error
}

func (f *fakeGameResultSaver) SaveGameResult(ctx context.Context, r storage.GameResult) error {
	f.calls++
	if f.calls <= f.failN {
		return f.err
	}
	return nil
}

func TestMakeOnGameOverCallback_RetryExhausted(t *testing.T) {
	saver := &fakeGameResultSaver{failN: 10, err: errors.New("transient failure")}
	var pending sync.WaitGroup

	cb := makeOnGameOverCallback(saver, &pending)

	start := time.Now()
	cb(storage.GameResult{GameID: "g1"})
	pending.Wait()
	elapsed := time.Since(start)

	assert.Equal(t, 3, saver.calls, "expected 3 attempts")
	assert.True(t, elapsed >= 300*time.Millisecond, "expected at least 300ms backoff (100ms + 200ms)")
}

func TestMakeOnGameOverCallback_SuccessOnFirstTry(t *testing.T) {
	saver := &fakeGameResultSaver{failN: 0}
	var pending sync.WaitGroup

	cb := makeOnGameOverCallback(saver, &pending)

	start := time.Now()
	cb(storage.GameResult{GameID: "g2"})
	pending.Wait()
	elapsed := time.Since(start)

	assert.Equal(t, 1, saver.calls, "expected 1 attempt")
	assert.True(t, elapsed < 50*time.Millisecond, "expected no backoff delay")
}

func TestMakeOnGameOverCallback_SuccessOnRetry(t *testing.T) {
	saver := &fakeGameResultSaver{failN: 1, err: errors.New("boom")}
	var pending sync.WaitGroup

	cb := makeOnGameOverCallback(saver, &pending)

	start := time.Now()
	cb(storage.GameResult{GameID: "g3"})
	pending.Wait()
	elapsed := time.Since(start)

	assert.Equal(t, 2, saver.calls, "expected 2 attempts (1 fail + 1 success)")
	assert.True(t, elapsed >= 100*time.Millisecond, "expected at least 100ms backoff")
	assert.True(t, elapsed < 200*time.Millisecond, "expected less than 200ms total")
}
