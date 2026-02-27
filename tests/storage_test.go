package tests

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rustwizard/balda/internal/storage"
	"github.com/rustwizard/balda/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestConnect(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("balda"),
		postgres.WithUsername("balda"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pgContainer.Terminate(ctx))
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	err = pool.Ping(ctx)
	assert.NoError(t, err)

	st := storage.New(pool, 10*time.Second)
	assert.NotNil(t, st.DB())
}

func TestMigrations(t *testing.T) {
	ctx := context.Background()

	s, cleanup := initStorage(ctx, t)
	defer cleanup()

	assert.NotNil(t, s.DB())

	tables := []string{"users", "user_state"}
	for _, table := range tables {
		var exists bool
		err := s.DB().QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
			table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "expected table %q to exist after migration", table)
	}
}

func startPG(ctx context.Context, t *testing.T) (pc *postgres.PostgresContainer, cleanup func()) {
	t.Helper()
	pc, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("balda"),
		postgres.WithUsername("balda"),
		postgres.WithPassword("balda"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	cleanup = func() {
		if err := pc.Terminate(ctx); err != nil {
			slog.Error("failed to terminate container", "error", err)
		}
	}

	return pc, cleanup
}

func initStorage(ctx context.Context, t *testing.T) (s *storage.Balda, cleanup func()) {
	t.Helper()
	pc, cleanup := startPG(ctx, t)

	dsn, err := pc.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, pool)

	s = storage.New(pool, 10*time.Second)

	err = os.Setenv("MIGRATION_CONN_STRING", dsn)
	require.NoError(t, err)

	// Migrate database
	slog.Info("Balda migration starting")

	dbVersion, err := migrations.Migrate(10 * time.Second)
	require.NoError(t, err)

	slog.Info("Balda migration success", slog.Int("database_version", dbVersion))

	return s, cleanup
}
