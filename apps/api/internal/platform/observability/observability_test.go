package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInitNoopWhenEndpointEmpty(t *testing.T) {
	p, err := Init(context.Background(), Config{ServiceName: "kubedojo-test"})
	if err != nil {
		t.Fatalf("init noop: %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown noop: %v", err)
	}
}

func TestInitRejectsInvalidEndpoint(t *testing.T) {
	if _, err := Init(context.Background(), Config{ServiceName: "kubedojo-test", OTLPEndpoint: "ftp://bad"}); err == nil {
		t.Fatal("expected error for invalid OTLP scheme")
	}
}

func TestHTTPMiddlewareRecordsSpansAndMetrics(t *testing.T) {
	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { _ = mp.Shutdown(context.Background()) }()

	prevTP, prevMP, prevProp := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		otel.SetTextMapPropagator(prevProp)
	}()

	mw := HTTPMiddleware("kubedojo-api", func(*http.Request) string { return "/api/v1/labs/{labId}" })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/labs/1234", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, want 201", rec.Code)
	}
	spans := spanExp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans=%d, want 1", len(spans))
	}
	if spans[0].Name != "POST /api/v1/labs/{labId}" {
		t.Fatalf("span name=%q", spans[0].Name)
	}
	if spans[0].Parent.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("traceparent not propagated: parent=%s", spans[0].Parent.TraceID())
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	if len(rm.ScopeMetrics) == 0 || len(rm.ScopeMetrics[0].Metrics) < 2 {
		t.Fatalf("expected request duration and count metrics, got %+v", rm.ScopeMetrics)
	}
}
