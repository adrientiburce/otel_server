package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	otelMetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
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
	}
}

func main() {
	// Initialize OpenTelemetry
	ctx := context.Background()

	// Set up Prometheus exporter for metrics
	promExporter, err := prometheus.New(prometheus.WithResourceAsConstantLabels(
		func(kv attribute.KeyValue) bool {
			return kv.Key == semconv.ServiceNameKey
		}))
	if err != nil {
		log.Fatal(err)
	}

	meterProvider := metric.NewMeterProvider(metric.WithReader(promExporter))
	otel.SetMeterProvider(meterProvider)

	// Ensure we create a meter for HTTP client
	meter := otel.GetMeterProvider().Meter("http-client")
	httpClientCounter, _ := meter.Int64Counter(
		"http_client_requests",
		// otel.WithDescription("Number of HTTP client requests"),
	)

	// Set up stdout exporter for traces
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("health-service"),
		)),
	)
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()
	otel.SetTracerProvider(tp)

	// Create a new router
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthHandler(w, r, httpClientCounter)
	}).Methods("GET")
	// Add Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Wrap the router with OpenTelemetry instrumentation
	wrappedRouter := otelhttp.NewHandler(r, "health-server")

	// Start the HTTP server
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", wrappedRouter))
}

func healthHandler(w http.ResponseWriter, r *http.Request, counter otelMetric.Int64Counter) {
	// Create an instrumented HTTP client
	client := NewHTTPClient(WithTimeout(2 * time.Second))

	// Increment the counter for outgoing requests
	counter.Add(r.Context(), 1)

	// Make an outgoing request to demonstrate client-side instrumentation
	resp, err := client.Get("https://httpbin.org/status/200")
	if err != nil {
		http.Error(w, "Failed to make outgoing request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		http.Error(w, "Outgoing request failed", resp.StatusCode)
	}
}
