package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rustwizard/balda/api/openapi"
	"github.com/rustwizard/balda/internal/achievements"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/game/bot"
	"github.com/rustwizard/balda/internal/gamecoord"
	"github.com/rustwizard/balda/internal/leaderboard"
	"github.com/rustwizard/balda/internal/lobby"
	"github.com/rustwizard/balda/internal/matchmaking"
	"github.com/rustwizard/balda/internal/presence"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/rustwizard/balda/internal/service"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/spf13/pflag"

	baldaapi "github.com/rustwizard/balda/internal/server/ogen"
	"github.com/rustwizard/balda/internal/session"
	"github.com/rustwizard/balda/migrations"
	"github.com/spf13/cobra"

	"log/slog"

	"github.com/rustwizard/cleargo/infra/flags"
)

const docsHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Balda GameServer API</title>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
window.onload = function() {
  SwaggerUIBundle({
    url: "/balda/api/v1/docs/openapi.yaml",
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
    layout: "BaseLayout"
  })
}
</script>
</body>
</html>`

var cfg Config

type PgConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	DatabaseName string
	MaxPoolSize  int
	SSL          string
}

type CentrifugoConfig struct {
	APIURL          string
	APIKey          string
	TokenHMACSecret string
}

type AuthConfig struct {
	JWTSecret string
	// EmailSignupEnabled allows email/password registration. Disabled in
	// prod: signup stays available for local testing while real users come
	// through Telegram auth.
	EmailSignupEnabled bool
}

type TelegramConfig struct {
	// BotToken validates Telegram Mini App init data. Empty means Telegram
	// auth is not configured; the endpoint then answers 503.
	BotToken string
	// AppURL is the public Mini App link (https://t.me/<bot>/<app>) used by
	// the client to build friend-invite links. Empty hides the invite button.
	AppURL string
}

type Config struct {
	ServerAddr string
	ServerPort int
	Pg         PgConfig
	Session    session.Config
	Presence   presence.Config
	Auth       AuthConfig
	Centrifugo CentrifugoConfig
	Telegram   TelegramConfig
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Balda Game Server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		flags.BindEnv(cmd)

		dbVersion, err := migrations.Migrate(10 * time.Second)
		if err != nil {
			slog.Error("failed to migrate database", slog.Any("error", err))
			return fmt.Errorf("failed to migrate database: %w", err)
		}

		slog.Info("database migration success", slog.Int("db_version", dbVersion))

		connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d",
			cfg.Pg.User, cfg.Pg.Password, cfg.Pg.Host, cfg.Pg.Port,
			cfg.Pg.DatabaseName, cfg.Pg.SSL, cfg.Pg.MaxPoolSize,
		)
		pool, err := pgxpool.New(cmd.Context(), connStr)
		if err != nil {
			return fmt.Errorf("connect to pg: %w", err)
		}
		defer pool.Close()

		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.Session.Addr,
			Username: cfg.Session.Username,
			Password: cfg.Session.Password,
			DB:       cfg.Session.DBNum,
		})
		pres := presence.NewService(cfg.Presence, rdb)

		cf := centrifugo.NewClient(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)

		s := storage.New(pool, 10*time.Second)

		achSvc := achievements.NewService(s.LoadAchievementDefinitions)
		if err := achSvc.Load(cmd.Context()); err != nil {
			return fmt.Errorf("load achievements: %w", err)
		}
		go achSvc.Start(cmd.Context(), 1*time.Minute)

		var pendingResults sync.WaitGroup

		// The notifier argument is intentionally ignored: the factory builds its own
		// CompositeNotifier combining gamecoord.Coordinator and, for bot players,
		// bot.Notifier.
		lby := lobby.New(func(_ context.Context, gameID string, players []*game.Player, _ game.Notifier) (*game.Game, error) {
			coord := gamecoord.New(gameID, players, cf)
			// Bot games are persisted too: only the human side is written
			// (rating, EXP, stats) since the bot has no player_state row.
			coord.SetOnGameOver(makeOnGameOverCallback(s, cf, achSvc, &pendingResults))

			var notifiers []game.Notifier
			notifiers = append(notifiers, coord)

			var botNotifier *bot.Notifier
			for _, p := range players {
				if p.Type == game.PlayerTypeBot {
					botNotifier = bot.NewNotifier(bot.NewEngine(bot.NewRandomValidStrategy(game.Dict)), p.ID)
					notifiers = append(notifiers, botNotifier)
					break
				}
			}

			g, err := game.NewGame(players, game.NewCompositeNotifier(notifiers...),
				game.WithOnlineChecker(presenceChecker{pres}))
			if err != nil {
				return nil, err
			}
			coord.SetGame(g)
			if botNotifier != nil {
				botNotifier.SetGame(g)
			}
			return g, nil
		})
		// publishMatchFound tells matched players (via the lobby channel) which
		// game to enter, with per-player subscription tokens and the board
		// snapshot so the client does not race the first turn events.
		publishMatchFound := func(rec *lobby.GameRecord, players []*game.Player, vsBot bool) {
			ev := centrifugo.EvMatchFound{
				Type:           "match_found",
				GameID:         rec.ID,
				VsBot:          vsBot,
				Board:          rec.Game.BoardSnapshot(),
				CurrentTurnUID: players[0].ID,
				Players:        make([]centrifugo.MatchFoundPlayer, 0, len(players)),
			}
			for _, p := range players {
				mp := centrifugo.MatchFoundPlayer{UID: p.ID, Exp: p.Exp, Rating: p.Rating}
				if p.Type != game.PlayerTypeBot {
					pid, err := uuid.Parse(p.ID)
					if err != nil {
						slog.Error("match_found: parse player id", slog.String("playerID", p.ID), slog.Any("error", err))
						continue
					}
					uid, err := s.GetUIDByPlayerID(context.Background(), pid)
					if err != nil {
						slog.Error("match_found: load uid", slog.String("playerID", p.ID), slog.Any("error", err))
						continue
					}
					token, err := centrifugo.GenerateSubscriptionToken(
						strconv.FormatInt(uid, 10), centrifugo.ChannelGame(rec.ID), cfg.Centrifugo.TokenHMACSecret, 24*time.Hour,
					)
					if err != nil {
						slog.Error("match_found: generate game token", slog.String("playerID", p.ID), slog.Any("error", err))
						continue
					}
					mp.GameToken = token
				}
				ev.Players = append(ev.Players, mp)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := cf.Publish(ctx, centrifugo.ChannelLobby, ev); err != nil {
				slog.Error("match_found: publish", slog.String("gameID", rec.ID), slog.Any("error", err))
			}
		}

		// startBotGame falls back to a bot game when no human opponent is
		// available. Used both by the matchmaking expire callback and
		// synchronously by QuickMatchJoin when the queue is empty.
		startBotGame := func(p *game.Player) error {
			players := []*game.Player{p, {ID: bot.BotPlayerID, Rating: storage.DefaultRating, Type: game.PlayerTypeBot}}
			rec, err := lby.StartGame(context.Background(), players, &game.NoopNotifier{})
			if err != nil {
				return err
			}
			publishMatchFound(rec, players, true)
			return nil
		}
		mm := matchmaking.New(matchmaking.DefaultConfig(), func(players []*game.Player) error {
			rec, err := lby.StartGame(context.Background(), players, &game.NoopNotifier{})
			if err != nil {
				return err
			}
			publishMatchFound(rec, players, false)
			return nil
		}).WithExpireCallback(startBotGame).WithOnlineChecker(presenceChecker{pres}.IsOnline)
		go mm.Run(cmd.Context())

		lb := leaderboard.NewService(s, rdb, 5*time.Minute)

		svc := service.New(lby, mm, s, lb, achSvc).WithBotFallback(startBotGame)

		h := handlers.New(svc, pres, cf, handlers.Config{
			JWTSecret:                 cfg.Auth.JWTSecret,
			CentrifugoTokenHMACSecret: cfg.Centrifugo.TokenHMACSecret,
			EmailSignupEnabled:        cfg.Auth.EmailSignupEnabled,
			TelegramBotToken:          cfg.Telegram.BotToken,
			TelegramAppURL:            cfg.Telegram.AppURL,
		})

		srv, err := baldaapi.NewServer(h, h,
			baldaapi.WithPathPrefix("/balda/api/v1"),
			baldaapi.WithErrorHandler(handlers.ErrorHandler),
		)
		if err != nil {
			return fmt.Errorf("create ogen server: %w", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/balda/api/v1/docs/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			_, _ = w.Write(openapi.Spec)
		})
		mux.HandleFunc("/balda/api/v1/docs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, docsHTML)
		})
		mux.Handle("/", srv)

		addr := fmt.Sprintf("%s:%d", cfg.ServerAddr, cfg.ServerPort)
		httpSrv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
		}

		go func() {
			slog.Info("starting server", slog.String("addr", addr))
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("server serve", slog.Any("error", err))
			}
		}()

		<-cmd.Context().Done()

		slog.Info("shutting down server")

		// Cancel all running games so their goroutines exit cleanly.
		lby.Shutdown()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", slog.Any("error", err))
		}

		// Wait for any in-flight SaveGameResult calls to finish.
		done := make(chan struct{})
		go func() {
			pendingResults.Wait()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("all pending game results saved")
		case <-shutdownCtx.Done():
			slog.Warn("shutdown timeout exceeded, some game results may be lost")
		}

		return nil
	},
}

func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix += "."
	}

	f := pflag.NewFlagSet("", pflag.PanicOnError)
	f.StringVar(&c.ServerAddr, prefix+"addr", "127.0.0.1", "server addr")
	f.IntVar(&c.ServerPort, prefix+"port", 9666, "server port")
	f.StringVar(&c.Pg.Host, "pg.host", "127.0.0.1", "postgres addr")
	f.IntVar(&c.Pg.Port, "pg.port", 5432, "postgres port")
	f.StringVar(&c.Pg.User, "pg.user", "", "postgres user")
	f.StringVar(&c.Pg.DatabaseName, "pg.database", "", "postgres database")
	f.StringVar(&c.Pg.Password, "pg.password", "", "postgres password")
	f.IntVar(&c.Pg.MaxPoolSize, "pg.max_pool_size", 10, "postgres max pool size")
	f.StringVar(&c.Pg.SSL, "pg.ssl", "disable", "postgres ssl")
	f.StringVar(&c.Auth.JWTSecret, "auth.jwt_secret", "", "HMAC secret for signing JWT access tokens")
	f.BoolVar(&c.Auth.EmailSignupEnabled, "auth.email_signup_enabled", true, "allow email/password registration (disable in prod)")
	f.StringVar(&c.Telegram.BotToken, "telegram.bot_token", "", "telegram bot token for Mini App auth (init data validation)")
	f.StringVar(&c.Telegram.AppURL, "telegram.app_url", "", "public Mini App URL (https://t.me/<bot>/<app>) for friend invites")
	f.StringVar(&c.Centrifugo.APIURL, "centrifugo.api_url", "http://127.0.0.1:8000/api", "centrifugo api url")
	f.StringVar(&c.Centrifugo.APIKey, "centrifugo.api_key", "", "centrifugo api key")
	f.StringVar(&c.Centrifugo.TokenHMACSecret, "centrifugo.token_hmac_secret_key", "", "centrifugo token hmac secret")
	return f
}

func init() {
	serverCmd.Flags().AddFlagSet(cfg.Flags("server"))
	serverCmd.Flags().AddFlagSet(cfg.Session.Flags("redis"))
	serverCmd.Flags().AddFlagSet(cfg.Presence.Flags("presence"))
}

// presenceChecker adapts *presence.Service to game.OnlineChecker, bridging the
// no-context FSM call to the ctx-based presence lookup with a short timeout.
type presenceChecker struct {
	p *presence.Service
}

func (c presenceChecker) IsOnline(playerID string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return c.p.IsOnline(ctx, playerID)
}

// gameResultSaver matches *storage.Balda so the callback can be unit-tested.
type gameResultSaver interface {
	SaveGameResultWithAchievements(ctx context.Context, r storage.GameResult, achSvc *achievements.Service) ([]storage.PlayerAchievementUnlock, error)
}

// makeOnGameOverCallback returns a callback that persists a game result with
// retry and exponential backoff (100 ms, 200 ms), updates achievement counters,
// and publishes achievement_unlocked events. It accounts its work in pending
// so the server can drain in-flight saves during graceful shutdown.
func makeOnGameOverCallback(saver gameResultSaver, cf *centrifugo.Client, achSvc *achievements.Service, pending *sync.WaitGroup) func(storage.GameResult) {
	return func(r storage.GameResult) {
		pending.Add(1)
		defer pending.Done()

		var unlocked []storage.PlayerAchievementUnlock
		var err error
		for i := 0; i < 3; i++ {
			if i > 0 {
				time.Sleep(time.Duration(i) * 100 * time.Millisecond)
			}
			unlocked, err = saver.SaveGameResultWithAchievements(context.Background(), r, achSvc)
			if err == nil {
				break
			}
			slog.Warn("save game result failed, retrying",
				slog.Int("attempt", i+1),
				slog.String("gameID", r.GameID),
				slog.Any("error", err),
			)
		}
		if err != nil {
			slog.Error("save game result failed after retries",
				slog.String("gameID", r.GameID),
				slog.Any("error", err),
			)
			return
		}

		if cf == nil || len(unlocked) == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		for _, pu := range unlocked {
			for _, a := range pu.Unlocked {
				ev := centrifugo.EvAchievementUnlocked{
					Type:          "achievement_unlocked",
					GameID:        r.GameID,
					PlayerUID:     pu.PlayerID,
					AchievementID: a.ID,
					Name:          a.Name,
				}
				if err := cf.Publish(ctx, centrifugo.ChannelGame(r.GameID), ev); err != nil {
					slog.Error("publish achievement_unlocked",
						slog.String("gameID", r.GameID),
						slog.String("playerID", pu.PlayerID),
						slog.String("achievement", a.ID),
						slog.Any("error", err),
					)
				}
			}
		}
	}
}
