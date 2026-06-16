package tests

import (
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

	creatorCtx, _ := signupCtx(t, h, "propose.creator@example.org")
	joinerCtx, _ := signupCtx(t, h, "propose.joiner@example.org")

	createRes, err := h.CreateGame(creatorCtx)
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	_, err = h.JoinGame(joinerCtx, baldaapi.JoinGameParams{ID: gameID})
	require.NoError(t, err)

	t.Run("missing claims returns 401", func(t *testing.T) {
		res, err := h.ProposeEndGame(context.Background(), baldaapi.ProposeEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameUnauthorized)
		require.True(t, ok, "expected *ProposeEndGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("unknown game id returns 404", func(t *testing.T) {
		res, err := h.ProposeEndGame(creatorCtx, baldaapi.ProposeEndGameParams{ID: uuid.New()})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameNotFound)
		require.True(t, ok, "expected *ProposeEndGameNotFound, got %T", res)
		assert.Equal(t, http.StatusNotFound, errResp.Status.Value)
	})

	t.Run("not player's turn returns 409", func(t *testing.T) {
		res, err := h.ProposeEndGame(joinerCtx, baldaapi.ProposeEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameConflict)
		require.True(t, ok, "expected *ProposeEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("current player proposes returns 204", func(t *testing.T) {
		res, err := h.ProposeEndGame(creatorCtx, baldaapi.ProposeEndGameParams{ID: gameID})
		require.NoError(t, err)
		_, ok := res.(*baldaapi.ProposeEndGameNoContent)
		require.True(t, ok, "expected *ProposeEndGameNoContent, got %T", res)
	})

	t.Run("proposing again while proposal pending returns 409", func(t *testing.T) {
		res, err := h.ProposeEndGame(creatorCtx, baldaapi.ProposeEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.ProposeEndGameConflict)
		require.True(t, ok, "expected *ProposeEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})
}

func TestAcceptEndGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	creatorCtx, _ := signupCtx(t, h, "accept.creator@example.org")
	joinerCtx, _ := signupCtx(t, h, "accept.joiner@example.org")

	createRes, err := h.CreateGame(creatorCtx)
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	_, err = h.JoinGame(joinerCtx, baldaapi.JoinGameParams{ID: gameID})
	require.NoError(t, err)

	t.Run("missing claims returns 401", func(t *testing.T) {
		res, err := h.AcceptEndGame(context.Background(), baldaapi.AcceptEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.AcceptEndGameUnauthorized)
		require.True(t, ok, "expected *AcceptEndGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("no proposal pending returns 409", func(t *testing.T) {
		res, err := h.AcceptEndGame(joinerCtx, baldaapi.AcceptEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.AcceptEndGameConflict)
		require.True(t, ok, "expected *AcceptEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	_, err = h.ProposeEndGame(creatorCtx, baldaapi.ProposeEndGameParams{ID: gameID})
	require.NoError(t, err)

	t.Run("proposer cannot accept their own proposal returns 409", func(t *testing.T) {
		res, err := h.AcceptEndGame(creatorCtx, baldaapi.AcceptEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.AcceptEndGameConflict)
		require.True(t, ok, "expected *AcceptEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("opponent accepts returns 204", func(t *testing.T) {
		res, err := h.AcceptEndGame(joinerCtx, baldaapi.AcceptEndGameParams{ID: gameID})
		require.NoError(t, err)
		_, ok := res.(*baldaapi.AcceptEndGameNoContent)
		require.True(t, ok, "expected *AcceptEndGameNoContent, got %T", res)
	})
}

func TestRejectEndGameHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	creatorCtx, _ := signupCtx(t, h, "reject.creator@example.org")
	joinerCtx, _ := signupCtx(t, h, "reject.joiner@example.org")

	createRes, err := h.CreateGame(creatorCtx)
	require.NoError(t, err)
	gameID := createRes.(*baldaapi.CreateGameResponse).Game.Value.ID.Value

	_, err = h.JoinGame(joinerCtx, baldaapi.JoinGameParams{ID: gameID})
	require.NoError(t, err)

	t.Run("missing claims returns 401", func(t *testing.T) {
		res, err := h.RejectEndGame(context.Background(), baldaapi.RejectEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.RejectEndGameUnauthorized)
		require.True(t, ok, "expected *RejectEndGameUnauthorized, got %T", res)
		assert.Equal(t, http.StatusUnauthorized, errResp.Status.Value)
	})

	t.Run("no proposal pending returns 409", func(t *testing.T) {
		res, err := h.RejectEndGame(joinerCtx, baldaapi.RejectEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.RejectEndGameConflict)
		require.True(t, ok, "expected *RejectEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	_, err = h.ProposeEndGame(creatorCtx, baldaapi.ProposeEndGameParams{ID: gameID})
	require.NoError(t, err)

	t.Run("proposer cannot reject their own proposal returns 409", func(t *testing.T) {
		res, err := h.RejectEndGame(creatorCtx, baldaapi.RejectEndGameParams{ID: gameID})
		require.NoError(t, err)
		errResp, ok := res.(*baldaapi.RejectEndGameConflict)
		require.True(t, ok, "expected *RejectEndGameConflict, got %T", res)
		assert.Equal(t, http.StatusConflict, errResp.Status.Value)
	})

	t.Run("opponent rejects returns 204", func(t *testing.T) {
		res, err := h.RejectEndGame(joinerCtx, baldaapi.RejectEndGameParams{ID: gameID})
		require.NoError(t, err)
		_, ok := res.(*baldaapi.RejectEndGameNoContent)
		require.True(t, ok, "expected *RejectEndGameNoContent, got %T", res)
	})
}

func TestProposeEndGameHTTP(t *testing.T) {
	srv, creatorToken, cleanup := setupServer(t)
	defer cleanup()

	joinerToken := postSignup(t, srv, "http.propose.joiner@example.org", "pass")

	createReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/balda/api/v1/games", http.NoBody)
	createReq.Header.Set("Authorization", "Bearer "+creatorToken)
	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusOK, createResp.StatusCode)
	var createBody struct {
		Game struct {
			ID string `json:"id"`
		} `json:"game"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&createBody))
	gameID := createBody.Game.ID

	joinReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/balda/api/v1/games/%s/join", srv.URL, gameID), http.NoBody)
	joinReq.Header.Set("Authorization", "Bearer "+joinerToken)
	joinResp, err := http.DefaultClient.Do(joinReq)
	require.NoError(t, err)
	defer joinResp.Body.Close()
	require.Equal(t, http.StatusOK, joinResp.StatusCode)

	proposeURL := fmt.Sprintf("%s/balda/api/v1/games/%s/propose-end", srv.URL, gameID)

	t.Run("missing token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, proposeURL, http.NoBody)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("wrong turn returns 409", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, proposeURL, http.NoBody)
		req.Header.Set("Authorization", "Bearer "+joinerToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("valid propose returns 204", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, proposeURL, http.NoBody)
		req.Header.Set("Authorization", "Bearer "+creatorToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}
