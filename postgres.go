package odj

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pedramktb/go-ctxotel"
	"github.com/pedramktb/go-ctxslog"
	"github.com/pedramktb/go-typx"
	postgresC "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Postgres establishes a connection pool to a PostgreSQL database using the provided connection parameters and options.
func Postgres(ctx context.Context, endpoint, db, user, pass string, opts ...typx.KV[string, string]) (*pgxpool.Pool, error) {
	if endpoint == "" {
		return nil, errors.New("database endpoint is required")
	}
	if db == "" {
		return nil, errors.New("database name is required")
	}
	if user == "" {
		return nil, errors.New("database user is required")
	}
	if pass == "" {
		return nil, errors.New("databbase password is required")
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   endpoint,
		Path:   db,
	}

	q := u.Query()
	for _, kv := range opts {
		q.Set(kv.Key, kv.Val)
	}
	u.RawQuery = q.Encode()

	config, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = &queryTracer{}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}

// RunWithPgLock returns a function that executes the provided function within a PostgreSQL advisory lock.
// The lock is identified by a hash of the given name,ensuring that only one instance of the function can run concurrently
// across different processes or threads that use the same lock name. If the lock cannot be acquired,
// the function will log a message and return without executing the provided function.
func RunWithPgLock(ctx context.Context, db *pgxpool.Pool, name string, fn func(ctx context.Context)) func() error {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	lockID := int64(h.Sum64())
	return func() error {
		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin tx for function lock %s: %w", name, err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var acquired bool
		if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", lockID).Scan(&acquired); err != nil {
			return fmt.Errorf("failed to acquire advisory lock for function %s: %w", name, err)
		}

		if !acquired {
			ctxslog.FromContext(ctx).InfoContext(ctx, "skipping function, lock not acquired", slog.String("function", name))
			return nil
		}

		fn(ctx)

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit tx for function lock %s: %w", name, err)
		}

		return nil
	}
}

type queryTracer struct{}

func (*queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	var fnName string
	for skip := 2; ; skip++ {
		if pc, _, _, ok := runtime.Caller(skip); ok {
			full := runtime.FuncForPC(pc).Name()
			if !strings.Contains(full, "github.com/jackc/pgx") {
				fnName = path.Base(full)
				break
			}
		} else {
			fnName = "Unknown"
			break
		}
	}
	ctx, _ = ctxotel.TracerProviderFromCtx(ctx).Tracer("postgresql").Start(ctx, fnName,
		trace.WithAttributes(
			attribute.String("query.statement", data.SQL),
			attribute.Int("query.arg_count", len(data.Args)),
		),
	)
	return ctx
}

func (*queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("query.command_tag", data.CommandTag.String()),
		attribute.Int64("query.rows_affected", data.CommandTag.RowsAffected()),
	)
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, data.Err.Error())
	} else {
		span.SetStatus(codes.Ok, "OK")
	}
	span.End()
}

// PostgresTestContainer starts a new Postgres test container with the specified options and returns the container instance.
// It sets the timezone to UTC and waits for the database system to be ready before returning.
func PostgresTestContainer(ctx context.Context, opts ...typx.KV[string, string]) (container testcontainers.Container) {
	_ = os.Setenv("TZ", "UTC")

	postgresContainer, err := postgresC.Run(ctx, "postgres:18.1",
		postgresC.WithUsername("test"),
		postgresC.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(time.Minute)),
	)
	if err != nil {
		panic(err)
	}

	return postgresContainer
}

func postgresTestContainerConnection(ctx context.Context, container testcontainers.Container, dbName string, opts ...typx.KV[string, string]) (db *pgxpool.Pool) {
	addr, err := container.Host(ctx)
	if err != nil {
		panic(err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		panic(err)
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword("test", "test"),
		Host:   addr + ":" + port.Port(),
		Path:   dbName,
	}
	q := u.Query()
	for _, kv := range opts {
		q.Set(kv.Key, kv.Val)
	}
	u.RawQuery = q.Encode()
	config, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		panic(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		panic(err)
	}
	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}
	return pool
}

// PostgresTestContainerCreateDB creates a new database with the specified name in the given Postgres test container,
// using the provided connection options. It first establishes a connection to the "postgres" database,
// executes the CREATE DATABASE command, and then returns a new connection pool to the newly created database.
func PostgresTestContainerCreateDB(ctx context.Context, container testcontainers.Container, name string, opts ...typx.KV[string, string]) *pgxpool.Pool {
	pool := postgresTestContainerConnection(ctx, container, "postgres", opts...)
	_, err := pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", name))
	if err != nil {
		panic(err)
	}
	pool.Close()
	return postgresTestContainerConnection(ctx, container, name, opts...)
}

// PostgresTestContainerDropDB drops the specified database from the given Postgres test container,
// using the provided connection pool to execute the drop command. It first closes the existing pool,
// creates a new connection to the "postgres" database, executes the drop command with force option, and then closes the pool again.
func PostgresTestContainerDropDB(ctx context.Context, container testcontainers.Container, name string, pool *pgxpool.Pool, opts ...typx.KV[string, string]) {
	pool.Close()
	pool = postgresTestContainerConnection(ctx, container, "postgres", opts...)
	_, _ = pool.Exec(ctx, fmt.Sprintf("DROP DATABASE %q WITH (force)", name))
	pool.Close()
}

// PostgresTestContainerSetupDB creates a new database in the given Postgres test container with a name derived from the test name,
// and returns a connection pool to that database. It also registers a cleanup function to drop the database after the test completes.
func PostgresTestContainerSetupDB(ctx context.Context, t *testing.T, container testcontainers.Container, opts ...typx.KV[string, string]) *pgxpool.Pool {
	t.Helper()
	name := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	pool := PostgresTestContainerCreateDB(ctx, container, name, opts...)
	t.Cleanup(func() {
		PostgresTestContainerDropDB(ctx, container, name, pool, opts...)
	})
	return pool
}
