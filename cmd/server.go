package cmd

import (
	"errors"
	"fmt"

	"github.com/rustwizard/balda/internal/session"

	"github.com/rs/zerolog/log"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/rustwizard/cleargo/db/pg"

	"github.com/spf13/pflag"

	"github.com/go-openapi/loads"
	"github.com/rustwizard/balda/internal/server/restapi"
	"github.com/rustwizard/balda/internal/server/restapi/operations"
	"github.com/spf13/cobra"

	"github.com/rustwizard/cleargo/flags"
)

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
		swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
		if err != nil {
			return fmt.Errorf("load swagger spec: %v", err)
		}

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
		// sessions service
		sess := session.NewService(cfg.Session)
		api := operations.NewBaldaGameServerAPI(swaggerSpec)
		// handlers
		api.SignupPostSignupHandler = handlers.NewSignUp(db, sess)
		api.AuthPostAuthHandler = handlers.NewAuth(db, sess)
		api.GetUsersStateUIDHandler = handlers.NewUserState(db, sess)
		api.APIKeyQueryParamAuth = func(token string) (interface{}, error) {
			log.Info().Msg("KeyAuth handler called")
			if token == cfg.XAPIToken {
				return true, nil
			}
			log.Error().Msgf("access attempt with incorrect api key auth: %s", token)

			return nil, errors.New("api key param: token error")
		}

		api.APIKeyHeaderAuth = func(token string) (interface{}, error) {
			log.Info().Msg("KeyAuth handler called")
			if token == cfg.XAPIToken {
				return true, nil
			}
			log.Error().Msgf("access attempt with incorrect api key auth: %s", token)

			return nil, errors.New("api key header: token error")
		}

		api.UseSwaggerUI()

		server := restapi.NewServer(api)
		server.Port = cfg.ServerPort
		server.Host = cfg.ServerAddr
		defer func(server *restapi.Server) {
			err := server.Shutdown()
			if err != nil {
				log.Err(err).Msg("server shutdown")
			}
		}(server)

		server.ConfigureAPI()

		if err := server.Serve(); err != nil {
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
