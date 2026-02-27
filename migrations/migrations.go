package migrations

import (
	"context"
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed *.sql
var migrations embed.FS

const (
	versionTable         = "public.schema_version"
	failedMigrateVersion = -1
)

func Migrate(timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dsn := os.Getenv("MIGRATION_CONN_STRING")
	if dsn == "" {
		return failedMigrateVersion, fmt.Errorf("migrations: failed to get MIGRATION_CONN_STRING: not set")
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return failedMigrateVersion, fmt.Errorf("migrations: failed to connect to postgres server: %w", err)
	}
	defer conn.Close(ctx)

	options := &migrate.MigratorOptions{DisableTx: false}
	migrator, err := migrate.NewMigratorEx(ctx, conn, versionTable, options)
	if err != nil {
		return failedMigrateVersion, fmt.Errorf("migrations: failed to init new migrator: %w", err)
	}

	if err := migrator.LoadMigrations(migrations); err != nil {
		return failedMigrateVersion, fmt.Errorf("migrations: failed to load migrations: %w", err)
	}

	if err := migrator.Migrate(ctx); err != nil {
		return failedMigrateVersion, fmt.Errorf("migrations: failed to migrate database: %w", err)
	}

	version, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return failedMigrateVersion, fmt.Errorf("migrations: failed to get migrate version: %w", err)
	}

	return int(version), nil
}
