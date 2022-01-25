package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/rustwizard/balda/internal/flags"
	"github.com/rustwizard/balda/internal/server/restapi/handlers"
	"github.com/rustwizard/cleargo/db/pg"

	"github.com/spf13/pflag"

	"github.com/go-openapi/loads"
	"github.com/rustwizard/balda/internal/server/restapi"
	"github.com/rustwizard/balda/internal/server/restapi/operations"
	"github.com/spf13/cobra"
)

var cfg Config

type Config struct {
	ServerAddr string
	ServerPort int
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Balda Game Server",
	// TODO: Run -> RunE
	Run: func(cmd *cobra.Command, args []string) {
		flags.BindEnv(cmd)
		swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
		if err != nil {
			log.Fatal().Err(err).Msg("load swagger spec")
		}
		db := pg.NewDB()
		// TODO: config params from flags
		err = db.Connect(&pg.Config{
			Host:         "pg",
			Port:         5432,
			User:         "balda",
			Password:     "password",
			DatabaseName: "balda",
			MaxPoolSize:  100,
			SSL:          "disable",
		})
		if err != nil {
			log.Fatal().Err(err).Msg("load swagger spec")
		}
		api := operations.NewBaldaGameServerAPI(swaggerSpec)
		// handlers
		api.SignupPostSignupHandler = handlers.NewSignUp(db)
		// TODO: call api.Validate()
		// TODO: impl api x-api-key checker
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
			log.Fatal().Err(err).Msg("serve")
		}
	},
}

// Flags ...
func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix += "."
	}

	f := pflag.NewFlagSet("", pflag.PanicOnError)
	f.StringVar(&c.ServerAddr, prefix+"addr", "127.0.0.1", "server addr")
	f.IntVar(&c.ServerPort, prefix+"port", 9666, "server port")
	return f
}

func init() {
	serverCmd.Flags().AddFlagSet(cfg.Flags("server"))
}
