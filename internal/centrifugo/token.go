package centrifugo

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateConnectionToken returns a Centrifugo connection JWT for the given user.
func GenerateConnectionToken(uid, secret string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": uid,
		"exp": jwt.NewNumericDate(time.Now().Add(ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateSubscriptionToken returns a Centrifugo subscription JWT
// that authorizes the given user to subscribe to the given channel.
func GenerateSubscriptionToken(uid, channel, secret string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":     uid,
		"channel": channel,
		"exp":     jwt.NewNumericDate(time.Now().Add(ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
