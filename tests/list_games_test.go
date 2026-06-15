package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/game"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListGamesHandler(t *testing.T) {
	h, lby, cleanup := setupFull(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("empty lobby returns empty list", func(t *testing.T) {
		res, err := h.ListGames(ctx)
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.ListGamesResponse)
		require.True(t, ok, "expected *ListGamesResponse, got %T", res)
		assert.Empty(t, resp.Games)
	})

	p1 := &game.Player{ID: uuid.NewString(), Exp: 100, Type: game.PlayerTypeHuman}
	p2 := &game.Player{ID: uuid.NewString(), Exp: 200, Type: game.PlayerTypeHuman}

	rec, err := lby.StartGame(ctx, []*game.Player{p1, p2}, &game.NoopNotifier{})
	require.NoError(t, err)

	t.Run("active game appears in list", func(t *testing.T) {
		res, err := h.ListGames(ctx)
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.ListGamesResponse)
		require.True(t, ok)
		require.Len(t, resp.Games, 1)

		g := resp.Games[0]
		assert.Equal(t, rec.ID, g.ID.Value.String())
		assert.True(t, g.StartedAt.IsSet())
		assert.Positive(t, g.StartedAt.Value)

		gotIDs := make([]string, len(g.Players))
		for i, p := range g.Players {
			gotIDs[i] = p.UID.Value.String()
		}
		assert.ElementsMatch(t, []string{p1.ID, p2.ID}, gotIDs)
	})

	p3 := &game.Player{ID: uuid.NewString(), Exp: 300, Type: game.PlayerTypeHuman}
	p4 := &game.Player{ID: uuid.NewString(), Exp: 400, Type: game.PlayerTypeHuman}

	_, err = lby.StartGame(ctx, []*game.Player{p3, p4}, &game.NoopNotifier{})
	require.NoError(t, err)

	t.Run("two active games both appear", func(t *testing.T) {
		res, err := h.ListGames(ctx)
		require.NoError(t, err)

		resp := res.(*baldaapi.ListGamesResponse)
		assert.Len(t, resp.Games, 2)
	})

	require.NoError(t, lby.Remove(rec.ID))

	t.Run("removed game disappears from list", func(t *testing.T) {
		res, err := h.ListGames(ctx)
		require.NoError(t, err)

		resp := res.(*baldaapi.ListGamesResponse)
		require.Len(t, resp.Games, 1)
		assert.NotEqual(t, rec.ID, resp.Games[0].ID.Value.String())
	})
}

func TestListGamesHTTP(t *testing.T) {
	srv, token, cleanup := setupServer(t)
	defer cleanup()

	gamesURL := srv.URL + "/balda/api/v1/games"

	t.Run("missing token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("valid token returns 200 with games array", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gamesURL, http.NoBody)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

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
