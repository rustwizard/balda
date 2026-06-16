package game

import (
	"context"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubOnline struct{ online bool }

func (s stubOnline) IsOnline(string) bool { return s.online }

// TestStartTurn_OfflinePlayerGetsGraceTimeout verifies that when the current
// human player is offline, their turn uses the shortened grace timer instead of
// the full turn duration, so an abandoned game advances quickly.
func TestStartTurn_OfflinePlayerGetsGraceTimeout(t *testing.T) {
	orig := game.OfflineGraceDuration
	game.OfflineGraceDuration = 40 * time.Millisecond
	t.Cleanup(func() { game.OfflineGraceDuration = orig })

	n := &mockNotifier{}
	players := makePlayers("p1", "p2")
	g, err := game.NewGameWithWord(players, "волна", n,
		game.WithTurnDuration(2*time.Second), // full turn is deliberately long
		game.WithOnlineChecker(stubOnline{online: false}),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	require.Eventually(t, func() bool { return n.timeoutCount() >= 1 },
		500*time.Millisecond, 5*time.Millisecond,
		"offline player's turn should time out within the grace window, not the full 2s")
}

// TestStartTurn_OnlinePlayerUsesFullTurn verifies that an online player keeps the
// full turn duration — the grace timer must never shorten a present player's turn.
func TestStartTurn_OnlinePlayerUsesFullTurn(t *testing.T) {
	orig := game.OfflineGraceDuration
	game.OfflineGraceDuration = 40 * time.Millisecond
	t.Cleanup(func() { game.OfflineGraceDuration = orig })

	n := &mockNotifier{}
	players := makePlayers("p1", "p2")
	g, err := game.NewGameWithWord(players, "волна", n,
		game.WithTurnDuration(2*time.Second),
		game.WithOnlineChecker(stubOnline{online: true}),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 0, n.timeoutCount(), "online player should not be rushed by the grace timer")
}
