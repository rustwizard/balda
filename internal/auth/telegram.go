package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TelegramUser holds the user fields extracted from validated Telegram
// Mini App init data.
type TelegramUser struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
}

// ErrTelegramInitData is returned when Telegram init data fails validation:
// malformed input, bad signature, expired auth_date or missing user payload.
var ErrTelegramInitData = errors.New("auth: invalid telegram init data")

// ValidateInitData verifies Telegram Mini App init data against the bot token
// and returns the embedded user. maxAge bounds the auth_date freshness.
//
// The check follows https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app:
// the data-check-string is every key=value pair except "hash", sorted by key
// and joined with '\n'; the secret key is HMAC-SHA256("WebAppData", botToken);
// the signature is HMAC-SHA256(dataCheckString, secret) as hex.
func ValidateInitData(initData, botToken string, maxAge time.Duration) (TelegramUser, error) {
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return TelegramUser{}, fmt.Errorf("%w: parse: %w", ErrTelegramInitData, err)
	}

	hash := vals.Get("hash")
	if hash == "" {
		return TelegramUser{}, fmt.Errorf("%w: no hash", ErrTelegramInitData)
	}

	pairs := make([]string, 0, len(vals))
	for key, values := range vals {
		if key == "hash" {
			continue
		}
		for _, v := range values {
			pairs = append(pairs, key+"="+v)
		}
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(botToken))
	secret := secretMAC.Sum(nil)

	sigMAC := hmac.New(sha256.New, secret)
	sigMAC.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(sigMAC.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(hash)) {
		return TelegramUser{}, fmt.Errorf("%w: bad signature", ErrTelegramInitData)
	}

	authDate, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return TelegramUser{}, fmt.Errorf("%w: bad auth_date", ErrTelegramInitData)
	}
	if age := time.Since(time.Unix(authDate, 0)); age > maxAge || age < -time.Minute {
		return TelegramUser{}, fmt.Errorf("%w: expired", ErrTelegramInitData)
	}

	userJSON := vals.Get("user")
	if userJSON == "" {
		return TelegramUser{}, fmt.Errorf("%w: no user", ErrTelegramInitData)
	}
	var u struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	}
	if err := json.Unmarshal([]byte(userJSON), &u); err != nil {
		return TelegramUser{}, fmt.Errorf("%w: bad user payload", ErrTelegramInitData)
	}
	if u.ID == 0 || u.FirstName == "" {
		return TelegramUser{}, fmt.Errorf("%w: incomplete user payload", ErrTelegramInitData)
	}

	return TelegramUser{
		ID:        u.ID,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Username:  u.Username,
	}, nil
}
