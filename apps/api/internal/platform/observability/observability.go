// Package observability configures OpenTelemetry tracing and metrics for KubeDojo services.
// When OTLPEndpoint is empty, it installs no-op providers so local development and tests make
// zero outbound telemetry calls.
package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config controls OpenTelemetry initialization.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	// OTLPEndpoint is the HTTP OTLP collector URL (e.g. http://otel-collector:4318).
	// Empty disables OTLP export cleanly.
	OTLPEndpoint string
}

// Provider holds the configured trace and metric providers for graceful shutdown.
type Provider struct {
	shutdown func(context.Context) error
}

// Shutdown flushes pending spans and metrics. Safe to call on a nil Provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

// Init configures global OpenTelemetry propagation, tracing and metrics.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if cfg.OTLPEndpoint == "" {
		return &Provider{}, nil
	}
	u, err := url.Parse(cfg.OTLPEndpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment.name", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(u.Host)}
	metricOpts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(u.Host)}
	if u.Scheme == "http" {
		traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
		metricOpts = append(metricOpts, otlpmetrichttp.WithInsecure())
	}

	traceExp, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("otel trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)

	metricExp, err := otlpmetrichttp.New(ctx, metricOpts...)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("otel metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	return &Provider{
		shutdown: func(ctx context.Context) error {
			return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
		},
	}, nil
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// HTTPMiddleware instruments HTTP handlers with low-cardinality OpenTelemetry spans and metrics.
// It never records query strings, headers, cookies, or request/response bodies.
func HTTPMiddleware(serviceName string, routePattern func(*http.Request) string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	meter := otel.Meter(serviceName)
	duration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of HTTP server requests."),
	)
	count, _ := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total HTTP server requests."),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			ctx, span := tracer.Start(ctx, r.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(semconv.HTTPRequestMethodKey.String(r.Method)),
			)
			defer span.End()

			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r.WithContext(ctx))

			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			route := "unmatched"
			if routePattern != nil {
				if p := routePattern(r); p != "" {
					route = p
				}
			}
			span.SetName(r.Method + " " + route)
			attrs := []attribute.KeyValue{
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.HTTPRoute(route),
				semconv.HTTPResponseStatusCode(status),
			}
			span.SetAttributes(attrs...)
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
			if duration != nil {
				duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
			}
			if count != nil {
				count.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
		})
	}
}
