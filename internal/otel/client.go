package otel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

type HTTPOption func(*http.Client)

func WithTimeout(timeout time.Duration) HTTPOption {
	return func(c *http.Client) {
		c.Timeout = timeout
	}
}

// NewHTTPClient creates a new HTTP client with OpenTelemetry instrumentation.
func NewHTTPClient(opts ...HTTPOption) *http.Client {
	client := &http.Client{
		Transport: otelhttp.NewTransport(
			cleanhttp.DefaultPooledTransport(),
			otelhttp.WithSpanNameFormatter(func(op string, r *http.Request) string {
				return fmt.Sprintf("I_AM_HTTP/%s/%s", r.Method, op)
			}),
			otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
				return otelhttptrace.NewClientTrace(ctx)
			}),
			// otelhttp.WithSpanOptions(trace.WithSpanKind(trace.SpanKindClient)),
			otelhttp.WithMetricAttributesFn(customAttributes),
		),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// Adds custom attributes to metrics.
func customAttributes(r *http.Request) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.URLPathKey.String(r.URL.Path),
		semconv.URLFullKey.String(r.URL.String()),
		semconv.ServerAddress(r.URL.Host),
	}
}
