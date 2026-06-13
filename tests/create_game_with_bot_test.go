package tests

import (
	"context"
	"net/http"
	"testing"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGameWithBotHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	playerCtx, pid := signupCtx(t, h, "bot.player@example.org")

	t.Run("missing claims returns 401", func(t *testing.T) {
		res, err := h.CreateGameWithBot(context.Background())
		require.NoError(t, err)

		errResp, ok := res.(*baldaapi.CreateGameWithBotUnauthorized)
		require.True(t, ok, "expected *CreateGameWithBotUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("valid session creates and starts a game with bot", func(t *testing.T) {
		res, err := h.CreateGameWithBot(playerCtx)
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.JoinGameResponse)
		require.True(t, ok, "expected *JoinGameResponse, got %T", res)
		require.True(t, resp.Game.IsSet())

		g := resp.Game.Value
		assert.True(t, g.ID.IsSet())
		assert.Equal(t, baldaapi.GameStatusInProgress, g.Status.Value)
		require.Len(t, g.Players, 2)
		assert.Equal(t, pid, g.Players[0].UID.Value)
		assert.True(t, resp.GameToken.IsSet())
		assert.NotEmpty(t, resp.GameToken.Value)
		assert.NotNil(t, resp.Board)
		assert.True(t, resp.CurrentTurnUID.IsSet())
		assert.Equal(t, pid.String(), resp.CurrentTurnUID.Value)
	})

	t.Run("player already in a game returns conflict", func(t *testing.T) {
		res, err := h.CreateGameWithBot(playerCtx)
		require.NoError(t, err)

		errResp, ok := res.(*baldaapi.CreateGameWithBotConflict)
		require.True(t, ok, "expected *CreateGameWithBotConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})
}
