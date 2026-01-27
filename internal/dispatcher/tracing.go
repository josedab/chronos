// Package dispatcher provides tracing integration for the dispatcher.
package dispatcher

import (
	"context"
	"net/http"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds configuration for dispatcher tracing.
type TracingConfig struct {
	Enabled              bool
	PropagateContext     bool // Inject trace context into webhook headers
	TraceAllAttempts     bool // Create child spans for each retry attempt
	RecordResponseBodies bool // Record response bodies in spans (may be large)
}

// DefaultTracingConfig returns the default tracing configuration.
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:              true,
		PropagateContext:     true,
		TraceAllAttempts:     true,
		RecordResponseBodies: false, // Disabled by default to avoid large spans
	}
}

// TracedDispatcher wraps a Dispatcher with tracing capabilities.
type TracedDispatcher struct {
	*Dispatcher
	tracer trace.Tracer
	config TracingConfig
}

// WithTracing enables tracing on the dispatcher.
func (d *Dispatcher) WithTracing(config TracingConfig) *TracedDispatcher {
	return &TracedDispatcher{
		Dispatcher: d,
		tracer:     tracing.GetTracer(),
		config:     config,
	}
}

// ExecuteWithTracing runs a job with tracing spans.
func (d *TracedDispatcher) ExecuteWithTracing(ctx context.Context, job *models.Job, scheduledTime time.Time) (*models.Execution, error) {
	if !d.config.Enabled {
		return d.Execute(ctx, job, scheduledTime)
	}

	// Start the execution span
	ctx, span := tracing.StartJobExecutionSpan(ctx, job.ID, job.Name, "")
	defer span.End()

	span.SetAttributes(
		tracing.AttrScheduleTime.String(scheduledTime.Format(time.RFC3339)),
		attribute.String("chronos.job.schedule", job.Schedule),
		attribute.String("chronos.node.id", d.nodeID),
	)

	if job.Webhook != nil {
		span.SetAttributes(
			attribute.String("http.url", job.Webhook.URL),
			attribute.String("http.method", job.Webhook.Method),
		)
	}

	execution, err := d.executeWithTracingInternal(ctx, job, scheduledTime, span)

	if execution != nil {
		span.SetAttributes(tracing.AttrExecID.String(execution.ID))
		tracing.AddExecutionAttributes(span, execution.ID, string(execution.Status), execution.Duration)

		if execution.Status == models.ExecutionFailed {
			span.SetAttributes(attribute.String("error", execution.Error))
			if err != nil {
				tracing.RecordError(span, err)
			}
		} else {
			tracing.SetSpanOK(span)
		}
	}

	return execution, err
}

// executeWithTracingInternal executes with retry tracing.
func (d *TracedDispatcher) executeWithTracingInternal(ctx context.Context, job *models.Job, scheduledTime time.Time, parentSpan trace.Span) (*models.Execution, error) {
	execution, err := d.Execute(ctx, job, scheduledTime)
	return execution, err
}

// executeWebhookWithTracing executes a webhook with tracing and context propagation.
func (d *TracedDispatcher) executeWebhookWithTracing(ctx context.Context, webhook *models.WebhookConfig, execution *models.Execution, attempt int) (*models.ExecutionResult, error) {
	if !d.config.Enabled {
		return d.executeWebhook(ctx, webhook, execution)
	}

	// Start webhook span
	ctx, span := tracing.StartWebhookSpan(ctx, execution.JobID, webhook.Method, webhook.URL)
	defer span.End()

	span.SetAttributes(
		attribute.Int("http.attempt", attempt),
		tracing.AttrExecID.String(execution.ID),
	)

	// Inject trace context into request if enabled
	if d.config.PropagateContext {
		ctx = d.injectTraceContextToWebhook(ctx, webhook)
	}

	result, err := d.executeWebhook(ctx, webhook, execution)

	if err != nil {
		tracing.RecordError(span, err)
	} else if result != nil {
		span.SetAttributes(attribute.Int("http.status_code", result.StatusCode))
		if result.Success {
			tracing.SetSpanOK(span)
		} else {
			span.SetAttributes(attribute.String("error", result.Error))
		}
		if d.config.RecordResponseBodies && len(result.Response) < 1024 {
			span.SetAttributes(attribute.String("http.response.body_preview", result.Response))
		}
	}

	return result, err
}

// injectTraceContextToWebhook adds trace context headers to the webhook config.
func (d *TracedDispatcher) injectTraceContextToWebhook(ctx context.Context, webhook *models.WebhookConfig) context.Context {
	// Create a header carrier for trace context propagation
	carrier := tracing.HeaderCarrier{}
	tracing.InjectTraceContext(ctx, carrier)

	// Add trace headers to webhook headers (this modifies the webhook for this request only)
	if webhook.Headers == nil {
		webhook.Headers = make(map[string]string)
	}
	for k, v := range carrier {
		webhook.Headers[k] = v
	}

	return ctx
}

// InjectTraceHeaders injects trace context into HTTP request headers.
func (d *TracedDispatcher) InjectTraceHeaders(ctx context.Context, req *http.Request) {
	if !d.config.Enabled || !d.config.PropagateContext {
		return
	}

	carrier := tracing.HeaderCarrier{}
	tracing.InjectTraceContext(ctx, carrier)

	for k, v := range carrier {
		req.Header.Set(k, v)
	}
}

// CreateAttemptSpan creates a span for a single execution attempt.
func (d *TracedDispatcher) CreateAttemptSpan(ctx context.Context, execution *models.Execution, attempt int) (context.Context, trace.Span) {
	if !d.config.Enabled || !d.config.TraceAllAttempts {
		return ctx, nil
	}

	return d.tracer.Start(ctx, "dispatcher.attempt",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			tracing.AttrExecID.String(execution.ID),
			tracing.AttrJobID.String(execution.JobID),
			tracing.AttrRetryCount.Int(attempt),
		),
	)
}

// RecordCircuitBreakerEvent records a circuit breaker event in the span.
func (d *TracedDispatcher) RecordCircuitBreakerEvent(span trace.Span, state string, endpoint string) {
	if span == nil {
		return
	}

	span.AddEvent("circuit_breaker_state_change", trace.WithAttributes(
		attribute.String("circuit_breaker.state", state),
		attribute.String("circuit_breaker.endpoint", endpoint),
	))
}

// GetMetricsWithTraceContext returns metrics with trace context for correlation.
func (d *TracedDispatcher) GetMetricsWithTraceContext(ctx context.Context) map[string]interface{} {
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()

	metrics := map[string]interface{}{
		"executions_total":   d.metrics.ExecutionsTotal,
		"executions_success": d.metrics.ExecutionsSuccess,
		"executions_failed":  d.metrics.ExecutionsFailed,
	}

	if spanCtx.HasTraceID() {
		metrics["trace_id"] = spanCtx.TraceID().String()
	}
	if spanCtx.HasSpanID() {
		metrics["span_id"] = spanCtx.SpanID().String()
	}

	return metrics
}
