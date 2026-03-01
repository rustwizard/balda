package session

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startRedis(ctx context.Context, t *testing.T) (addr string, cleanup func()) {
	t.Helper()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:8-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	addr = fmt.Sprintf("%s:%s", host, port.Port())
	cleanup = func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate redis container: %v", err)
		}
	}
	return addr, cleanup
}

func TestSessionService(t *testing.T) {
	ctx := context.Background()

	addr, cleanup := startRedis(ctx, t)
	defer cleanup()

	svc := NewService(Config{Addr: addr, Expiration: defaultExpiration})

	t.Run("Save and Get", func(t *testing.T) {
		u := &User{Sid: "test_sid", UID: 1000}
		err := svc.Save(u)
		require.NoError(t, err)

		got, err := svc.Get(1000)
		assert.NoError(t, err)
		assert.Equal(t, int64(1000), got.UID)
		assert.Equal(t, "test_sid", got.Sid)
	})

	t.Run("Get not found", func(t *testing.T) {
		got, err := svc.Get(9999)
		assert.EqualError(t, err, ErrNotFound.Error())
		assert.Nil(t, got)
	})

	t.Run("Save empty sid", func(t *testing.T) {
		err := svc.Save(&User{Sid: "", UID: 2000})
		assert.EqualError(t, err, ErrEmptySessionID.Error())
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
