# OpenTelemetry Testing Service

This project demonstrates the implementation of OpenTelemetry (OTEL) for metrics and traces in a Go service. It serves as a practical example of how to instrument a Go application with OpenTelemetry for observability.
w
## Features

- HTTP server with a `/health` endpoint that makes downstream calls
- Prometheus metrics exposition at `/metrics`
- OpenTelemetry trace generation and export to stdout
- HTTP client instrumentation with custom metrics
- Example of both server and client-side telemetry

## Technical Details

The service includes:
- A health check endpoint that makes an outbound HTTP request to `httpbin.org`
- Prometheus metrics integration for monitoring
- Custom counter metric for HTTP client requests

## Endpoints

- `/health` - Health check endpoint that makes a downstream call
- `/metrics` - Prometheus metrics endpoint


## Metric Label Analysis

For the metric `http_client_duration_milliseconds_count` you will find for ex a line like this at `/metrics`:
```
http_client_duration_milliseconds_bucket{http_method="GET",http_status_code="200",net_peer_name="httpbin.org",otel_scope_name="go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",otel_scope_version="0.59.0",service_name="health-service",url_full="https://httpbin.org/status/200",url_path="/status/200",le="10"} 0
```

those are the labels associated with the metric :

```
{
	http_method="GET",
	http_status_code="200",
	net_peer_name="httpbin.org",
	otel_scope_name="go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
	otel_scope_version="0.59.0",
	service_name="health-service",
	url_full="https://httpbin.org/status/200",
	url_path="/status/200",
}
```

### Default Labels (Added by OTel HTTP Instrumentation)
- `http_method="GET"` - The HTTP method used in the request
- `http_status_code="502"` - The HTTP response status code
- `net_peer_name="httpbin.org"` - The target hostname (_Deprecated_ in favor of server.address)
- `otel_scope_name="go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"` - The instrumentation scope name
- `otel_scope_version="0.59.0"` - The version of the OTel instrumentation


## New semantic conventions
Note: The `net_peer_name` attribute is deprecated and will be replaced by `server.address`.
To enable the new `server.address` attribute, set the environment variable (see [this release](https://github.com/open-telemetry/opentelemetry-go-contrib/releases/tag/v1.34.0))
```
OTEL_SEMCONV_STABILITY_OPT_IN=http/compat
```

### Labels added by our `customAttributes` method
- `url_path="/status/200"` - The path component of the URL
- `url_full="https://httpbin.org/status/200"` - The complete URL
