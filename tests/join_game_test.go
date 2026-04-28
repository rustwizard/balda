package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	creatorRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Creator", Lastname: "User", Email: "join.creator@example.org", Password: "pass",
	})
	require.NoError(t, err)
	creator := creatorRes.(*baldaapi.SignupResponse).User.Value

	joinerRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Joiner", Lastname: "User", Email: "join.joiner@example.org", Password: "pass",
	})
	require.NoError(t, err)
	joiner := joinerRes.(*baldaapi.SignupResponse).User.Value

	t.Run("unknown session returns 401", func(t *testing.T) {
		res, err := h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: "bad-sid", ID: uuid.New()})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.JoinGameUnauthorized)
		require.True(t, ok, "expected *JoinGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("unknown game id returns 404", func(t *testing.T) {
		res, err := h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: joiner.Sid.Value, ID: uuid.New()})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.JoinGameNotFound)
		require.True(t, ok, "expected *JoinGameNotFound, got %T", res)
		assert.Equal(t, http.StatusNotFound, errResp.Status.Value)
	})

	// Creator opens a waiting game.
	createRes, err := h.CreateGame(ctx, baldaapi.CreateGameParams{XAPISession: creator.Sid.Value})
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	t.Run("joiner joins and game becomes in_progress", func(t *testing.T) {
		res, err := h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
		require.NoError(t, err)

		resp, ok := res.(*baldaapi.JoinGameResponse)
		require.True(t, ok, "expected *JoinGameResponse, got %T", res)
		require.True(t, resp.Game.IsSet())

		g := resp.Game.Value
		assert.Equal(t, gameID, g.ID.Value)
		assert.Equal(t, baldaapi.GameStatusInProgress, g.Status.Value)
		assert.Len(t, g.Players, 2)
		playerUIDs := make([]uuid.UUID, len(g.Players))
		for i, p := range g.Players {
			playerUIDs[i] = p.UID.Value
		}
		assert.Contains(t, playerUIDs, creator.UID.Value)
		assert.Contains(t, playerUIDs, joiner.UID.Value)
	})

	t.Run("joining an already running game returns 409", func(t *testing.T) {
		thirdRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
			Firstname: "Third", Lastname: "User", Email: "join.third@example.org", Password: "pass",
		})
		require.NoError(t, err)
		third := thirdRes.(*baldaapi.SignupResponse).User.Value

		res, err := h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: third.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.JoinGameConflict)
		require.True(t, ok, "expected *JoinGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("player already in a game cannot join another", func(t *testing.T) {
		host2Res, err := h.Signup(ctx, &baldaapi.SignupRequest{
			Firstname: "Host2", Lastname: "User", Email: "join.host2@example.org", Password: "pass",
		})
		require.NoError(t, err)
		host2 := host2Res.(*baldaapi.SignupResponse).User.Value

		createRes2, err := h.CreateGame(ctx, baldaapi.CreateGameParams{XAPISession: host2.Sid.Value})
		require.NoError(t, err)
		gameID2 := createRes2.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

		// joiner is already in the first game
		res, err := h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: joiner.Sid.Value, ID: gameID2})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.JoinGameConflict)
		require.True(t, ok, "expected *JoinGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})
}

func TestJoinGameHTTP(t *testing.T) {
	srv, email, password, cleanup := setupServer(t)
	defer cleanup()

	// Auth the pre-seeded creator (X-API-Key is required).
	authBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
	authReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth", bytes.NewReader(authBody))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("X-API-Key", testAPIToken)
	resp, err := http.DefaultClient.Do(authReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	var authData struct {
		Player struct{ Sid string `json:"sid"` } `json:"player"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&authData))
	creatorSid := authData.Player.Sid

	joinerSid := postSignup(t, srv, "http.joiner@example.org", "pass")

	// Creator creates a waiting game.
	createReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/games", http.NoBody)
	createReq.Header.Set("X-API-Key", testAPIToken)
	createReq.Header.Set("X-API-Session", creatorSid)
	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusOK, createResp.StatusCode)

	var createBody struct {
		Game struct{ ID string `json:"id"` } `json:"game"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createBody))
	gameID := createBody.Game.ID

	joinURL := fmt.Sprintf("%s/balda/api/v1/games/%s/join", srv.URL, gameID)

	t.Run("missing api key returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, joinURL, http.NoBody)
		req.Header.Set("X-API-Session", joinerSid)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("unknown session returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, joinURL, http.NoBody)
		req.Header.Set("X-API-Key", testAPIToken)
		req.Header.Set("X-API-Session", "bad-sid")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("unknown game id returns 404", func(t *testing.T) {
		url := fmt.Sprintf("%s/balda/api/v1/games/%s/join", srv.URL, uuid.New())
		req, _ := http.NewRequest(http.MethodPost, url, http.NoBody)
		req.Header.Set("X-API-Key", testAPIToken)
		req.Header.Set("X-API-Session", joinerSid)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("valid join returns 200 with in_progress status", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, joinURL, http.NoBody)
		req.Header.Set("X-API-Key", testAPIToken)
		req.Header.Set("X-API-Session", joinerSid)
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
			} `json:"game"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
		assert.Equal(t, gameID, body.Game.ID)
		assert.Equal(t, "in_progress", body.Game.Status)
		assert.Len(t, body.Game.Players, 2)
	})
}
