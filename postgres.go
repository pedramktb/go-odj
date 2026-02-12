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

func PostgresTestContainer(ctx context.Context, opts ...typx.KV[string, string]) (container testcontainers.Container) {
	_ = os.Setenv("TZ", "UTC")

	postgresContainer, err := postgresC.Run(ctx, "postgres:14.17",
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

func PostgresTestContainerCreateDB(ctx context.Context, container testcontainers.Container, name string, opts ...typx.KV[string, string]) *pgxpool.Pool {
	pool := postgresTestContainerConnection(ctx, container, "postgres", opts...)
	_, err := pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", name))
	if err != nil {
		panic(err)
	}
	pool.Close()
	return postgresTestContainerConnection(ctx, container, name, opts...)
}

func PostgresTestContainerDropDB(ctx context.Context, container testcontainers.Container, name string, pool *pgxpool.Pool, opts ...typx.KV[string, string]) {
	pool.Close()
	pool = postgresTestContainerConnection(ctx, container, "postgres", opts...)
	_, _ = pool.Exec(ctx, fmt.Sprintf("DROP DATABASE %q WITH (force)", name))
	pool.Close()
}

func PostgresTestContainerSetupDB(ctx context.Context, t *testing.T, container testcontainers.Container, opts ...typx.KV[string, string]) *pgxpool.Pool {
	t.Helper()
	name := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	pool := PostgresTestContainerCreateDB(ctx, container, name, opts...)
	t.Cleanup(func() {
		PostgresTestContainerDropDB(ctx, container, name, pool, opts...)
	})
	return pool
}
