package tests

import (
	"testing"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchmakingJoinLeave(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx, _ := signupCtx(t, h, "qm.basic@example.org")

	// Join the queue.
	res, err := h.MatchmakingJoin(ctx)
	require.NoError(t, err)
	_, isOK := res.(*baldaapi.MatchmakingJoinResponse)
	require.True(t, isOK, "expected *MatchmakingJoinResponse, got %T", res)

	// Joining again while queued is a conflict.
	res, err = h.MatchmakingJoin(ctx)
	require.NoError(t, err)
	_, isConflict := res.(*baldaapi.MatchmakingJoinConflict)
	require.True(t, isConflict, "expected *MatchmakingJoinConflict, got %T", res)

	// Leaving works and is idempotent.
	leaveRes, err := h.MatchmakingLeave(ctx)
	require.NoError(t, err)
	_, isNoContent := leaveRes.(*baldaapi.MatchmakingLeaveNoContent)
	require.True(t, isNoContent, "expected *MatchmakingLeaveNoContent, got %T", leaveRes)

	leaveRes, err = h.MatchmakingLeave(ctx)
	require.NoError(t, err)
	_, isNoContent = leaveRes.(*baldaapi.MatchmakingLeaveNoContent)
	require.True(t, isNoContent, "leave must be idempotent, got %T", leaveRes)

	// And joining after leaving works again.
	res, err = h.MatchmakingJoin(ctx)
	require.NoError(t, err)
	_, isOK = res.(*baldaapi.MatchmakingJoinResponse)
	require.True(t, isOK, "expected *MatchmakingJoinResponse, got %T", res)
}

func TestMatchmakingJoin_AlreadyInGame(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx, _ := signupCtx(t, h, "qm.ingame@example.org")

	// Create a game so the player is busy.
	createRes, err := h.CreateGame(ctx)
	require.NoError(t, err)
	require.IsType(t, &baldaapi.CreateGameResponse{}, createRes)

	res, err := h.MatchmakingJoin(ctx)
	require.NoError(t, err)
	conflict, isConflict := res.(*baldaapi.MatchmakingJoinConflict)
	require.True(t, isConflict, "expected *MatchmakingJoinConflict, got %T", res)
	assert.Equal(t, 409, conflict.Status.Value)
}
