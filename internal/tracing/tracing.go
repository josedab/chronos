// Package tracing provides OpenTelemetry distributed tracing for Chronos.
package tracing

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TracerName is the name of the Chronos tracer.
	TracerName = "github.com/chronos/chronos"
)

// Config holds tracing configuration.
type Config struct {
	Enabled     bool   `yaml:"enabled"`
	ServiceName string `yaml:"service_name"`
	Endpoint    string `yaml:"endpoint"`     // OTLP endpoint
	SampleRate  float64 `yaml:"sample_rate"` // 0.0 to 1.0
}

// DefaultConfig returns the default tracing configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:     false,
		ServiceName: "chronos",
		Endpoint:    "http://localhost:4318", // OTLP local development default
		SampleRate:  1.0,
	}
}

// Tracer is the global tracer for Chronos.
var tracer trace.Tracer

func init() {
	tracer = otel.Tracer(TracerName)
}

// GetTracer returns the Chronos tracer.
func GetTracer() trace.Tracer {
	return tracer
}

// SetTracer sets a custom tracer (useful for testing).
func SetTracer(t trace.Tracer) {
	tracer = t
}

// SpanAttributes for common Chronos operations.
var (
	AttrJobID       = attribute.Key("chronos.job.id")
	AttrJobName     = attribute.Key("chronos.job.name")
	AttrExecID      = attribute.Key("chronos.execution.id")
	AttrExecStatus  = attribute.Key("chronos.execution.status")
	AttrScheduleTime = attribute.Key("chronos.schedule.time")
	AttrNodeID      = attribute.Key("chronos.node.id")
	AttrIsLeader    = attribute.Key("chronos.node.is_leader")
	AttrRetryCount  = attribute.Key("chronos.retry.count")
)

// StartJobExecutionSpan starts a span for job execution.
func StartJobExecutionSpan(ctx context.Context, jobID, jobName, execID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "job.execute",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			AttrJobID.String(jobID),
			AttrJobName.String(jobName),
			AttrExecID.String(execID),
		),
	)
}

// StartWebhookSpan starts a span for webhook dispatch.
func StartWebhookSpan(ctx context.Context, jobID, method, url string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "webhook.dispatch",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			AttrJobID.String(jobID),
			attribute.String("http.method", method),
			attribute.String("http.url", url),
		),
	)
}

// StartSchedulerSpan starts a span for scheduler tick.
func StartSchedulerSpan(ctx context.Context) (context.Context, trace.Span) {
	return tracer.Start(ctx, "scheduler.tick",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
}

// StartAPISpan starts a span for API request handling.
func StartAPISpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "api."+operation,
		trace.WithSpanKind(trace.SpanKindServer),
	)
}

// RecordError records an error on the span.
func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanOK marks the span as successful.
func SetSpanOK(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}

// AddJobAttributes adds job-related attributes to a span.
func AddJobAttributes(span trace.Span, jobID, jobName string) {
	span.SetAttributes(
		AttrJobID.String(jobID),
		AttrJobName.String(jobName),
	)
}

// AddExecutionAttributes adds execution-related attributes to a span.
func AddExecutionAttributes(span trace.Span, execID, status string, duration time.Duration) {
	span.SetAttributes(
		AttrExecID.String(execID),
		AttrExecStatus.String(status),
		attribute.Float64("duration_seconds", duration.Seconds()),
	)
}

// Propagator returns the context propagator for distributed tracing.
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// InjectTraceContext injects trace context into a carrier.
func InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	Propagator().Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from a carrier.
func ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return Propagator().Extract(ctx, carrier)
}

// HeaderCarrier is an adapter for HTTP headers.
type HeaderCarrier map[string]string

func (c HeaderCarrier) Get(key string) string {
	return c[key]
}

func (c HeaderCarrier) Set(key, value string) {
	c[key] = value
}

func (c HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
