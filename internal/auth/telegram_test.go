package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBotToken = "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"

// buildInitData constructs a correctly signed init data string for tests.
func buildInitData(t *testing.T, fields map[string]string) string {
	t.Helper()

	pairs := make([]string, 0, len(fields))
	for k, v := range fields {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(testBotToken))
	sigMAC := hmac.New(sha256.New, secretMAC.Sum(nil))
	sigMAC.Write([]byte(dataCheckString))

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hex.EncodeToString(sigMAC.Sum(nil)))
	return q.Encode()
}

func validFields() map[string]string {
	return map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"query_id":  "AAHdF6IQAAAAAN0XohDhrOrc",
		"user":      `{"id":279058397,"first_name":"Vladislav","last_name":"Kibenko","username":"vdkfrost"}`,
	}
}

func TestValidateInitData_Valid(t *testing.T) {
	u, err := ValidateInitData(buildInitData(t, validFields()), testBotToken, 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(279058397), u.ID)
	assert.Equal(t, "Vladislav", u.FirstName)
	assert.Equal(t, "Kibenko", u.LastName)
	assert.Equal(t, "vdkfrost", u.Username)
}

func TestValidateInitData_TamperedField(t *testing.T) {
	fields := validFields()
	fields["user"] = `{"id":1,"first_name":"Mallory","last_name":"","username":"mallory"}`
	// Sign with the original fields, then swap in the tampered user payload.
	initData := buildInitData(t, validFields())
	tampered := strings.Replace(initData,
		url.QueryEscape(validFields()["user"]),
		url.QueryEscape(fields["user"]), 1)
	require.NotEqual(t, initData, tampered)

	_, err := ValidateInitData(tampered, testBotToken, 24*time.Hour)
	assert.ErrorIs(t, err, ErrTelegramInitData)
}

func TestValidateInitData_WrongBotToken(t *testing.T) {
	_, err := ValidateInitData(buildInitData(t, validFields()), "999:wrong-token", 24*time.Hour)
	assert.ErrorIs(t, err, ErrTelegramInitData)
}

func TestValidateInitData_Expired(t *testing.T) {
	fields := validFields()
	fields["auth_date"] = strconv.FormatInt(time.Now().Add(-48*time.Hour).Unix(), 10)
	_, err := ValidateInitData(buildInitData(t, fields), testBotToken, 24*time.Hour)
	assert.ErrorIs(t, err, ErrTelegramInitData)
}

func TestValidateInitData_Malformed(t *testing.T) {
	cases := map[string]string{
		"empty":        "",
		"garbage":      "%%%",
		"no hash":      "auth_date=123&user=%7B%7D",
		"no auth_date": buildInitData(t, map[string]string{"user": `{"id":1,"first_name":"A"}`}),
		"no user": buildInitData(t, map[string]string{
			"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		}),
	}
	for name, initData := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ValidateInitData(initData, testBotToken, 24*time.Hour)
			assert.ErrorIs(t, err, ErrTelegramInitData, fmt.Sprintf("input %q", initData))
		})
	}
}
