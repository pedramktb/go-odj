# go-odj

go-odj is a package that holds a number of utilities that are used frequently in ODJ Go-based applications.

## Utilities

- [Bootstrap](./bootstrap.go): a pre-defined bootstrap function that:
  1. Sets timezone to UTC
  2. Adds a lifecycle into context for closers to register with
  3. Loads environment variables from `local.env`
  4. Sets up a logger into context
  5. Sets up maxprocs
- [Logging](./logging.go): returns a logger in context based on deployment stage
- [OpenAPISpecHandler](./openapi_spec.go): provides a handler for the rendered OpenAPI spec from bytes of a HTML file
- [Info](./info.go): provides a handler that can be used for `/info`, `/readiness` and `/liveness` routes. It contains basic information about the service such as version, build time and git commit. Note that the version, build time and git commit are expected to be set at build time using ldflags.
- [Stage](./stage.go): provides ODJ stages using an enum and env loading.
- [OgenError](./ogen_error.go): provides an error handlers compatible with tagerr Errors.
- [Otel](./otel.go): provides an OTEL trace provider
- [OtelProxy](./otel_proxy.go): provides a handler that can be used to proxy Otel spans to a configured Otel collector.
- [Postgres](./postgres.go): provides Postgres with Tracing and Ready-to-use test containers.
- [SIAM](./siam.go): provides a helper that can read SIAM group membership claim regardless of it being a string or an array.
