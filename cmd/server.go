package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rustwizard/balda/api/openapi"
	"github.com/rustwizard/balda/internal/centrifugo"
	"github.com/rustwizard/balda/internal/game"
	"github.com/rustwizard/balda/internal/game/bot"
	"github.com/rustwizard/balda/internal/gamecoord"
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

type Config struct {
	ServerAddr string
	ServerPort int
	Pg         PgConfig
	Session    session.Config
	Presence   presence.Config
	Centrifugo CentrifugoConfig
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
		sess := session.NewService(cfg.Session, rdb)
		pres := presence.NewService(cfg.Presence, rdb)

		cf := centrifugo.NewClient(cfg.Centrifugo.APIURL, cfg.Centrifugo.APIKey)

		s := storage.New(pool, 10*time.Second)

		var pendingResults sync.WaitGroup

		// The notifier argument is intentionally ignored: the factory builds its own
		// CompositeNotifier combining gamecoord.Coordinator and, for bot players,
		// bot.Notifier.
		lby := lobby.New(func(_ context.Context, gameID string, players []*game.Player, _ game.Notifier) (*game.Game, error) {
			coord := gamecoord.New(gameID, players, cf)
			// Bot games are intentionally not persisted: the bot is not a real
			// player_state row, so there is no valid player_id for game_results.
			// game_over is still published to Centrifugo so the frontend can finish
			// the game UI normally.
			if !hasBotPlayer(players) {
				coord.SetOnGameOver(makeOnGameOverCallback(s, &pendingResults))
			}

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

			g, err := game.NewGame(players, game.NewCompositeNotifier(notifiers...))
			if err != nil {
				return nil, err
			}
			coord.SetGame(g)
			if botNotifier != nil {
				botNotifier.SetGame(g)
			}
			return g, nil
		})
		mm := matchmaking.New(matchmaking.DefaultConfig(), func(players []*game.Player) error {
			_, err := lby.StartGame(cmd.Context(), players, &game.NoopNotifier{})
			return err
		})

		svc := service.New(lby, mm, s)

		h := handlers.New(svc, sess, pres, cf, cfg.Centrifugo.TokenHMACSecret)

		srv, err := baldaapi.NewServer(h, h, baldaapi.WithPathPrefix("/balda/api/v1"))
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

// hasBotPlayer reports whether any player in the list is a bot.
func hasBotPlayer(players []*game.Player) bool {
	for _, p := range players {
		if p.Type == game.PlayerTypeBot {
			return true
		}
	}
	return false
}

// gameResultSaver matches *storage.Storage so the callback can be unit-tested.
type gameResultSaver interface {
	SaveGameResult(ctx context.Context, r storage.GameResult) error
}

// makeOnGameOverCallback returns a callback that persists a game result with
// retry and exponential backoff (100 ms, 200 ms). It accounts its work in
// pending so the server can drain in-flight saves during graceful shutdown.
func makeOnGameOverCallback(saver gameResultSaver, pending *sync.WaitGroup) func(storage.GameResult) {
	return func(r storage.GameResult) {
		pending.Add(1)
		defer pending.Done()

		var err error
		for i := 0; i < 3; i++ {
			if i > 0 {
				time.Sleep(time.Duration(i) * 100 * time.Millisecond)
			}
			err = saver.SaveGameResult(context.Background(), r)
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
		}
	}
}
