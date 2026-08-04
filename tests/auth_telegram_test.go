package tests

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/centrifugo"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTelegramBotToken = "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

// buildTelegramInitData signs test init data with the test bot token,
// mimicking what the Telegram WebView produces.
func buildTelegramInitData(t *testing.T, fields map[string]string) string {
	t.Helper()

	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(testTelegramBotToken))
	sigMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	sigMAC.Write([]byte(dataCheckString))

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hex.EncodeToString(sigMAC.Sum(nil)))
	return q.Encode()
}

func telegramUserFields(tgID int64) map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":` + strconv.FormatInt(tgID, 10) + `,"first_name":"Tema","last_name":"Testov","username":"tema_test"}`,
	}
}

func setupTelegramHandlers(t *testing.T) (*handlers.Handlers, func()) {
	t.Helper()
	ctx := context.Background()
	core := setupCore(ctx, t)
	cf := centrifugo.NewClient("http://localhost:8000/api", "test-key")
	h := handlers.New(core.svc, core.pres, cf, handlers.Config{JWTSecret: testJWTSecret, CentrifugoTokenHMACSecret: "test-secret", TelegramBotToken: testTelegramBotToken})
	return h, core.cleanup
}

func TestAuthTelegram(t *testing.T) {
	h, cleanup := setupTelegramHandlers(t)
	defer cleanup()

	ctx := context.Background()
	const tgID = int64(279058397)
	initData := buildTelegramInitData(t, telegramUserFields(tgID))

	t.Run("first visit registers the user", func(t *testing.T) {
		res, err := h.AuthTelegram(ctx, &baldaapi.TelegramAuthRequest{InitData: initData})
		require.NoError(t, err)

		ok, isOK := res.(*baldaapi.AuthResponse)
		require.True(t, isOK, "expected *AuthResponse, got %T", res)
		require.True(t, ok.Player.IsSet())
		assert.Equal(t, "Tema", ok.Player.Value.Firstname.Value)
		assert.Equal(t, "Testov", ok.Player.Value.Lastname.Value)
		assert.NotEmpty(t, ok.AccessToken.Value)
		assert.NotEmpty(t, ok.RefreshToken.Value)
	})

	t.Run("second visit logs into the same account", func(t *testing.T) {
		var firstUID string
		res, err := h.AuthTelegram(ctx, &baldaapi.TelegramAuthRequest{InitData: initData})
		require.NoError(t, err)
		first := res.(*baldaapi.AuthResponse) //nolint:errcheck
		firstUID = first.Player.Value.UID.Value.String()

		res, err = h.AuthTelegram(ctx, &baldaapi.TelegramAuthRequest{InitData: initData})
		require.NoError(t, err)
		second := res.(*baldaapi.AuthResponse) //nolint:errcheck
		assert.Equal(t, firstUID, second.Player.Value.UID.Value.String(), "must not create a duplicate user")
	})

	t.Run("tampered init data is rejected", func(t *testing.T) {
		tampered := strings.Replace(initData, "Tema", "Mallory", 1)
		require.NotEqual(t, initData, tampered)

		res, err := h.AuthTelegram(ctx, &baldaapi.TelegramAuthRequest{InitData: tampered})
		require.NoError(t, err)

		unauthorized, isUnauthorized := res.(*baldaapi.AuthTelegramUnauthorized)
		require.True(t, isUnauthorized, "expected *AuthTelegramUnauthorized, got %T", res)
		assert.Equal(t, 401, unauthorized.Status.Value)
	})
}

func TestAuthTelegram_NotConfigured(t *testing.T) {
	ctx := context.Background()
	core := setupCore(ctx, t)
	defer core.cleanup()
	cf := centrifugo.NewClient("http://localhost:8000/api", "test-key")
	h := handlers.New(core.svc, core.pres, cf, handlers.Config{JWTSecret: testJWTSecret, CentrifugoTokenHMACSecret: "test-secret"})

	res, err := h.AuthTelegram(ctx, &baldaapi.TelegramAuthRequest{InitData: "whatever=1"})
	require.NoError(t, err)

	unavailable, isUnavailable := res.(*baldaapi.AuthTelegramServiceUnavailable)
	require.True(t, isUnavailable, "expected *AuthTelegramServiceUnavailable, got %T", res)
	assert.Equal(t, 503, unavailable.Status.Value)
}
