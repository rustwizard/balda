package migrate

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4/database/pgx"

	"github.com/golang-migrate/migrate/v4"

	"github.com/rustwizard/cleargo/flags"

	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file" // register migrate driver
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var cfg Config

var MigrateUp = &cobra.Command{
	Use:   "up",
	Short: "apply new migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info().Msg("starting migrations")
		flags.BindEnv(cmd)
		p := pgx.Postgres{}
		db, err := p.Open(cfg.BuildDSN())
		if err != nil {
			return fmt.Errorf("migrations: new instance: %w", err)
		}
		m, err := migrate.NewWithDatabaseInstance("file://./migrations", "pgx", db)
		if err != nil {
			return fmt.Errorf("migrations: new instance: %w", err)
		}

		version, dirty, err := m.Version()
		if err != nil && err != migrate.ErrNilVersion {
			return fmt.Errorf("cannot get meta info from database. got err: %w", err)
		}

		log.Info().Uint("current_version", version).Bool("dirty", dirty).Msg("current migration status")

		err = m.Up()
		if err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrations: up: %w", err)
		} else if err == migrate.ErrNoChange {
			log.Info().Msg("no migration required. database is already on last schema revision")
			return nil
		}

		newversion, dirty, err := m.Version()
		if err != nil {
			return err
		}
		if dirty {
			return errors.New("database is dirty; manual fix is required")
		}

		log.Info().Uint("new_version", newversion).Msg("successfully migrated")
		return nil
	},
}

func init() {
	MigrateUp.Flags().AddFlagSet(cfg.Flags("postgres"))
}
