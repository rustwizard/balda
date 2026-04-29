package gamecoord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindWinnerByScore(t *testing.T) {
	cases := []struct {
		name       string
		scores     []game.PlayerState
		excludeUID string
		want       string
	}{
		{
			name: "single winner",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
				{UID: "b", Score: 5},
			},
			want: "a",
		},
		{
			name: "draw equal scores",
			scores: []game.PlayerState{
				{UID: "a", Score: 7},
				{UID: "b", Score: 7},
			},
			want: "",
		},
		{
			name: "draw three way tie",
			scores: []game.PlayerState{
				{UID: "a", Score: 3},
				{UID: "b", Score: 3},
				{UID: "c", Score: 3},
			},
			want: "",
		},
		{
			name: "exclude kicked player",
			scores: []game.PlayerState{
				{UID: "a", Score: 100},
				{UID: "b", Score: 5},
			},
			excludeUID: "a",
			want:       "b",
		},
		{
			name: "exclude not in scores",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
				{UID: "b", Score: 5},
			},
			excludeUID: "z",
			want:       "a",
		},
		{
			name: "all excluded",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
			},
			excludeUID: "a",
			want:       "",
		},
		{
			name: "three players winner by score",
			scores: []game.PlayerState{
				{UID: "a", Score: 5},
				{UID: "b", Score: 12},
				{UID: "c", Score: 8},
			},
			want: "b",
		},
		{
			name: "three players draw top two",
			scores: []game.PlayerState{
				{UID: "a", Score: 10},
				{UID: "b", Score: 10},
				{UID: "c", Score: 3},
			},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, findWinnerByScore(tc.scores, tc.excludeUID))
		})
	}
}

// TestPublishGameOver_PersistsBeforePublish verifies that onGameOver is invoked
// before the Centrifugo Publish call when a game ends via kick. This guarantees
// the database is the source of truth by the time clients see the game_over event.
func TestPublishGameOver_PersistsBeforePublish(t *testing.T) {
	saved := make(chan struct{})
	published := make(chan struct{}, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Channel string `json:"channel"`
			Data    struct {
				Type string `json:"type"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode publish request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Only enforce ordering for the game_over event.
		if payload.Data.Type != "game_over" {
			w.WriteHeader(http.StatusOK)
			return
		}

		select {
		case <-saved:
			// OK — save happened before publish
		default:
			t.Error("Centrifugo Publish called before onGameOver")
		}
		published <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cf := centrifugo.NewClient(ts.URL, "test-key")

	p1 := &game.Player{ID: "p1", Score: 10}
	p2 := &game.Player{ID: "p2", Score: 5}

	coord := New("game-1", []*game.Player{p1, p2}, cf)
	coord.SetOnGameOver(func(r storage.GameResult) {
		close(saved)
	})

	g, err := game.NewGameWithWord([]*game.Player{p1, p2}, "масло", coord)
	require.NoError(t, err)
	coord.SetGame(g)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	// Give the game loop time to enter its select before we kick.
	time.Sleep(50 * time.Millisecond)
	g.Kick()

	// Wait for the game loop to exit.
	select {
	case <-g.Done():
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for game to finish")
	}

	// Wait for the asynchronous publish to complete.
	select {
	case <-published:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for Centrifugo publish")
	}
}
