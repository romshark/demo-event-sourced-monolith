package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/romshark/conductor/db/dbpgx"
	"github.com/stretchr/testify/require"
)

const AdminDSN = "postgres://demo:demo@localhost:5432/postgres?sslmode=disable"

//go:embed migrations/*.sql
var migrationsFS embed.FS

func ApplyMigrations(dsn string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migrate source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("creating migrate instance: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}

func MustOpen(ctx context.Context, log *slog.Logger, dsn string) *dbpgx.DB {
	db, err := dbpgx.Open(ctx, log, dsn, 0, dbpgx.DefaultBackoff())
	if err != nil {
		panic(err)
	}
	return db
}

// NewTestDB creates a new dbpgx-based test database for the given test.
func NewTestDB(t testing.TB, log *slog.Logger) (db *dbpgx.DB) {
	t.Helper()
	ctx := t.Context()

	// Derive a unique test DB name from the test name
	dbName := "test_" + strings.ReplaceAll(t.Name(), "/", "_")

	adminPool, err := pgxpool.New(ctx, AdminDSN)
	require.NoError(t, err)
	defer adminPool.Close()

	dbNameSanitized := pgx.Identifier{dbName}.Sanitize()
	_, err = adminPool.Exec(ctx, `DROP DATABASE IF EXISTS `+dbNameSanitized)
	require.NoError(t, err)
	_, err = adminPool.Exec(ctx, `CREATE DATABASE `+dbNameSanitized)
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://demo:demo@localhost:5432/%s?sslmode=disable", dbName)
	err = ApplyMigrations(dsn)
	require.NoError(t, err)

	db, err = dbpgx.Open(ctx, log, dsn, 0, dbpgx.DefaultBackoff())
	require.NoError(t, err)

	t.Cleanup(db.Close)
	return db
}
