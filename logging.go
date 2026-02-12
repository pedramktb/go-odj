package odj

import (
	"context"
	"log/slog"
	"os"

	"github.com/pedramktb/go-ctxslog"
	"go.opentelemetry.io/otel/trace"
)

// Logging returns a new context with an attached slog logger. Whenever logging with this logger,
// if the context at that time contains an active OpenTelemetry span, the logger will automatically
// include the trace_id and span_id as attributes in the log records.
func Logging(ctx context.Context) context.Context {
	return ctxslog.WithAttrs(
		ctxslog.NewContext(ctx, slogHandler),
		func(ctx context.Context, _ slog.Record) []slog.Attr {
			spanCtx := trace.SpanFromContext(ctx).SpanContext()
			if spanCtx.IsValid() {
				return []slog.Attr{
					slog.String("trace_id", spanCtx.TraceID().String()),
					slog.String("span_id", spanCtx.SpanID().String()),
				}
			}
			return nil
		},
	)
}

var slogHandler = func() slog.Handler {
	var handler slog.Handler
	switch Stage {
	case StageLocal:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})
	case StageTest, StageDev, StageQA:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})
	case StageProd:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
	}
	return handler
}()
