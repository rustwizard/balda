package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signupHTTP signs up a new user over HTTP and returns the issued token pair.
func signupHTTP(t *testing.T, srv *httptest.Server, email string) (access, refresh string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"firstname": "Tok", "lastname": "User", "email": email, "password": "pass",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.AccessToken)
	require.NotEmpty(t, out.RefreshToken)
	return out.AccessToken, out.RefreshToken
}

func postRefresh(t *testing.T, srv *httptest.Server, refreshToken string) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestRefreshTokenHTTP(t *testing.T) {
	srv, _, cleanup := setupServer(t)
	defer cleanup()

	t.Run("valid refresh returns a new pair and rotates the old token", func(t *testing.T) {
		_, refresh := signupHTTP(t, srv, "refresh.valid@example.org")

		resp := postRefresh(t, srv, refresh)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			TokenType    string `json:"token_type"`
			ExpiresIn    int    `json:"expires_in"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		assert.NotEmpty(t, out.AccessToken)
		assert.NotEmpty(t, out.RefreshToken)
		assert.NotEqual(t, refresh, out.RefreshToken, "refresh token must rotate")
		assert.Equal(t, "Bearer", out.TokenType)
		assert.Positive(t, out.ExpiresIn)

		// The new access token must authorize a protected endpoint.
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/balda/api/v1/games", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+out.AccessToken)
		protectedResp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer protectedResp.Body.Close()
		assert.Equal(t, http.StatusOK, protectedResp.StatusCode)
	})

	t.Run("unknown refresh token returns 401", func(t *testing.T) {
		resp := postRefresh(t, srv, "deadbeef-not-a-real-token")
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("replaying a rotated token returns 401", func(t *testing.T) {
		_, refresh := signupHTTP(t, srv, "refresh.replay@example.org")

		// First use rotates it.
		first := postRefresh(t, srv, refresh)
		first.Body.Close()
		require.Equal(t, http.StatusOK, first.StatusCode)

		// Reusing the now-revoked token must fail.
		replay := postRefresh(t, srv, refresh)
		defer replay.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, replay.StatusCode)
	})
}

func TestLogoutHTTP(t *testing.T) {
	srv, _, cleanup := setupServer(t)
	defer cleanup()

	logoutURL := srv.URL + "/balda/api/v1/auth/logout"

	t.Run("missing token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, logoutURL, http.NoBody)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("logout revokes the refresh token", func(t *testing.T) {
		access, refresh := signupHTTP(t, srv, "logout.user@example.org")

		body, _ := json.Marshal(map[string]string{"refresh_token": refresh})
		req, _ := http.NewRequest(http.MethodPost, logoutURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// The revoked refresh token can no longer be exchanged.
		after := postRefresh(t, srv, refresh)
		defer after.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, after.StatusCode)
	})
}
