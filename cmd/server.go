package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rustwizard/balda/api/openapi"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/rustwizard/cleargo/db/pg"

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

type Config struct {
	ServerAddr string
	ServerPort int
	Pg         pg.Config
	Session    session.Config
	XAPIToken  string
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Balda Game Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		flags.BindEnv(cmd)

		dbVersion, err := migrations.Migrate(10 * time.Second)
		if err != nil {
			slog.Error("failed to migrate database", slog.Any("error", err))
			return fmt.Errorf("failed to migrate database: %v", err)
		}

		slog.Info("database migration success", slog.Int("db_version", dbVersion))

		db := pg.NewDB()
		err = db.Connect(&pg.Config{
			Host:         cfg.Pg.Host,
			Port:         cfg.Pg.Port,
			User:         cfg.Pg.User,
			Password:     cfg.Pg.Password,
			DatabaseName: cfg.Pg.DatabaseName,
			MaxPoolSize:  cfg.Pg.MaxPoolSize,
			SSL:          cfg.Pg.SSL,
		})
		if err != nil {
			return fmt.Errorf("connect to pg: %v", err)
		}

		sess := session.NewService(cfg.Session)
		h := handlers.New(db, sess, cfg.XAPIToken)

		srv, err := baldaapi.NewServer(h, h, baldaapi.WithPathPrefix("/balda/api/v1"))
		if err != nil {
			return fmt.Errorf("create ogen server: %v", err)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/balda/api/v1/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			w.Write(openapi.Spec)
		})
		mux.HandleFunc("/balda/api/v1/docs", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, docsHTML)
		})
		mux.Handle("/", srv)

		addr := fmt.Sprintf("%s:%d", cfg.ServerAddr, cfg.ServerPort)
		slog.Info("starting server", slog.String("addr", addr))
		if err := http.ListenAndServe(addr, mux); err != nil {
			return fmt.Errorf("server serve: %v", err)
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
	f.IntVar(&c.Pg.MaxPoolSize, "pg.max_pool_size", 0, "postgres max pool size")
	f.StringVar(&c.Pg.SSL, "pg.ssl", "disable", "postgres ssl")
	f.StringVar(&c.XAPIToken, prefix+"x_api_token", "", "x-api-token for header or query param")
	return f
}

func init() {
	serverCmd.Flags().AddFlagSet(cfg.Flags("server"))
	serverCmd.Flags().AddFlagSet(cfg.Session.Flags("redis"))
}
