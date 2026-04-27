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

func TestProposeEndGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	cRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Creator", Lastname: "User", Email: "propose.creator@example.org", Password: "pass",
	})
	require.NoError(t, err)
	creator := cRes.(*baldaapi.SignupResponse).User.Value

	jRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Joiner", Lastname: "User", Email: "propose.joiner@example.org", Password: "pass",
	})
	require.NoError(t, err)
	joiner := jRes.(*baldaapi.SignupResponse).User.Value

	createRes, err := h.CreateGame(ctx, baldaapi.CreateGameParams{XAPISession: creator.Sid.Value})
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	_, err = h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
	require.NoError(t, err)

	t.Run("unknown session returns 401", func(t *testing.T) {
		res, err := h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: "bad-sid", ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameUnauthorized)
		require.True(t, ok, "expected *ProposeEndGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("unknown game id returns 404", func(t *testing.T) {
		res, err := h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: creator.Sid.Value, ID: uuid.New()})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameNotFound)
		require.True(t, ok, "expected *ProposeEndGameNotFound, got %T", res)
		assert.Equal(t, http.StatusNotFound, errResp.Status.Value)
	})

	t.Run("not player's turn returns 409", func(t *testing.T) {
		res, err := h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameConflict)
		require.True(t, ok, "expected *ProposeEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("current player proposes returns 204", func(t *testing.T) {
		res, err := h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: creator.Sid.Value, ID: gameID})
		require.NoError(t, err)
		_, ok := res.(*baldaapi.ProposeEndGameNoContent)
		require.True(t, ok, "expected *ProposeEndGameNoContent, got %T", res)
	})

	t.Run("proposing again while proposal pending returns 409", func(t *testing.T) {
		res, err := h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: creator.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameConflict)
		require.True(t, ok, "expected *ProposeEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})
}

func TestAcceptEndGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	cRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Creator", Lastname: "User", Email: "accept.creator@example.org", Password: "pass",
	})
	require.NoError(t, err)
	creator := cRes.(*baldaapi.SignupResponse).User.Value

	jRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Joiner", Lastname: "User", Email: "accept.joiner@example.org", Password: "pass",
	})
	require.NoError(t, err)
	joiner := jRes.(*baldaapi.SignupResponse).User.Value

	createRes, err := h.CreateGame(ctx, baldaapi.CreateGameParams{XAPISession: creator.Sid.Value})
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	_, err = h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
	require.NoError(t, err)

	t.Run("unknown session returns 401", func(t *testing.T) {
		res, err := h.AcceptEndGame(ctx, baldaapi.AcceptEndGameParams{XAPISession: "bad-sid", ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.AcceptEndGameUnauthorized)
		require.True(t, ok, "expected *AcceptEndGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("no proposal pending returns 409", func(t *testing.T) {
		res, err := h.AcceptEndGame(ctx, baldaapi.AcceptEndGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.AcceptEndGameConflict)
		require.True(t, ok, "expected *AcceptEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	_, err = h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: creator.Sid.Value, ID: gameID})
	require.NoError(t, err)

	t.Run("proposer cannot accept their own proposal returns 409", func(t *testing.T) {
		res, err := h.AcceptEndGame(ctx, baldaapi.AcceptEndGameParams{XAPISession: creator.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.AcceptEndGameConflict)
		require.True(t, ok, "expected *AcceptEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("opponent accepts returns 204", func(t *testing.T) {
		res, err := h.AcceptEndGame(ctx, baldaapi.AcceptEndGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
		require.NoError(t, err)
		_, ok := res.(*baldaapi.AcceptEndGameNoContent)
		require.True(t, ok, "expected *AcceptEndGameNoContent, got %T", res)
	})
}

func TestRejectEndGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ctx := context.Background()

	cRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Creator", Lastname: "User", Email: "reject.creator@example.org", Password: "pass",
	})
	require.NoError(t, err)
	creator := cRes.(*baldaapi.SignupResponse).User.Value

	jRes, err := h.Signup(ctx, &baldaapi.SignupRequest{
		Firstname: "Joiner", Lastname: "User", Email: "reject.joiner@example.org", Password: "pass",
	})
	require.NoError(t, err)
	joiner := jRes.(*baldaapi.SignupResponse).User.Value

	createRes, err := h.CreateGame(ctx, baldaapi.CreateGameParams{XAPISession: creator.Sid.Value})
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	_, err = h.JoinGame(ctx, baldaapi.JoinGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
	require.NoError(t, err)

	t.Run("unknown session returns 401", func(t *testing.T) {
		res, err := h.RejectEndGame(ctx, baldaapi.RejectEndGameParams{XAPISession: "bad-sid", ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.RejectEndGameUnauthorized)
		require.True(t, ok, "expected *RejectEndGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("no proposal pending returns 409", func(t *testing.T) {
		res, err := h.RejectEndGame(ctx, baldaapi.RejectEndGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.RejectEndGameConflict)
		require.True(t, ok, "expected *RejectEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	_, err = h.ProposeEndGame(ctx, baldaapi.ProposeEndGameParams{XAPISession: creator.Sid.Value, ID: gameID})
	require.NoError(t, err)

	t.Run("proposer cannot reject their own proposal returns 409", func(t *testing.T) {
		res, err := h.RejectEndGame(ctx, baldaapi.RejectEndGameParams{XAPISession: creator.Sid.Value, ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.RejectEndGameConflict)
		require.True(t, ok, "expected *RejectEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("opponent rejects returns 204", func(t *testing.T) {
		res, err := h.RejectEndGame(ctx, baldaapi.RejectEndGameParams{XAPISession: joiner.Sid.Value, ID: gameID})
		require.NoError(t, err)
		_, ok := res.(*baldaapi.RejectEndGameNoContent)
		require.True(t, ok, "expected *RejectEndGameNoContent, got %T", res)
	})
}

func TestProposeEndGameHTTP(t *testing.T) {
	srv, email, password, cleanup := setupServer(t)
	defer cleanup()

	authBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
	authReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/auth", bytes.NewReader(authBody))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("X-API-Key", testAPIToken)
	authResp, err := http.DefaultClient.Do(authReq)
	require.NoError(t, err)
	defer authResp.Body.Close()
	var authData struct {
		Player struct{ Sid string `json:"sid"` } `json:"player"`
	}
	require.NoError(t, json.NewDecoder(authResp.Body).Decode(&authData))
	creatorSid := authData.Player.Sid

	joinerSid := postSignup(t, srv, "http.propose.joiner@example.org", "pass")

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

	joinReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/balda/api/v1/games/%s/join", srv.URL, gameID), http.NoBody)
	joinReq.Header.Set("X-API-Key", testAPIToken)
	joinReq.Header.Set("X-API-Session", joinerSid)
	joinResp, err := http.DefaultClient.Do(joinReq)
	require.NoError(t, err)
	defer joinResp.Body.Close()
	require.Equal(t, http.StatusOK, joinResp.StatusCode)

	proposeURL := fmt.Sprintf("%s/balda/api/v1/games/%s/propose-end", srv.URL, gameID)

	t.Run("missing api key returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, proposeURL, http.NoBody)
		req.Header.Set("X-API-Session", creatorSid)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("wrong turn returns 409", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, proposeURL, http.NoBody)
		req.Header.Set("X-API-Key", testAPIToken)
		req.Header.Set("X-API-Session", joinerSid)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("valid propose returns 204", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, proposeURL, http.NoBody)
		req.Header.Set("X-API-Key", testAPIToken)
		req.Header.Set("X-API-Session", creatorSid)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}
