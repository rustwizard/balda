package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rustwizard/balda/internal/auth"
	"github.com/rustwizard/balda/internal/flname"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
)

// Signup implements baldaapi.Handler.
func (h *Handlers) Signup(ctx context.Context, req *baldaapi.SignupRequest) (baldaapi.SignupRes, error) {
	if !h.emailSignupEnabled {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("registration is disabled"),
			Status:  baldaapi.NewOptInt(http.StatusForbidden),
			Type:    baldaapi.NewOptString("Forbidden"),
		}, nil
	}

	created, err := h.svc.CreateUser(ctx, req.Firstname, req.Lastname, req.Email, req.Password, flname.GenNickname())
	if err != nil {
		slog.Error("signup: create user", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("failed to create user"),
			Status:  baldaapi.NewOptInt(http.StatusBadRequest),
			Type:    baldaapi.NewOptString("BadRequest"),
		}, nil
	}

	access, refresh, err := h.issueTokens(ctx, created.UID, created.PlayerID, created.Role, "", "")
	if err != nil {
		slog.Error("signup: issue tokens", slog.Any("error", err))
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	cfToken, lobbyToken, err := h.generateCentrifugoTokens(created.UID)
	if err != nil {
		return &baldaapi.ErrorResponse{
			Message: baldaapi.NewOptString("internal error"),
			Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
			Type:    baldaapi.NewOptString("InternalServerError"),
		}, nil
	}

	return &baldaapi.SignupResponse{
		User: baldaapi.NewOptPlayer(baldaapi.Player{
			UID:       baldaapi.NewOptUUID(created.PlayerID),
			Firstname: baldaapi.NewOptString(req.Firstname),
			Lastname:  baldaapi.NewOptString(req.Lastname),
			Exp:       baldaapi.NewOptInt64(0),
			Rating:    baldaapi.NewOptInt64(created.Rating),
		}),
		AccessToken:     baldaapi.NewOptString(access),
		RefreshToken:    baldaapi.NewOptString(refresh),
		TokenType:       baldaapi.NewOptString("Bearer"),
		ExpiresIn:       baldaapi.NewOptInt(int(auth.AccessTokenTTL.Seconds())),
		CentrifugoToken: baldaapi.NewOptString(cfToken),
		LobbyToken:      baldaapi.NewOptString(lobbyToken),
	}, nil
}
