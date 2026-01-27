package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestGetTracer(t *testing.T) {
	tracer := GetTracer()
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}
}

func TestSetTracer(t *testing.T) {
	original := GetTracer()
	defer SetTracer(original)

	mockTracer := noop.NewTracerProvider().Tracer("test")
	SetTracer(mockTracer)

	if GetTracer() != mockTracer {
		t.Error("expected tracer to be set")
	}
}

func TestStartJobExecutionSpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartJobExecutionSpan(ctx, "job-1", "Test Job", "exec-1")
	defer span.End()

	if span == nil {
		t.Fatal("expected non-nil span")
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestStartWebhookSpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartWebhookSpan(ctx, "job-1", "POST", "https://example.com/webhook")
	defer span.End()

	if span == nil {
		t.Fatal("expected non-nil span")
	}
}

func TestStartSchedulerSpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartSchedulerSpan(ctx)
	defer span.End()

	if span == nil {
		t.Fatal("expected non-nil span")
	}
}

func TestStartAPISpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartAPISpan(ctx, "createJob")
	defer span.End()

	if span == nil {
		t.Fatal("expected non-nil span")
	}
}

func TestHeaderCarrier(t *testing.T) {
	carrier := HeaderCarrier{}

	carrier.Set("key1", "value1")
	carrier.Set("key2", "value2")

	if carrier.Get("key1") != "value1" {
		t.Errorf("expected 'value1', got %q", carrier.Get("key1"))
	}

	keys := carrier.Keys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("expected disabled by default")
	}
	if cfg.ServiceName != "chronos" {
		t.Errorf("expected service name 'chronos', got %q", cfg.ServiceName)
	}
	if cfg.SampleRate != 1.0 {
		t.Errorf("expected sample rate 1.0, got %f", cfg.SampleRate)
	}
}

func TestInjectExtractTraceContext(t *testing.T) {
	ctx := context.Background()
	carrier := HeaderCarrier{}

	// Inject and extract (should not panic even without active span)
	InjectTraceContext(ctx, carrier)
	newCtx := ExtractTraceContext(ctx, carrier)

	if newCtx == nil {
		t.Fatal("expected non-nil context")
	}
}
