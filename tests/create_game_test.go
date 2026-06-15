package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	playerCtx, pid := signupCtx(t, h, "game.creator@example.org")

	t.Run("missing claims returns 401", func(t *testing.T) {
		res, err := h.CreateGame(context.Background())
		require.NoError(t, err)

		errResp, ok := res.(*baldaapi.ErrorResponse)
		require.True(t, ok, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("valid session creates a waiting game", func(t *testing.T) {
		res, err := h.CreateGame(playerCtx)
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.CreateGameResponse)
		require.True(t, ok, "expected *CreateGameResponse, got %T", res)
		require.True(t, resp.Game.IsSet())

		g := resp.Game.Value
		assert.True(t, g.ID.IsSet())
		assert.NotEmpty(t, g.ID.Value.String())
		assert.Equal(t, baldaapi.GameStatusWaiting, g.Status.Value)
		assert.True(t, g.StartedAt.IsSet())
		assert.Positive(t, g.StartedAt.Value)
		require.Len(t, g.Players, 1)
		assert.Equal(t, pid, g.Players[0].UID.Value)
	})

	t.Run("player already in a game returns conflict", func(t *testing.T) {
		// The player already has a game from the previous sub-test.
		res, err := h.CreateGame(playerCtx)
		require.NoError(t, err)

		errResp, ok := res.(*baldaapi.ErrorResponse)
		require.True(t, ok, "expected *ErrorResponse, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})
}

func TestCreateGameHTTP(t *testing.T) {
	srv, token, cleanup := setupServer(t)
	defer cleanup()

	gamesURL := srv.URL + "/balda/api/v1/games"

	t.Run("missing token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, gamesURL, http.NoBody)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer bad-token")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("valid request creates game and returns 200", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Game struct {
				ID      string `json:"id"`
				Status  string `json:"status"`
				Players []struct {
					UID string `json:"uid"`
					Exp int64  `json:"exp"`
				} `json:"players"`
				StartedAt int64 `json:"started_at"`
			} `json:"game"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

		assert.NotEmpty(t, body.Game.ID)
		assert.Equal(t, "waiting", body.Game.Status)
		assert.Len(t, body.Game.Players, 1)
		assert.Positive(t, body.Game.StartedAt)
	})
}
