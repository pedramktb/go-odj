package odj

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"github.com/pedramktb/go-ctxotel"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// OtelTrace initializes an OpenTelemetry tracer provider with the given SpanExporter.
func OtelTrace(ctx context.Context, exporter sdktrace.SpanExporter) (context.Context, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

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
		return ctx, err
	}

	return ctxotel.NewTracerProviderCtx(ctx,
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resources),
	), nil
}

// OtelTraceGRPCBasicAuthExporter creates an OTLP gRPC SpanExporter using basic authentication.
func OtelTraceGRPCBasicAuthExporter(ctx context.Context, endpoint, user, pass string) (sdktrace.SpanExporter, error) {
	if endpoint == "" {
		return nil, errors.New("otel trace endpoint is required")
	}
	if user == "" {
		return nil, errors.New("otel trace user is required")
	}
	if pass == "" {
		return nil, errors.New("otel trace password is required")
	}

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

	return otlptracegrpc.New(ctx, opts...)
}

// OtelTraceGCPExporter creates a Google Cloud Trace SpanExporter using Application Default Credentials.
// projectID may be empty to auto-detect from the GCP metadata server.
func OtelTraceGCPExporter(projectID string) (sdktrace.SpanExporter, error) {
	opts := []cloudtrace.Option{}
	if projectID != "" {
		opts = append(opts, cloudtrace.WithProjectID(projectID))
	}
	return cloudtrace.New(opts...)
}

// OtelTraceMiddleware is an HTTP middleware that extracts OpenTelemetry trace context from incoming requests and injects it into the request context.
func OtelTraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
