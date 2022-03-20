package migrate

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var MigrateRootCmd = &cobra.Command{
	Use:   "migrate",
	Short: "postgresql migrate tools",
}

func Execute() {
	err := MigrateRootCmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("got err while running migrations")
	}
}

func init() {
	MigrateRootCmd.AddCommand(MigrateUp)
}
