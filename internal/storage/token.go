package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RefreshToken mirrors a row in the refresh_tokens table.
type RefreshToken struct {
	TokenID   uuid.UUID
	UserID    int64
	TokenHash string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Revoked   bool
	UserAgent string
	IPAddr    string
}

// SaveRefreshToken stores an HMAC-SHA256-hashed refresh token for the user.
// TokenID, IssuedAt and Revoked are not used on insert (defaults apply).
// Empty UserAgent / IPAddr are stored as NULL.
func (b *Balda) SaveRefreshToken(ctx context.Context, rt RefreshToken) error {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	_, err := b.db.Exec(ctx,
		`INSERT INTO refresh_tokens(user_id, token_hash, expires_at, user_agent, ip_addr)
		 VALUES($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::inet)`,
		rt.UserID, rt.TokenHash, rt.ExpiresAt, rt.UserAgent, rt.IPAddr,
	)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken fetches a token row by its hash. The returned error wraps
// pgx.ErrNoRows when no row matches, so callers can detect a missing token.
func (b *Balda) GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error) {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	var rt RefreshToken
	err := b.db.QueryRow(ctx,
		`SELECT token_id, user_id, token_hash, issued_at, expires_at, revoked,
		        COALESCE(user_agent, ''), COALESCE(host(ip_addr), '')
		 FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rt.TokenID, &rt.UserID, &rt.TokenHash, &rt.IssuedAt, &rt.ExpiresAt, &rt.Revoked, &rt.UserAgent, &rt.IPAddr)
	if err != nil {
		return RefreshToken{}, fmt.Errorf("get refresh token: %w", err)
	}
	return rt, nil
}

// RevokeRefreshToken marks a single token as revoked. Idempotent.
func (b *Balda) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	_, err := b.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`, tokenHash,
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserTokens marks all of a user's tokens as revoked (replay-attack response).
func (b *Balda) RevokeAllUserTokens(ctx context.Context, uid int64) error {
	ctx, cancel := context.WithTimeout(ctx, b.t)
	defer cancel()

	_, err := b.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked = true WHERE user_id = $1 AND revoked = false`, uid,
	)
	if err != nil {
		return fmt.Errorf("revoke all user tokens: %w", err)
	}
	return nil
}
