package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/game"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/notifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGamesHandler(t *testing.T) {
	h, lby, cleanup := setupFull(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("empty lobby returns empty list", func(t *testing.T) {
		res, err := h.ListGames(ctx, baldaapi.ListGamesParams{})
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.ListGamesResponse)
		require.True(t, ok, "expected *ListGamesResponse, got %T", res)
		assert.Empty(t, resp.Games)
	})

	p1 := &game.Player{ID: uuid.NewString()}
	p2 := &game.Player{ID: uuid.NewString()}

	rec, err := lby.StartGame(ctx, []*game.Player{p1, p2}, &notifier.Noop{})
	require.NoError(t, err)

	t.Run("active game appears in list", func(t *testing.T) {
		res, err := h.ListGames(ctx, baldaapi.ListGamesParams{})
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.ListGamesResponse)
		require.True(t, ok)
		require.Len(t, resp.Games, 1)

		g := resp.Games[0]
		assert.Equal(t, rec.ID, g.ID.Value.String())
		assert.True(t, g.StartedAt.IsSet())
		assert.Positive(t, g.StartedAt.Value)

		gotIDs := make([]string, len(g.PlayerIds))
		for i, uid := range g.PlayerIds {
			gotIDs[i] = uid.String()
		}
		assert.ElementsMatch(t, []string{p1.ID, p2.ID}, gotIDs)
	})

	p3 := &game.Player{ID: uuid.NewString()}
	p4 := &game.Player{ID: uuid.NewString()}

	_, err = lby.StartGame(ctx, []*game.Player{p3, p4}, &notifier.Noop{})
	require.NoError(t, err)

	t.Run("two active games both appear", func(t *testing.T) {
		res, err := h.ListGames(ctx, baldaapi.ListGamesParams{})
		require.NoError(t, err)

		resp := res.(*baldaapi.ListGamesResponse)
		assert.Len(t, resp.Games, 2)
	})

	require.NoError(t, lby.Remove(rec.ID))

	t.Run("removed game disappears from list", func(t *testing.T) {
		res, err := h.ListGames(ctx, baldaapi.ListGamesParams{})
		require.NoError(t, err)

		resp := res.(*baldaapi.ListGamesResponse)
		require.Len(t, resp.Games, 1)
		assert.NotEqual(t, rec.ID, resp.Games[0].ID.Value.String())
	})
}

func TestListGamesHTTP(t *testing.T) {
	srv, email, password, cleanup := setupServer(t)
	defer cleanup()

	gamesURL := srv.URL + "/balda/api/v1/games"

	// Auth once to get a valid session SID.
	authResp, err := http.DefaultClient.Do(func() *http.Request {
		body, _ := json.Marshal(map[string]string{"email": email, "password": password})
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", testAPIToken)
		return req
	}())
	require.NoError(t, err)
	defer authResp.Body.Close()

	var authBody struct {
		Player struct {
			Sid string `json:"sid"`
		} `json:"player"`
	}
	require.NoError(t, json.NewDecoder(authResp.Body).Decode(&authBody))
	sid := authBody.Player.Sid
	require.NotEmpty(t, sid)

	t.Run("missing api key returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-API-Session", sid)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("valid api key returns 200 with games array", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("X-API-Key", testAPIToken)
		req.Header.Set("X-API-Session", sid)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var body struct {
			Games []json.RawMessage `json:"games"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.NotNil(t, body.Games)
	})
}
