package odj

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pedramktb/go-ctxslog"
	"github.com/pedramktb/go-lifecycle"
	"go.uber.org/automaxprocs/maxprocs"
)

// Bootstrap initializes the application context with logging, and lifecycle management.
// It sets the timezone to UTC and configures the maximum number of CPU cores to use based on the container's limits.
//
// The function returns a context that should be used throughout the application, a cancel function to trigger shutdown,
// and a channel that will receive any errors that occur during shutdown.
func Bootstrap() (context.Context, context.CancelFunc, <-chan error) {
	_ = os.Setenv("TZ", "UTC")

	ctx, cancel, shutdownErrs := lifecycle.Context(time.Minute)

	ctx = Logging(ctx)

	if _, err := maxprocs.Set(maxprocs.Logger(func(s string, i ...any) {
		ctxslog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf(s, i...))
	})); err != nil {
		ctxslog.FromContext(ctx).ErrorContext(ctx, "failed to set maxprocs", slog.Any("err", err))
	}

	return ctx, cancel, shutdownErrs
}
