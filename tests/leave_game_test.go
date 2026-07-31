package tests

import (
	"context"
	"testing"

	"github.com/google/uuid"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLeaveGame(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	createGame := func(ctx context.Context) uuid.UUID {
		t.Helper()
		res, err := h.CreateGame(ctx)
		require.NoError(t, err)
		created, ok := res.(*baldaapi.CreateGameResponse)
		require.True(t, ok, "expected *CreateGameResponse, got %T", res)
		return created.Game.Value.ID.Value
	}

	t.Run("leave waiting game and create again", func(t *testing.T) {
		ctx, _ := signupCtx(t, h, "leave.creator@example.org")
		gid := createGame(ctx)

		res, err := h.LeaveGame(ctx, baldaapi.LeaveGameParams{ID: gid})
		require.NoError(t, err)
		_, isNoContent := res.(*baldaapi.LeaveGameNoContent)
		require.True(t, isNoContent, "expected *LeaveGameNoContent, got %T", res)

		// The player is free to create a new game right away.
		_ = createGame(ctx)
	})

	t.Run("cannot leave an in-progress game", func(t *testing.T) {
		ctx, _ := signupCtx(t, h, "leave.owner@example.org")
		gid := createGame(ctx)

		ctx2, _ := signupCtx(t, h, "leave.joiner@example.org")
		_, err := h.JoinGame(ctx2, baldaapi.JoinGameParams{ID: gid})
		require.NoError(t, err)

		res, err := h.LeaveGame(ctx, baldaapi.LeaveGameParams{ID: gid})
		require.NoError(t, err)
		conflict, isConflict := res.(*baldaapi.LeaveGameConflict)
		require.True(t, isConflict, "expected *LeaveGameConflict, got %T", res)
		assert.Equal(t, 409, conflict.Status.Value)
	})

	t.Run("non-participant gets 403", func(t *testing.T) {
		ctx, _ := signupCtx(t, h, "leave.host@example.org")
		gid := createGame(ctx)

		ctx3, _ := signupCtx(t, h, "leave.outsider@example.org")
		res, err := h.LeaveGame(ctx3, baldaapi.LeaveGameParams{ID: gid})
		require.NoError(t, err)
		_, isForbidden := res.(*baldaapi.LeaveGameForbidden)
		require.True(t, isForbidden, "expected *LeaveGameForbidden, got %T", res)
	})
}
