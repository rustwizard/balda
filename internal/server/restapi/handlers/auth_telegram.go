package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/rustwizard/balda/internal/auth"
	"github.com/rustwizard/balda/internal/flname"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/storage"
)

// telegramInitDataMaxAge bounds how old the Telegram init data may be.
const telegramInitDataMaxAge = 24 * time.Hour

// AuthTelegram implements baldaapi.Handler. It validates Telegram Mini App
// init data against the bot token, then logs in the linked user or creates a
// new one on first visit.
func (h *Handlers) AuthTelegram(ctx context.Context, req *baldaapi.TelegramAuthRequest) (baldaapi.AuthTelegramRes, error) {
	if h.cfg.TelegramBotToken == "" {
		return &baldaapi.AuthTelegramServiceUnavailable{
			Status:  baldaapi.NewOptInt(http.StatusServiceUnavailable),
			Message: baldaapi.NewOptString("telegram auth is not configured"),
			Type:    baldaapi.NewOptString("ServiceUnavailable"),
		}, nil
	}

	tgUser, err := auth.ValidateInitData(req.InitData, h.cfg.TelegramBotToken, telegramInitDataMaxAge)
	if err != nil {
		return &baldaapi.AuthTelegramUnauthorized{
			Status:  baldaapi.NewOptInt(http.StatusUnauthorized),
			Message: baldaapi.NewOptString("invalid telegram init data"),
			Type:    baldaapi.NewOptString("Unauthorized"),
		}, nil
	}

	u, err := h.svc.GetUserByTelegramID(ctx, tgUser.ID)
	if err != nil {
		if !errors.Is(err, storage.ErrInvalidCredentials) {
			slog.Error("auth telegram: get user", slog.Any("error", err))
			return authTelegramInternalError(), nil
		}

		// First visit: register the user linked to their Telegram account.
		nickname := tgUser.Username
		if nickname == "" {
			nickname = flname.GenNickname()
		}
		created, err := h.svc.CreateTelegramUser(ctx, storage.CreateTelegramUserParams{
			Firstname:  tgUser.FirstName,
			Lastname:   tgUser.LastName,
			Nickname:   nickname,
			TelegramID: tgUser.ID,
		})
		if err != nil {
			slog.Error("auth telegram: create user", slog.Any("error", err))
			return authTelegramInternalError(), nil
		}
		slog.Info("auth telegram: registered new user", slog.Int64("telegram_id", tgUser.ID))
		u = storage.UserAuth{
			UID:       created.UID,
			Firstname: tgUser.FirstName,
			Lastname:  tgUser.LastName,
			PlayerID:  created.PlayerID,
			Rating:    created.Rating,
			Role:      created.Role,
		}
	}

	resp, errResp := h.newAuthResponse(ctx, u)
	if errResp != nil {
		return authTelegramInternalError(), nil
	}
	return resp, nil
}

// authTelegramInternalError maps the shared response builder's generic error
// to this operation's typed 500.
func authTelegramInternalError() *baldaapi.AuthTelegramInternalServerError {
	return &baldaapi.AuthTelegramInternalServerError{
		Status:  baldaapi.NewOptInt(http.StatusInternalServerError),
		Message: baldaapi.NewOptString("internal error"),
		Type:    baldaapi.NewOptString("InternalServerError"),
	}
}
