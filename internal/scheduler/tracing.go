// Package scheduler provides tracing integration for the scheduler.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds configuration for scheduler tracing.
type TracingConfig struct {
	Enabled     bool
	TraceOnTick bool // Trace every scheduler tick (can be verbose)
}

// DefaultTracingConfig returns the default tracing configuration.
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:     true,
		TraceOnTick: false, // Disable by default to reduce noise
	}
}

// TracedScheduler wraps a Scheduler with tracing capabilities.
type TracedScheduler struct {
	*Scheduler
	tracer trace.Tracer
	config TracingConfig
}

// WithTracing enables tracing on the scheduler.
// Returns the original scheduler if config.Enabled is false.
func (s *Scheduler) WithTracing(config TracingConfig) *TracedScheduler {
	return &TracedScheduler{
		Scheduler: s,
		tracer:    tracing.GetTracer(),
		config:    config,
	}
}

// ExecuteJobWithTracing executes a job with tracing spans.
func (s *TracedScheduler) ExecuteJobWithTracing(ctx context.Context, job *models.Job, scheduledTime time.Time) *models.Execution {
	if !s.config.Enabled {
		return s.executeJobInternal(ctx, job, scheduledTime)
	}

	// Start the job execution span
	ctx, span := tracing.StartJobExecutionSpan(ctx, job.ID, job.Name, "")
	defer span.End()

	// Add additional attributes
	span.SetAttributes(
		tracing.AttrScheduleTime.String(scheduledTime.Format(time.RFC3339)),
		attribute.String("chronos.job.schedule", job.Schedule),
	)

	if job.Timezone != "" {
		span.SetAttributes(attribute.String("chronos.job.timezone", job.Timezone))
	}

	execution := s.executeJobInternal(ctx, job, scheduledTime)

	if execution != nil {
		span.SetAttributes(tracing.AttrExecID.String(execution.ID))
		if execution.Status == models.ExecutionFailed {
			span.SetAttributes(attribute.String("error", execution.Error))
			tracing.RecordError(span, fmt.Errorf("%s", execution.Error))
		} else {
			tracing.SetSpanOK(span)
		}
		tracing.AddExecutionAttributes(span, execution.ID, string(execution.Status), execution.Duration)
	}

	return execution
}

// executeJobInternal is the internal job execution (calls the dispatcher).
func (s *TracedScheduler) executeJobInternal(ctx context.Context, job *models.Job, scheduledTime time.Time) *models.Execution {
	execution, err := s.dispatcher.Execute(ctx, job, scheduledTime)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Msg("Job execution failed")
	}
	return execution
}

// tickWithTracing performs a scheduler tick with tracing.
func (s *TracedScheduler) tickWithTracing(ctx context.Context) {
	if !s.config.Enabled || !s.config.TraceOnTick {
		s.tick()
		return
	}

	ctx, span := tracing.StartSchedulerSpan(ctx)
	defer span.End()

	s.jobsMu.RLock()
	jobCount := len(s.jobs)
	enabledCount := 0
	for _, job := range s.jobs {
		if job.Enabled {
			enabledCount++
		}
	}
	s.jobsMu.RUnlock()

	span.SetAttributes(
		attribute.Int("chronos.scheduler.jobs_total", jobCount),
		attribute.Int("chronos.scheduler.jobs_enabled", enabledCount),
		attribute.Int64("chronos.scheduler.queue_size", int64(s.nextRuns.Len())),
	)

	s.tick()

	tracing.SetSpanOK(span)
}

// TriggerJobWithTracing triggers a job execution with tracing.
func (s *TracedScheduler) TriggerJobWithTracing(ctx context.Context, jobID string) (*models.Execution, error) {
	if !s.config.Enabled {
		return s.TriggerJob(jobID)
	}

	ctx, span := s.tracer.Start(ctx, "scheduler.trigger_job",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(tracing.AttrJobID.String(jobID)),
	)
	defer span.End()

	execution, err := s.TriggerJob(jobID)
	if err != nil {
		tracing.RecordError(span, err)
		return nil, err
	}

	if execution != nil {
		span.SetAttributes(
			tracing.AttrExecID.String(execution.ID),
			tracing.AttrJobName.String(execution.JobName),
		)
	}

	tracing.SetSpanOK(span)
	return execution, nil
}

// AddJobWithTracing adds a job with tracing.
func (s *TracedScheduler) AddJobWithTracing(ctx context.Context, job *models.Job) error {
	if !s.config.Enabled {
		return s.AddJob(job)
	}

	_, span := s.tracer.Start(ctx, "scheduler.add_job",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			tracing.AttrJobID.String(job.ID),
			tracing.AttrJobName.String(job.Name),
		),
	)
	defer span.End()

	if err := s.AddJob(job); err != nil {
		tracing.RecordError(span, err)
		return err
	}

	tracing.SetSpanOK(span)
	return nil
}

// RemoveJobWithTracing removes a job with tracing.
func (s *TracedScheduler) RemoveJobWithTracing(ctx context.Context, jobID string) {
	if !s.config.Enabled {
		s.RemoveJob(jobID)
		return
	}

	_, span := s.tracer.Start(ctx, "scheduler.remove_job",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(tracing.AttrJobID.String(jobID)),
	)
	defer span.End()

	s.RemoveJob(jobID)
	tracing.SetSpanOK(span)
}
