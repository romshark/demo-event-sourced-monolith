// Package testdb uses test containers to provide a database for testing purposes.
package testdb

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/romshark/demo-event-sourced-monolith/database"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// New starts a PostgreSQL container, applies migrations,
// and returns a database connection. It automatically adds cleanup funcs.
func New(
	tb testing.TB, ctx context.Context, log *slog.Logger,
) (testDatabase database.Database) {
	req := testcontainers.ContainerRequest{
		Image: "postgres:17",
		Env: map[string]string{
			"POSTGRES_USER":     "demo",
			"POSTGRES_PASSWORD": "demo",
			"POSTGRES_DB":       "demo",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.
			ForListeningPort(nat.Port("5432/tcp")).
			WithStartupTimeout(60 * time.Second),
	}

	cont, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	if err != nil {
		tb.Fatalf("starting postgres container: %v", err)
	}

	host, err := cont.Host(ctx)
	if err != nil {
		tb.Fatalf("container host: %v", err)
	}
	mapped, err := cont.MappedPort(ctx, nat.Port("5432/tcp"))
	if err != nil {
		tb.Fatalf("container port: %v", err)
	}

	dsn := fmt.Sprintf(
		"postgres://demo:demo@%s:%s/demo?sslmode=disable",
		host, mapped.Port(),
	)

	testDatabase, err = database.Open(ctx, log, dsn, 0)
	if err != nil {
		tb.Fatalf("connecting to test db: %v", err)
	}

	tb.Cleanup(testDatabase.Close)
	tb.Cleanup(func() {
		if err := cont.Terminate(context.Background()); err != nil {
			tb.Errorf("terminating container: %v", err)
		}
	})

	return testDatabase
}
