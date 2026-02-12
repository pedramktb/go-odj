package odj

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/pedramktb/go-ctxotel"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// OtelTrace initializes an OpenTelemetry tracer provider with a gRPC exporter that sends trace data
// to the specified endpoint using basic authentication.
func OtelTrace(ctx context.Context, endpoint, user, pass string) (context.Context, error) {
	if endpoint == "" {
		return nil, errors.New("otel trace endpoint is required")
	}
	if user == "" {
		return nil, errors.New("otel trace user is required")
	}
	if pass == "" {
		return nil, errors.New("otel trace password is required")
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithHeaders(map[string]string{
			"Authorization": "Basic " + base64.StdEncoding.EncodeToString(
				[]byte(user+":"+pass),
			),
		}),
	}

	if Stage == StageLocal {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	resources, err := resource.New(ctx,
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(Component),
			semconv.ServiceVersionKey.String(FullVersion+"+"+GitSHA),
			semconv.DeploymentEnvironmentNameKey.String(Stage.String()),
		),
	)
	if err != nil {
		return nil, err
	}

	return ctxotel.NewTracerProviderCtx(ctx,
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
	), nil
}

// OtelTraceMiddleware is an HTTP middleware that extracts OpenTelemetry trace context from incoming requests and injects it into the request context.
func OtelTraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
