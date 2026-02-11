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
