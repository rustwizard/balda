package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// RefreshTokenTTL is the lifetime of a refresh token.
const RefreshTokenTTL = 30 * 24 * time.Hour

// GenerateRefreshToken returns an opaque 64-char hex string backed by 32 random bytes.
func GenerateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: generate refresh token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashRefreshToken returns HMAC-SHA256(secret, rawToken) as a hex string.
// Deterministic — safe for use as a unique DB index key for lookup.
func HashRefreshToken(secret, rawToken string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(rawToken))
	return hex.EncodeToString(mac.Sum(nil))
}
