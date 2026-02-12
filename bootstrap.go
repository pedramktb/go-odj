package odj

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/pedramktb/go-ctxslog"
	"github.com/pedramktb/go-lifecycle"
	"go.uber.org/automaxprocs/maxprocs"
)

// Bootstrap initializes the application context with logging, environment variables, and lifecycle management.
// It sets the timezone to UTC, loads environment variables from "secrets.local.env" and "local.env",
// and configures the maximum number of CPU cores to use based on the container's limits.
//
// The function returns a context that should be used throughout the application, a cancel function to trigger shutdown,
// and a channel that will receive any errors that occur during shutdown.
//
// Note: The "secrets.local.env" file should be used for sensitive information or env overrides and should not be committed to version control,
// while "local.env" can be used for non-sensitive configuration.
// This way you can still use a secrets.local.env.dist file with placeholder values for reference,
// while keeping the non-sensitive defaults in local.env.
func Bootstrap() (context.Context, context.CancelFunc, <-chan error) {
	_ = os.Setenv("TZ", "UTC")

	ctx, cancel, shutdownErrs := lifecycle.Context(time.Minute)

	_ = godotenv.Load("secrets.local.env")
	_ = godotenv.Load("local.env")

	ctx = Logging(ctx)

	if _, err := maxprocs.Set(maxprocs.Logger(func(s string, i ...any) {
		ctxslog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(s, i...))
	})); err != nil {
		ctxslog.FromContext(ctx).ErrorContext(ctx, "failed to set maxprocs", slog.Any("err", err))
	}

	return ctx, cancel, shutdownErrs
}
