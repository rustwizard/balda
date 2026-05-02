package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/flname"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
)

// Signup implements baldaapi.Handler.
func (h *Handlers) Signup(ctx context.Context, req *baldaapi.SignupRequest) (baldaapi.SignupRes, error) {
	slog.Info("signup handler called")

	created, err := h.svc.CreateUser(ctx, req.Firstname, req.Lastname, req.Email, req.Password, flname.GenNickname())
	if err != nil {
		slog.Error("signup: create user", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	sid, err := uuid.NewRandom()
	if err != nil {
		slog.Error("signup: gen session id", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	if err = h.sess.Save(&session.User{Sid: sid.String(), UID: created.UID}); err != nil {
		slog.Error("signup: save session", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	cfToken, lobbyToken, err := h.generateCentrifugoTokens(created.UID)
	if err != nil {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString(""),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("SignUp Error"),
		}, nil
	}

	return &baldaapi.SignupResponse{
		User: baldaapi.NewOptPlayer(baldaapi.Player{
			UID:       baldaapi.NewOptUUID(created.PlayerID),
			Firstname: baldaapi.NewOptString(req.Firstname),
			Lastname:  baldaapi.NewOptString(req.Lastname),
			Sid:       baldaapi.NewOptString(sid.String()),
			Key:       baldaapi.NewOptString(created.APIKey),
			Exp:       baldaapi.NewOptInt64(0),
		}),
		CentrifugoToken: baldaapi.NewOptString(cfToken),
		LobbyToken:      baldaapi.NewOptString(lobbyToken),
	}, nil
}
