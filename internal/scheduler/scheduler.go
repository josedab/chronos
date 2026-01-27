// Package scheduler provides the job scheduling engine.
package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chronos/chronos/internal/dispatcher"
	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/storage"
	"github.com/chronos/chronos/pkg/cron"
	"github.com/rs/zerolog"
)

// SchedulerStore defines the storage operations needed by the scheduler.
type SchedulerStore interface {
	storage.JobStore
	storage.ExecutionStore
	storage.ScheduleStore
}

// Scheduler manages job scheduling and execution.
type Scheduler struct {
	store      SchedulerStore
	dispatcher *dispatcher.Dispatcher
	logger     zerolog.Logger

	// Job state
	jobs     map[string]*models.Job
	jobsMu   sync.RWMutex

	// Schedule cache - avoids reparsing cron expressions
	scheduleCache   map[string]cron.Schedule
	locationCache   map[string]*time.Location
	scheduleCacheMu sync.RWMutex

	// Scheduling state
	nextRuns *PriorityQueue
	queueMu  sync.Mutex

	// Control
	ticker   *time.Ticker
	stopCh   chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	isLeader bool
	leaderMu sync.RWMutex

	// Configuration
	tickInterval    time.Duration
	missedRunPolicy MissedRunPolicy

	// Metrics
	metrics *Metrics
}

// MissedRunPolicy defines how to handle missed runs.
type MissedRunPolicy string

const (
	MissedRunIgnore     MissedRunPolicy = "ignore"
	MissedRunExecuteOne MissedRunPolicy = "execute_one"
	MissedRunExecuteAll MissedRunPolicy = "execute_all"
)

// Metrics tracks scheduler metrics using atomic counters for thread safety.
type Metrics struct {
	JobsTotal       atomic.Int64
	JobsEnabled     atomic.Int64
	ScheduledRuns   atomic.Int64
	MissedRuns      atomic.Int64
	ExecutionsTotal atomic.Int64
}

// Config holds scheduler configuration.
type Config struct {
	TickInterval    time.Duration
	MissedRunPolicy MissedRunPolicy
}

// DefaultConfig returns the default scheduler configuration.
func DefaultConfig() *Config {
	return &Config{
		TickInterval:    time.Second,
		MissedRunPolicy: MissedRunExecuteOne,
	}
}

// New creates a new Scheduler.
func New(store SchedulerStore, disp *dispatcher.Dispatcher, logger zerolog.Logger, cfg *Config) *Scheduler {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	pq := &PriorityQueue{}
	heap.Init(pq)

	return &Scheduler{
		store:           store,
		dispatcher:      disp,
		logger:          logger.With().Str("component", "scheduler").Logger(),
		jobs:            make(map[string]*models.Job),
		scheduleCache:   make(map[string]cron.Schedule),
		locationCache:   make(map[string]*time.Location),
		nextRuns:        pq,
		stopCh:          make(chan struct{}),
		tickInterval:    cfg.TickInterval,
		missedRunPolicy: cfg.MissedRunPolicy,
		metrics:         &Metrics{},
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info().Msg("Starting scheduler")

	// Create cancellable context for job execution
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Load jobs from store
	if err := s.loadJobs(); err != nil {
		return err
	}

	// Start the ticker
	s.ticker = time.NewTicker(s.tickInterval)

	go s.run(ctx)

	return nil
}

// Stop stops the scheduler and waits for running jobs to complete.
func (s *Scheduler) Stop() {
	s.logger.Info().Msg("Stopping scheduler")
	
	// Cancel context to signal running jobs
	if s.cancel != nil {
		s.cancel()
	}
	
	close(s.stopCh)
	if s.ticker != nil {
		s.ticker.Stop()
	}
	
	// Wait for all running jobs to complete
	s.wg.Wait()
	s.logger.Info().Msg("Scheduler stopped, all jobs completed")
}

// run is the main scheduler loop.
func (s *Scheduler) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			if s.IsLeader() {
				s.tick()
			}
		}
	}
}

// tick checks for jobs that need execution.
func (s *Scheduler) tick() {
	now := time.Now()

	// Collect jobs to execute while holding the lock
	type pendingJob struct {
		job     *models.Job
		nextRun time.Time
	}
	var jobsToExecute []pendingJob
	var jobsToCancel []string

	s.queueMu.Lock()
	for s.nextRuns.Len() > 0 {
		item := (*s.nextRuns)[0]
		if item.NextRun.After(now) {
			break
		}

		// Remove from queue
		heap.Pop(s.nextRuns)

		// Get the job
		s.jobsMu.RLock()
		job, exists := s.jobs[item.JobID]
		s.jobsMu.RUnlock()

		if !exists || !job.Enabled {
			continue
		}

		// Schedule next run (within lock since it modifies queue)
		s.scheduleNextRun(job, now)

		// Collect job for execution
		jobsToExecute = append(jobsToExecute, pendingJob{job: job, nextRun: item.NextRun})
	}
	s.queueMu.Unlock()

	// Process jobs outside the lock to avoid blocking queue operations
	for _, pending := range jobsToExecute {
		job := pending.job

		// Check concurrency policy (dispatcher calls are outside the lock now)
		if job.Concurrency == models.ConcurrencyForbid && s.dispatcher.IsJobRunning(job.ID) {
			s.logger.Debug().Str("job_id", job.ID).Msg("Skipping job due to concurrency policy")
			continue
		}

		if job.Concurrency == models.ConcurrencyReplace && s.dispatcher.IsJobRunning(job.ID) {
			jobsToCancel = append(jobsToCancel, job.ID)
		}

		// Execute job in background with panic recovery
		s.wg.Add(1)
		go func(job *models.Job, nextRun time.Time) {
			defer s.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error().
						Interface("panic", r).
						Str("job_id", job.ID).
						Str("job_name", job.Name).
						Msg("Job execution panicked")
				}
			}()
			s.executeJob(job, nextRun)
		}(job, pending.nextRun)
	}

	// Cancel jobs that need replacement (outside all locks)
	for _, jobID := range jobsToCancel {
		s.dispatcher.CancelJobExecutions(jobID)
	}
}

// executeJob runs a job.
func (s *Scheduler) executeJob(job *models.Job, scheduledTime time.Time) {
	s.logger.Info().
		Str("job_id", job.ID).
		Str("job_name", job.Name).
		Time("scheduled_time", scheduledTime).
		Msg("Executing job")

	// Use scheduler's context as parent to enable cancellation on shutdown
	timeout := job.Timeout.Duration()
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()

	execution, err := s.dispatcher.Execute(ctx, job, scheduledTime)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("job_id", job.ID).
			Msg("Job execution failed")
	}

	// Record execution
	if execution != nil {
		if err := s.store.RecordExecution(execution); err != nil {
			s.logger.Error().
				Err(err).
				Str("job_id", job.ID).
				Str("execution_id", execution.ID).
				Msg("Failed to record execution")
		}

		s.metrics.ExecutionsTotal.Add(1)
	}

	// Update schedule state
	state := &models.ScheduleState{
		JobID:   job.ID,
		LastRun: scheduledTime,
	}
	if err := s.store.SaveScheduleState(state); err != nil {
		s.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to save schedule state")
	}
}

// scheduleNextRun schedules the next run for a job.
func (s *Scheduler) scheduleNextRun(job *models.Job, from time.Time) {
	nextRun, err := s.calculateNextRun(job, from)
	if err != nil {
		s.logger.Error().Err(err).Str("job_id", job.ID).Msg("Failed to calculate next run")
		return
	}

	heap.Push(s.nextRuns, &ScheduledRun{
		JobID:   job.ID,
		NextRun: nextRun,
	})

	s.metrics.ScheduledRuns.Add(1)
}

// calculateNextRun computes the next execution time for a job.
func (s *Scheduler) calculateNextRun(job *models.Job, from time.Time) (time.Time, error) {
	schedule, err := s.getSchedule(job.Schedule)
	if err != nil {
		return time.Time{}, err
	}

	loc, err := s.getLocation(job.Timezone)
	if err != nil {
		return time.Time{}, err
	}

	return schedule.Next(from.In(loc)).UTC(), nil
}

// getSchedule returns a cached schedule or parses and caches a new one.
func (s *Scheduler) getSchedule(expr string) (cron.Schedule, error) {
	s.scheduleCacheMu.RLock()
	schedule, ok := s.scheduleCache[expr]
	s.scheduleCacheMu.RUnlock()
	if ok {
		return schedule, nil
	}

	schedule, err := cron.Parse(expr)
	if err != nil {
		return nil, err
	}

	s.scheduleCacheMu.Lock()
	s.scheduleCache[expr] = schedule
	s.scheduleCacheMu.Unlock()

	return schedule, nil
}

// getLocation returns a cached location or loads and caches a new one.
func (s *Scheduler) getLocation(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}

	s.scheduleCacheMu.RLock()
	loc, ok := s.locationCache[tz]
	s.scheduleCacheMu.RUnlock()
	if ok {
		return loc, nil
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}

	s.scheduleCacheMu.Lock()
	s.locationCache[tz] = loc
	s.scheduleCacheMu.Unlock()

	return loc, nil
}

// InvalidateScheduleCache removes a schedule from the cache (e.g., when job is updated).
func (s *Scheduler) InvalidateScheduleCache(schedule string) {
	s.scheduleCacheMu.Lock()
	delete(s.scheduleCache, schedule)
	s.scheduleCacheMu.Unlock()
}

// loadJobs loads all jobs from the store.
func (s *Scheduler) loadJobs() error {
	jobs, err := s.store.ListJobs()
	if err != nil {
		return err
	}

	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()

	now := time.Now()
	for _, job := range jobs {
		s.jobs[job.ID] = job

		if job.Enabled {
			s.queueMu.Lock()
			nextRun, err := s.calculateNextRun(job, now)
			if err == nil {
				heap.Push(s.nextRuns, &ScheduledRun{
					JobID:   job.ID,
					NextRun: nextRun,
				})
			}
			s.queueMu.Unlock()
		}
	}

	s.metrics.JobsTotal.Store(int64(len(jobs)))

	s.logger.Info().Int("count", len(jobs)).Msg("Loaded jobs")
	return nil
}

// AddJob adds a new job to the scheduler.
func (s *Scheduler) AddJob(job *models.Job) error {
	s.jobsMu.Lock()
	s.jobs[job.ID] = job
	s.jobsMu.Unlock()

	if job.Enabled && s.IsLeader() {
		s.queueMu.Lock()
		nextRun, err := s.calculateNextRun(job, time.Now())
		if err == nil {
			heap.Push(s.nextRuns, &ScheduledRun{
				JobID:   job.ID,
				NextRun: nextRun,
			})
		}
		s.queueMu.Unlock()
	}

	s.metrics.JobsTotal.Add(1)
	if job.Enabled {
		s.metrics.JobsEnabled.Add(1)
	}

	return nil
}

// UpdateJob updates a job in the scheduler.
func (s *Scheduler) UpdateJob(job *models.Job) error {
	s.jobsMu.Lock()
	s.jobs[job.ID] = job
	s.jobsMu.Unlock()

	// Re-schedule if enabled
	if job.Enabled && s.IsLeader() {
		s.queueMu.Lock()
		// Remove existing scheduled runs for this job
		s.removeJobFromQueue(job.ID)
		// Add new scheduled run
		nextRun, err := s.calculateNextRun(job, time.Now())
		if err == nil {
			heap.Push(s.nextRuns, &ScheduledRun{
				JobID:   job.ID,
				NextRun: nextRun,
			})
		}
		s.queueMu.Unlock()
	}

	return nil
}

// RemoveJob removes a job from the scheduler.
func (s *Scheduler) RemoveJob(jobID string) {
	s.jobsMu.Lock()
	delete(s.jobs, jobID)
	s.jobsMu.Unlock()

	s.queueMu.Lock()
	s.removeJobFromQueue(jobID)
	s.queueMu.Unlock()

	s.metrics.JobsTotal.Add(-1)
}

// removeJobFromQueue removes all scheduled runs for a job.
// Must be called with queueMu held.
func (s *Scheduler) removeJobFromQueue(jobID string) {
	// Remove all instances of this job from the queue
	for {
		idx := s.nextRuns.FindByJobID(jobID)
		if idx < 0 {
			break
		}
		s.nextRuns.Remove(idx)
	}
}

// TriggerJob manually triggers a job execution.
func (s *Scheduler) TriggerJob(jobID string) (execution *models.Execution, err error) {
	// Add panic recovery for manual triggers
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().
				Interface("panic", r).
				Str("job_id", jobID).
				Msg("Manual job trigger panicked")
			err = fmt.Errorf("job trigger panicked: %v", r)
		}
	}()

	s.jobsMu.RLock()
	job, exists := s.jobs[jobID]
	s.jobsMu.RUnlock()

	if !exists {
		return nil, models.ErrJobNotFound
	}

	// Use scheduler's context as parent to respect shutdown signals
	parentCtx := s.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	timeout := job.Timeout.Duration()
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	execution, err = s.dispatcher.Execute(ctx, job, time.Now())
	if err != nil {
		return execution, err
	}

	// Record execution
	if execution != nil {
		if err := s.store.RecordExecution(execution); err != nil {
			s.logger.Error().Err(err).Msg("Failed to record manual execution")
		}
	}

	return execution, nil
}

// GetJob returns a job by ID.
func (s *Scheduler) GetJob(jobID string) (*models.Job, bool) {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	job, exists := s.jobs[jobID]
	return job, exists
}

// ListJobs returns all jobs.
func (s *Scheduler) ListJobs() []*models.Job {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// SetLeader sets the leader status.
func (s *Scheduler) SetLeader(isLeader bool) {
	s.leaderMu.Lock()
	wasLeader := s.isLeader
	s.isLeader = isLeader
	s.leaderMu.Unlock()

	if isLeader && !wasLeader {
		s.logger.Info().Msg("Became leader, activating scheduler")
		s.rebuildQueue()
	} else if !isLeader && wasLeader {
		s.logger.Info().Msg("Lost leadership, deactivating scheduler")
	}
}

// IsLeader returns true if this node is the leader.
func (s *Scheduler) IsLeader() bool {
	s.leaderMu.RLock()
	defer s.leaderMu.RUnlock()
	return s.isLeader
}

// rebuildQueue rebuilds the priority queue with all enabled jobs.
// Lock ordering: jobsMu first, then queueMu to prevent deadlocks.
func (s *Scheduler) rebuildQueue() {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()

	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	// Clear the queue
	s.nextRuns = &PriorityQueue{}
	heap.Init(s.nextRuns)

	now := time.Now()
	for _, job := range s.jobs {
		if job.Enabled {
			nextRun, err := s.calculateNextRun(job, now)
			if err == nil {
				heap.Push(s.nextRuns, &ScheduledRun{
					JobID:   job.ID,
					NextRun: nextRun,
				})
			}
		}
	}
}

// MetricsSnapshot is a point-in-time snapshot of scheduler metrics.
type MetricsSnapshot struct {
	JobsTotal       int64
	JobsEnabled     int64
	ScheduledRuns   int64
	MissedRuns      int64
	ExecutionsTotal int64
}

// GetMetrics returns a snapshot of the current metrics.
func (s *Scheduler) GetMetrics() MetricsSnapshot {
	return MetricsSnapshot{
		JobsTotal:       s.metrics.JobsTotal.Load(),
		JobsEnabled:     s.metrics.JobsEnabled.Load(),
		ScheduledRuns:   s.metrics.ScheduledRuns.Load(),
		MissedRuns:      s.metrics.MissedRuns.Load(),
		ExecutionsTotal: s.metrics.ExecutionsTotal.Load(),
	}
}

// GetNextRuns returns the upcoming scheduled runs.
func (s *Scheduler) GetNextRuns(limit int) []ScheduledRun {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()

	runs := make([]ScheduledRun, 0, limit)
	for i := 0; i < s.nextRuns.Len() && i < limit; i++ {
		runs = append(runs, *(*s.nextRuns)[i])
	}
	return runs
}
