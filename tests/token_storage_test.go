package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/rustwizard/balda/internal/auth"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenStorage(t *testing.T) {
	ctx := context.Background()
	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	// Seed a user to satisfy the foreign key.
	var uid int64
	err := s.Pool().QueryRow(ctx,
		`INSERT INTO users(email, hash_password) VALUES($1, $2) RETURNING user_id`,
		"token.user@example.org", "x",
	).Scan(&uid)
	require.NoError(t, err)

	const secret = "test-secret-at-least-32-bytes-long!!"
	raw, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	hash := auth.HashRefreshToken(secret, raw)
	exp := time.Now().Add(auth.RefreshTokenTTL)

	require.NoError(t, s.SaveRefreshToken(ctx, storage.RefreshToken{
		UserID: uid, TokenHash: hash, ExpiresAt: exp, UserAgent: "test-agent", IPAddr: "192.168.1.1",
	}))

	t.Run("get returns the saved token", func(t *testing.T) {
		rt, err := s.GetRefreshToken(ctx, hash)
		require.NoError(t, err)
		assert.Equal(t, uid, rt.UserID)
		assert.Equal(t, hash, rt.TokenHash)
		assert.False(t, rt.Revoked)
		assert.Equal(t, "test-agent", rt.UserAgent)
		assert.Equal(t, "192.168.1.1", rt.IPAddr)
		assert.WithinDuration(t, exp, rt.ExpiresAt, time.Second)
	})

	t.Run("get unknown hash returns ErrNoRows", func(t *testing.T) {
		_, err := s.GetRefreshToken(ctx, "nonexistent")
		assert.ErrorIs(t, err, pgx.ErrNoRows)
	})

	t.Run("revoke marks the token revoked", func(t *testing.T) {
		require.NoError(t, s.RevokeRefreshToken(ctx, hash))
		rt, err := s.GetRefreshToken(ctx, hash)
		require.NoError(t, err)
		assert.True(t, rt.Revoked)
	})

	t.Run("revoke all marks remaining tokens revoked", func(t *testing.T) {
		raw2, err := auth.GenerateRefreshToken()
		require.NoError(t, err)
		hash2 := auth.HashRefreshToken(secret, raw2)
		require.NoError(t, s.SaveRefreshToken(ctx, storage.RefreshToken{
			UserID: uid, TokenHash: hash2, ExpiresAt: exp,
		}))

		require.NoError(t, s.RevokeAllUserTokens(ctx, uid))
		rt, err := s.GetRefreshToken(ctx, hash2)
		require.NoError(t, err)
		assert.True(t, rt.Revoked)
		assert.Empty(t, rt.UserAgent)
		assert.Empty(t, rt.IPAddr)
	})
}
