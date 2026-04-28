package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/rustwizard/balda/internal/centrifugo"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/service"
	"github.com/rustwizard/balda/internal/session"
)

// Handlers implements baldaapi.Handler and baldaapi.SecurityHandler.
type Handlers struct {
	svc                     *service.Balda
	sess                    *session.Service
	xAPIToken               string
	cf                      *centrifugo.Client
	centrifugoTokenHMACSecret string
}

func New(svc *service.Balda, sess *session.Service, xAPIToken string, cf *centrifugo.Client, centrifugoTokenHMACSecret string) *Handlers {
	return &Handlers{svc: svc, sess: sess, xAPIToken: xAPIToken, cf: cf, centrifugoTokenHMACSecret: centrifugoTokenHMACSecret}
}

// generateCentrifugoTokens returns a connection token and a lobby subscription token for the given user.
func (h *Handlers) generateCentrifugoTokens(uid int64) (cfToken, lobbyToken string, err error) {
	sub := strconv.FormatInt(uid, 10)
	ttl := 24 * time.Hour
	cfToken, err = centrifugo.GenerateConnectionToken(sub, h.centrifugoTokenHMACSecret, ttl)
	if err != nil {
		slog.Error("generate centrifugo connection token", slog.Any("error", err))
		return
	}
	lobbyToken, err = centrifugo.GenerateSubscriptionToken(sub, centrifugo.ChannelLobby, h.centrifugoTokenHMACSecret, ttl)
	if err != nil {
		slog.Error("generate centrifugo lobby token", slog.Any("error", err))
	}
	return
}

// publishLobbyUpdate fetches the current game list and publishes EvLobbyUpdate
// to the lobby channel so all connected clients refresh without an API call.
func (h *Handlers) publishLobbyUpdate(ctx context.Context) {
	games := h.svc.ListGames()
	ev := centrifugo.EvLobbyUpdate{
		Type:  "lobby_update",
		Games: make([]centrifugo.GameEntry, 0, len(games)),
	}
	for _, g := range games {
		playerIDs := make([]string, 0, len(g.Players))
		lobbyPlayers := make([]centrifugo.LobbyPlayer, 0, len(g.Players))
		for _, p := range g.Players {
			playerIDs = append(playerIDs, p.ID)
			lobbyPlayers = append(lobbyPlayers, centrifugo.LobbyPlayer{UID: p.ID, Exp: p.Exp})
		}
		ev.Games = append(ev.Games, centrifugo.GameEntry{
			ID:        g.ID,
			PlayerIDs: playerIDs,
			Players:   lobbyPlayers,
			Status:    string(g.Status),
			StartedAt: g.StartedAt.UnixMilli(),
		})
	}
	if err := h.cf.Publish(ctx, centrifugo.ChannelLobby, ev); err != nil {
		slog.Error("publish lobby_update", slog.Any("error", err))
	}
}

// HandleAPIKeyHeader implements baldaapi.SecurityHandler.
func (h *Handlers) HandleAPIKeyHeader(ctx context.Context, _ baldaapi.OperationName, t baldaapi.APIKeyHeader) (context.Context, error) {
	slog.Info("KeyAuth header handler called")
	if t.APIKey == h.xAPIToken {
		return ctx, nil
	}
	slog.Error("access attempt with incorrect api key header", slog.String("token", t.APIKey))
	return nil, errors.New("api key header: token error")
}

// HandleAPIKeyQueryParam implements baldaapi.SecurityHandler.
func (h *Handlers) HandleAPIKeyQueryParam(ctx context.Context, _ baldaapi.OperationName, t baldaapi.APIKeyQueryParam) (context.Context, error) {
	slog.Info("KeyAuth query param handler called")
	if t.APIKey == h.xAPIToken {
		return ctx, nil
	}
	slog.Error("access attempt with incorrect api key query param", slog.String("token", t.APIKey))
	return nil, errors.New("api key param: token error")
}
