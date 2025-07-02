package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	log  *slog.Logger
	pool *pgxpool.Pool
}

const OpenTimeout = 60 * time.Second

// Open connects to the database using a connection pool, applies migrations,
// and returns a Database. It will retry until OpenTimeout.
func Open(
	ctx context.Context, log *slog.Logger, dsn string, maxConns int32,
) (Database, error) {
	if maxConns < 1 {
		maxConns = int32(runtime.NumCPU())
	}
	outer, outerCancel := context.WithTimeout(ctx, OpenTimeout)
	defer outerCancel()

	var pool *pgxpool.Pool
	for {
		if err := outer.Err(); err != nil {
			return Database{}, fmt.Errorf("connecting database timed out: %w", err)
		}

		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			return Database{}, fmt.Errorf("invalid DSN: %w", err)
		}

		cfg.MaxConns = int32(maxConns)

		p, err := pgxpool.NewWithConfig(outer, cfg)
		if err != nil {
			return Database{}, fmt.Errorf("creating pgx pool with config: %w", err)
		}

		tmpCtx, cancel := context.WithTimeout(outer, 1*time.Second)
		err = p.Ping(tmpCtx)
		cancel()
		if err != nil {
			p.Close()
			continue
		}

		pool = p
		break
	}

	if err := applyMigrations(dsn); err != nil {
		pool.Close()
		return Database{}, fmt.Errorf("migrating schema: %w", err)
	}

	return Database{log: log, pool: pool}, nil
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

func applyMigrations(dsn string) error {
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

type Tx struct {
	lock sync.Mutex
	tx   pgx.Tx
}

func (t *Tx) Exec(
	ctx context.Context, sql string, args ...any,
) (pgconn.CommandTag, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.tx.Exec(ctx, sql, args...)
}

func (t *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.tx.Query(ctx, sql, args...)
}

func (t *Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.tx.QueryRow(ctx, sql, args...)
}

// TxRW starts a new read-write transaction and executes fn inside of it.
// If fn returns an error or panic occurs, the transaction is rolled back,
// otherwise it is committed.
func (d Database) TxRW(
	ctx context.Context, fn func(context.Context, *Tx) error,
) error {
	return d.withTx(ctx, pgx.ReadWrite, fn)
}

// TxReadOnly starts a new read-only transaction and executes fn inside.
func (d Database) TxReadOnly(
	ctx context.Context, fn func(context.Context, *Tx) error,
) error {
	return d.withTx(ctx, pgx.ReadOnly, fn)
}

func (d Database) withTx(
	ctx context.Context, mode pgx.TxAccessMode, fn func(context.Context, *Tx) error,
) error {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: mode,
	})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			if rb := tx.Rollback(ctx); rb != nil {
				d.log.Error("rollback after panic failure",
					slog.Any("panic", p),
					slog.Any("err", rb))
			}
		}
	}()

	if err := fn(ctx, &Tx{tx: tx}); err != nil {
		if rb := tx.Rollback(ctx); rb != nil {
			return fmt.Errorf("rolling back transaction: %v (original: %w)", rb, err)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (d Database) Exec(
	ctx context.Context, sql string, args ...any,
) (pgconn.CommandTag, error) {
	return d.pool.Exec(ctx, sql, args...)
}

func (d Database) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return d.pool.Query(ctx, sql, args...)
}

func (d Database) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return d.pool.QueryRow(ctx, sql, args...)
}

func (d Database) Close() {
	d.pool.Close()
}

// func SetSchema(ctx context.Context, tx *Tx, schema string) error {
// 	if schema == "" {
// 		return nil
// 	}
// 	sanitized := pgx.Identifier{schema}.Sanitize()
// 	if _, err := tx.Exec(ctx, `SET LOCAL search_path TO `+sanitized); err != nil {
// 		return fmt.Errorf("setting search_path to %q: %w", schema, err)
// 	}
// 	return nil
// }
