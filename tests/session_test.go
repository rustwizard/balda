package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rustwizard/balda/internal/session"
)

func TestSessionService(t *testing.T) {
	ctx := context.Background()

	addr, cleanup := startRedis(ctx, t)
	defer cleanup()

	svc := session.NewService(session.Config{Addr: addr, Expiration: 30 * time.Second})

	t.Run("Save and Get", func(t *testing.T) {
		u := &session.User{Sid: "test_sid", UID: 1000}
		err := svc.Save(u)
		require.NoError(t, err)

		got, err := svc.Get(1000)
		assert.NoError(t, err)
		assert.Equal(t, int64(1000), got.UID)
		assert.Equal(t, "test_sid", got.Sid)
	})

	t.Run("Get not found", func(t *testing.T) {
		got, err := svc.Get(9999)
		assert.EqualError(t, err, session.ErrNotFound.Error())
		assert.Nil(t, got)
	})

	t.Run("Save empty sid", func(t *testing.T) {
		err := svc.Save(&session.User{Sid: "", UID: 2000})
		assert.EqualError(t, err, session.ErrEmptySessionID.Error())
	})

	t.Run("Create", func(t *testing.T) {
		sid, err := svc.Create(3000)
		require.NoError(t, err)
		assert.NotEmpty(t, sid)

		got, err := svc.Get(3000)
		require.NoError(t, err)
		assert.Equal(t, int64(3000), got.UID)
		assert.Equal(t, sid, got.Sid)
	})
}
