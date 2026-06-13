// Package auth provides JWT access tokens, opaque refresh tokens, and helpers
// for carrying authenticated claims through the request context.
package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessTokenTTL is the lifetime of an access token JWT.
const AccessTokenTTL = time.Hour

// Claims is the set of claims embedded in an access token.
//
// RFC 7519 §4.1.2 defines "sub" as a string, so the internal user_id is stored
// both as the registered Subject (string) and as the custom UID claim (int64)
// for direct use in Go without re-parsing.
type Claims struct {
	UID      int64     `json:"uid"`
	PlayerID uuid.UUID `json:"pid"`
	Role     string    `json:"role"`
	jwt.RegisteredClaims
}

// GenerateAccessToken issues a signed HS256 access token for the given user.
func GenerateAccessToken(uid int64, pid uuid.UUID, role, secret string) (string, error) {
	now := time.Now()
	claims := Claims{
		UID:      uid,
		PlayerID: pid,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(uid, 10),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseAccessToken verifies the token signature and expiry and returns its claims.
func ParseAccessToken(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse access token: %w", err)
	}
	return claims, nil
}

// ctxKey is an unexported context key type to avoid collisions.
type ctxKey struct{}

// WithClaims returns a copy of ctx carrying the given claims.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, claims)
}

// FromContext extracts claims placed by WithClaims, if any.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(ctxKey{}).(*Claims)
	return claims, ok
}
