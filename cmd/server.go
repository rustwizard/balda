package cmd

import (
	"log"

	"github.com/go-openapi/loads"
	"github.com/rustwizard/balda/internal/server/restapi"
	"github.com/rustwizard/balda/internal/server/restapi/operations"
	"github.com/spf13/cobra"
)

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

func init() {
	rootCmd.AddCommand(serverCmd)
}
