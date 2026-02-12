package odj

import "net/http"

// OpenAPISpecHandler returns an HTTP handler function that serves the provided OpenAPI specification as a response to incoming requests.
// The handler sets the response status to 200 OK and the Content-Type header to "text/html; charset=utf-8"
// before writing the OpenAPI specification bytes to the response body.
func OpenAPISpecHandler(spec []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(spec)
	}
}
