package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-at-least-32-bytes-long!!"

func TestGenerateAndParseAccessToken(t *testing.T) {
	pid := uuid.New()
	tok, err := auth.GenerateAccessToken(42, pid, "player", testSecret)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	claims, err := auth.ParseAccessToken(tok, testSecret)
	require.NoError(t, err)
	assert.Equal(t, int64(42), claims.UID)
	assert.Equal(t, pid, claims.PlayerID)
	assert.Equal(t, "player", claims.Role)
	assert.Equal(t, "42", claims.Subject)
	assert.NotEmpty(t, claims.ID) // jti
	assert.WithinDuration(t, time.Now().Add(auth.AccessTokenTTL), claims.ExpiresAt.Time, 5*time.Second)
}

func TestParseAccessToken_WrongSecret(t *testing.T) {
	tok, err := auth.GenerateAccessToken(1, uuid.New(), "player", testSecret)
	require.NoError(t, err)

	_, err = auth.ParseAccessToken(tok, "another-secret")
	assert.Error(t, err)
}

func TestParseAccessToken_Garbage(t *testing.T) {
	_, err := auth.ParseAccessToken("not-a-jwt", testSecret)
	assert.Error(t, err)
}

func TestHashRefreshToken_Deterministic(t *testing.T) {
	raw, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	assert.Len(t, raw, 64) // 32 bytes hex-encoded

	h1 := auth.HashRefreshToken(testSecret, raw)
	h2 := auth.HashRefreshToken(testSecret, raw)
	assert.Equal(t, h1, h2, "hash must be deterministic for indexed lookup")
	assert.NotEqual(t, raw, h1, "stored hash must differ from raw token")

	hOther := auth.HashRefreshToken("other-secret", raw)
	assert.NotEqual(t, h1, hOther, "hash must depend on the secret")
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
	a, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	b, err := auth.GenerateRefreshToken()
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestClaimsContextRoundTrip(t *testing.T) {
	claims := &auth.Claims{UID: 7, Role: "admin"}
	ctx := auth.WithClaims(context.Background(), claims)

	got, ok := auth.FromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, claims, got)

	_, ok = auth.FromContext(context.Background())
	assert.False(t, ok)
}
