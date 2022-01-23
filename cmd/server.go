package cmd

import (
	"log"

	"github.com/spf13/pflag"

	"github.com/go-openapi/loads"
	"github.com/rustwizard/balda/internal/server/restapi"
	"github.com/rustwizard/balda/internal/server/restapi/operations"
	"github.com/spf13/cobra"
)

var cfg Config

type Config struct {
	Addr string
	Port int
}

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Balda Game Server",
	Run: func(cmd *cobra.Command, args []string) {
		swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
		if err != nil {
			log.Fatalln(err)
		}
		api := operations.NewBaldaGameServerAPI(swaggerSpec)
		server := restapi.NewServer(api)
		server.Port = 9666
		defer func(server *restapi.Server) {
			err := server.Shutdown()
			if err != nil {
				log.Println(err)
			}
		}(server)

		server.ConfigureAPI()

		if err := server.Serve(); err != nil {
			log.Fatalln(err)
		}
	},
}

// Flags ...
func (c *Config) Flags(prefix string) *pflag.FlagSet {
	if prefix != "" {
		prefix += "."
	}

	f := pflag.NewFlagSet("", pflag.PanicOnError)
	f.StringVar(&c.Addr, prefix+"addr", "127.0.0.1", "server addr")
	f.IntVar(&c.Port, prefix+"port", 9666, "server port")
	return f
}

func init() {
	serverCmd.Flags().AddFlagSet(cfg.Flags("server"))
	rootCmd.AddCommand(serverCmd)
}
