package odj

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

type otelProxy struct {
	*http.ServeMux
	traceClient coltracepb.TraceServiceClient
	attributes  []*commonpb.KeyValue
}

// NewOtelTraceProxy creates a new OpenTelemetry proxy handler that forwards OTLP/HTTP protobuf requests
// to a configured OTel gRPC collector. This is because ODJ/StackIT did not feel like implementing/allowing OTLP/HTTP.
func NewOtelTraceProxy(srcComponent, endpoint, user, pass string) (http.Handler, error) {
	if endpoint == "" {
		return nil, errors.New("otel trace endpoint is required")
	}
	if user == "" {
		return nil, errors.New("otel trace user is required")
	}
	if pass == "" {
		return nil, errors.New("otel trace password is required")
	}

	var opts []grpc.DialOption
	if Stage == StageLocal {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}
	opts = append(opts, grpc.WithPerRPCCredentials(
		&otelAuth{"Basic " + base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s:%s", user, pass))},
	))

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC collector: %w", err)
	}

	client := coltracepb.NewTraceServiceClient(conn)
	attributes := []attribute.KeyValue{
		attribute.String("otel_proxy.service.name", Component),
		attribute.String("otel_proxy.service.version", FullVersion),
		attribute.String("otel_proxy.deployment.environment", Stage.String()),
		semconv.ServiceNameKey.String(srcComponent),
		semconv.DeploymentEnvironmentNameKey.String(Stage.String()),
	}
	p := &otelProxy{
		traceClient: client,
		attributes:  make([]*commonpb.KeyValue, 0, len(attributes)),
	}
	for _, attr := range attributes {
		kv := &commonpb.KeyValue{
			Key: string(attr.Key),
			Value: &commonpb.AnyValue{
				Value: &commonpb.AnyValue_StringValue{
					StringValue: attr.Value.AsString(),
				},
			},
		}
		p.attributes = append(p.attributes, kv)
	}
	p.ServeMux = http.NewServeMux()
	p.HandleFunc("/v1/traces", p.traces)
	// You can implement /v1/metrics, /v1/logs, etc. if needed (though even the gRPC collector does not support them yet)
	return p, nil
}

func (p *otelProxy) traces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	if err := r.Body.Close(); err != nil {
		log.Printf("Error closing request body: %v", err)
	}

	var req coltracepb.ExportTraceServiceRequest
	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		var genericPayload map[string]any
		if err := json.Unmarshal(body, &genericPayload); err != nil {
			log.Printf("Error unmarshaling JSON into generic map: %v", err)
			http.Error(w, "Bad request body", http.StatusBadRequest)
			return
		}

		// Recursively find and convert hex IDs to Base64.
		transformHexIdsToBase64(genericPayload)

		// Marshal the corrected structure back to JSON bytes.
		correctedBody, err := json.Marshal(genericPayload)
		if err != nil {
			log.Printf("Error re-marshaling corrected JSON: %v", err)
			http.Error(w, "Internal server error during conversion", http.StatusInternalServerError)
			return
		}

		if err := protojson.Unmarshal(correctedBody, &req); err != nil {
			log.Printf("Error unmarshaling JSON: %v", err)
			http.Error(w, "Bad request body", http.StatusBadRequest)
			return
		}
	case strings.HasPrefix(contentType, "application/x-protobuf"):
		if err := proto.Unmarshal(body, &req); err != nil {
			log.Printf("Error unmarshaling protobuf: %v", err)
			http.Error(w, "Bad request body", http.StatusBadRequest)
			return
		}
	default:
		log.Printf("Unsupported Content-Type: %s", contentType)
		http.Error(w, "Unsupported Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	// Enforce resource attributes
	p.overrideResourceAttributes(&req)

	log.Printf("Forwarding %d resource spans to gRPC collector (from %s)", len(req.GetResourceSpans()), contentType)
	resp, err := p.traceClient.Export(r.Context(), &req)
	if err != nil {
		log.Printf("Error exporting traces to gRPC collector: %v", err)
		// Return a generic server error to the client. The specific error is logged.
		http.Error(w, "Failed to forward traces", http.StatusInternalServerError)
		return
	}

	// The gRPC collector returns an ExportTraceServiceResponse.
	// We must marshal this response back into the original content type.
	var respBody []byte
	var respContentType string

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		respBody, err = protojson.Marshal(resp)
		respContentType = "application/json"
	case strings.HasPrefix(contentType, "application/x-protobuf"):
		respBody, err = proto.Marshal(resp)
		respContentType = "application/x-protobuf"
	}

	if err != nil {
		log.Printf("Error marshaling gRPC response: %v", err)
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	// Set the correct content type and write the response.
	w.Header().Set("Content-Type", respContentType)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(respBody); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

type otelAuth struct {
	token string
}

func (a *otelAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": a.token,
	}, nil
}

func (a *otelAuth) RequireTransportSecurity() bool {
	return false
}

func transformHexIdsToBase64(data any) {
	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			// Check for traceId (32 hex chars = 16 bytes) and spanId (16 hex chars = 8 bytes)
			if key == "traceId" || key == "spanId" || key == "parentSpanId" {
				if idStr, ok := val.(string); ok {
					// Attempt to decode from hex
					decoded, err := hex.DecodeString(idStr)
					if err == nil {
						// If successful, re-encode as Base64 and replace the value
						v[key] = base64.StdEncoding.EncodeToString(decoded)
					}
				}
			} else {
				// Recurse for other keys
				transformHexIdsToBase64(val)
			}
		}
	case []any:
		// If it's a slice, iterate and recurse
		for _, item := range v {
			transformHexIdsToBase64(item)
		}
	}
}

func (p *otelProxy) overrideResourceAttributes(req *coltracepb.ExportTraceServiceRequest) {
	for _, rs := range req.ResourceSpans {
		if rs.Resource == nil {
			continue
		}
		rs.Resource.Attributes = upsertAttribute(rs.Resource.Attributes, p.attributes...)
	}
}

func upsertAttribute(attrs []*commonpb.KeyValue, upsert ...*commonpb.KeyValue) []*commonpb.KeyValue {
	for _, kv := range attrs {
		for _, newKv := range upsert {
			if kv.Key == newKv.Key {
				kv.Value = newKv.Value
			}
		}
	}
	for _, newKv := range upsert {
		found := false
		for _, kv := range attrs {
			if kv.Key == newKv.Key {
				found = true
				break
			}
		}
		if !found {
			attrs = append(attrs, newKv)
		}
	}
	return attrs
}
