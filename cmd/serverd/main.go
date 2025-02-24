package main

import (
	"context"
	"log"
	"net/http"
	"time"

	myotel "otel_server/internal/otel"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func main() {
	// Initialize OpenTelemetry
	ctx := context.Background()
	serviceResource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("health-service"),
	)
	// Set up Prometheus exporter for metrics
	promExporter, err := prometheus.New(prometheus.WithResourceAsConstantLabels(
		func(kv attribute.KeyValue) bool {
			return kv.Key == semconv.ServiceNameKey
		}))
	if err != nil {
		log.Fatal(err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(promExporter),
		metric.WithResource(serviceResource),
	)
	otel.SetMeterProvider(meterProvider)

	// Ensure we create a meter for HTTP client
	meter := otel.GetMeterProvider().Meter("http-client")
	httpClientCounter, err := meter.Int64Counter("http_client_requests")
	if err != nil {
		log.Println(err)
	}

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
	client := myotel.NewHTTPClient(myotel.WithTimeout(2 * time.Second))
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
