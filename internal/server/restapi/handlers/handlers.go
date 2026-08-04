package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rustwizard/balda/internal/auth"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/presence"
	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/service"
	"github.com/rustwizard/balda/internal/storage"
)

// Config holds the static configuration Handlers needs at runtime.
type Config struct {
	JWTSecret                 string
	CentrifugoTokenHMACSecret string
	EmailSignupEnabled        bool
	TelegramBotToken          string
	TelegramAppURL            string
}

// Handlers implements baldaapi.Handler and baldaapi.SecurityHandler.
type Handlers struct {
	svc  *service.Balda
	pres *presence.Service
	cf   *centrifugo.Client
	cfg  Config
}

func New(svc *service.Balda, pres *presence.Service, cf *centrifugo.Client, cfg Config) *Handlers {
	return &Handlers{svc: svc, pres: pres, cf: cf, cfg: cfg}
}

// uidFromContext returns the authenticated user's id from the JWT claims placed
// by HandleBearerAuth. ok is false only if the security middleware was bypassed.
func uidFromContext(ctx context.Context) (uid int64, ok bool) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return 0, false
	}
	return c.UID, true
}

// tokenMeta carries optional client metadata attached to a refresh token.
type tokenMeta struct {
	UserAgent string
	IPAddr    string
}

// issueTokens mints an access token and a rotated refresh token for the user,
// persisting the HMAC hash of the refresh token. Returns the raw tokens.
func (h *Handlers) issueTokens(ctx context.Context, uid int64, pid uuid.UUID, role string, meta tokenMeta) (access, refresh string, err error) {
	access, err = auth.GenerateAccessToken(uid, pid, role, h.cfg.JWTSecret)
	if err != nil {
		return "", "", fmt.Errorf("issue tokens: access: %w", err)
	}
	refresh, err = auth.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("issue tokens: refresh: %w", err)
	}
	hash := auth.HashRefreshToken(h.cfg.JWTSecret, refresh)
	if err := h.svc.SaveRefreshToken(ctx, storage.RefreshToken{
		UserID:    uid,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(auth.RefreshTokenTTL),
		UserAgent: meta.UserAgent,
		IPAddr:    meta.IPAddr,
	}); err != nil {
		return "", "", fmt.Errorf("issue tokens: save: %w", err)
	}
	return access, refresh, nil
}

// generateCentrifugoTokens returns a connection token and a lobby subscription token for the given user.
func (h *Handlers) generateCentrifugoTokens(uid int64) (cfToken, lobbyToken string, err error) {
	sub := strconv.FormatInt(uid, 10)
	ttl := 24 * time.Hour
	cfToken, err = centrifugo.GenerateConnectionToken(sub, h.cfg.CentrifugoTokenHMACSecret, ttl)
	if err != nil {
		slog.Error("generate centrifugo connection token", slog.Any("error", err))
		return
	}
	lobbyToken, err = centrifugo.GenerateSubscriptionToken(sub, centrifugo.ChannelLobby, h.cfg.CentrifugoTokenHMACSecret, ttl)
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
			lobbyPlayers = append(lobbyPlayers, centrifugo.LobbyPlayer{UID: p.ID, Exp: p.Exp, Rating: p.Rating})
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

// HandleBearerAuth implements baldaapi.SecurityHandler. It verifies the JWT
// access token and injects the resulting claims into the request context.
// A returned error is rendered by ogen as 401 Unauthorized.
func (h *Handlers) HandleBearerAuth(ctx context.Context, _ baldaapi.OperationName, t baldaapi.BearerAuth) (context.Context, error) {
	claims, err := auth.ParseAccessToken(t.Token, h.cfg.JWTSecret)
	if err != nil {
		return ctx, err
	}
	return auth.WithClaims(ctx, claims), nil
}
