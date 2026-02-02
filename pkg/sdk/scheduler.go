// Package sdk provides an embeddable scheduling SDK for integrating Chronos
// into Go applications without running a separate server.
package sdk

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// Scheduler provides an embeddable job scheduler.
type Scheduler struct {
	mu        sync.RWMutex
	cron      *cron.Cron
	jobs      map[string]*Job
	running   bool
	hooks     *Hooks
	logger    Logger
	metrics   MetricsCollector
	executor  Executor
	errorHandler ErrorHandler
	timezone  *time.Location
}

// Job represents a scheduled job.
type Job struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Schedule    string            `json:"schedule"`
	Timezone    string            `json:"timezone"`
	Handler     JobHandler        `json:"-"`
	Tags        map[string]string `json:"tags,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	MaxRetries  int               `json:"max_retries,omitempty"`
	RetryDelay  time.Duration     `json:"retry_delay,omitempty"`
	Enabled     bool              `json:"enabled"`
	LastRun     *time.Time        `json:"last_run,omitempty"`
	NextRun     *time.Time        `json:"next_run,omitempty"`
	RunCount    int64             `json:"run_count"`
	ErrorCount  int64             `json:"error_count"`
	entryID     cron.EntryID
	createdAt   time.Time
}

// JobHandler is a function that handles job execution.
type JobHandler func(ctx context.Context, job *Job) error

// Hooks provides callbacks for job lifecycle events.
type Hooks struct {
	BeforeRun  func(job *Job)
	AfterRun   func(job *Job, duration time.Duration, err error)
	OnError    func(job *Job, err error)
	OnSchedule func(job *Job)
	OnRemove   func(job *Job)
}

// Logger interface for custom logging.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// MetricsCollector interface for custom metrics.
type MetricsCollector interface {
	JobScheduled(job *Job)
	JobStarted(job *Job)
	JobCompleted(job *Job, duration time.Duration)
	JobFailed(job *Job, err error, duration time.Duration)
}

// Executor interface for custom job execution.
type Executor interface {
	Execute(ctx context.Context, job *Job) error
}

// ErrorHandler handles job execution errors.
type ErrorHandler func(job *Job, err error)

// Option configures the scheduler.
type Option func(*Scheduler)

// WithTimezone sets the default timezone.
func WithTimezone(tz string) Option {
	return func(s *Scheduler) {
		loc, err := time.LoadLocation(tz)
		if err == nil {
			s.timezone = loc
		}
	}
}

// WithLogger sets a custom logger.
func WithLogger(l Logger) Option {
	return func(s *Scheduler) {
		s.logger = l
	}
}

// WithMetrics sets a custom metrics collector.
func WithMetrics(m MetricsCollector) Option {
	return func(s *Scheduler) {
		s.metrics = m
	}
}

// WithHooks sets lifecycle hooks.
func WithHooks(h *Hooks) Option {
	return func(s *Scheduler) {
		s.hooks = h
	}
}

// WithExecutor sets a custom executor.
func WithExecutor(e Executor) Option {
	return func(s *Scheduler) {
		s.executor = e
	}
}

// WithErrorHandler sets an error handler.
func WithErrorHandler(h ErrorHandler) Option {
	return func(s *Scheduler) {
		s.errorHandler = h
	}
}

// New creates a new embedded scheduler.
func New(opts ...Option) *Scheduler {
	s := &Scheduler{
		jobs:     make(map[string]*Job),
		hooks:    &Hooks{},
		timezone: time.UTC,
		logger:   &defaultLogger{},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.cron = cron.New(
		cron.WithLocation(s.timezone),
		cron.WithSeconds(),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),
		),
	)

	return s
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.cron.Start()
	s.running = true
	s.logger.Info("scheduler started")
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	ctx := s.cron.Stop()
	s.logger.Info("scheduler stopped")
	return ctx
}

// IsRunning returns whether the scheduler is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Schedule adds a job to the scheduler.
func (s *Scheduler) Schedule(name, schedule string, handler JobHandler, opts ...JobOption) (*Job, error) {
	job := &Job{
		ID:        uuid.New().String(),
		Name:      name,
		Schedule:  schedule,
		Timezone:  s.timezone.String(),
		Handler:   handler,
		Tags:      make(map[string]string),
		Enabled:   true,
		createdAt: time.Now(),
	}

	for _, opt := range opts {
		opt(job)
	}

	// Parse the schedule to validate and get next run time
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule: %w", err)
	}

	next := sched.Next(time.Now())
	job.NextRun = &next

	// Create the cron entry
	entryID, err := s.cron.AddFunc(schedule, s.createJobRunner(job))
	if err != nil {
		return nil, fmt.Errorf("failed to schedule job: %w", err)
	}

	job.entryID = entryID

	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()

	if s.hooks.OnSchedule != nil {
		s.hooks.OnSchedule(job)
	}

	if s.metrics != nil {
		s.metrics.JobScheduled(job)
	}

	s.logger.Info("job scheduled", "id", job.ID, "name", job.Name, "schedule", schedule)
	return job, nil
}

// ScheduleFunc is a convenience wrapper for Schedule with a simple function.
func (s *Scheduler) ScheduleFunc(name, schedule string, fn func()) (*Job, error) {
	return s.Schedule(name, schedule, func(ctx context.Context, job *Job) error {
		fn()
		return nil
	})
}

// Remove removes a job from the scheduler.
func (s *Scheduler) Remove(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	s.cron.Remove(job.entryID)
	delete(s.jobs, jobID)

	if s.hooks.OnRemove != nil {
		s.hooks.OnRemove(job)
	}

	s.logger.Info("job removed", "id", jobID, "name", job.Name)
	return nil
}

// Pause pauses a job.
func (s *Scheduler) Pause(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.Enabled = false
	s.logger.Info("job paused", "id", jobID, "name", job.Name)
	return nil
}

// Resume resumes a paused job.
func (s *Scheduler) Resume(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.Enabled = true
	s.logger.Info("job resumed", "id", jobID, "name", job.Name)
	return nil
}

// RunNow triggers immediate execution of a job.
func (s *Scheduler) RunNow(jobID string) error {
	s.mu.RLock()
	job, exists := s.jobs[jobID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	go s.executeJob(job)
	return nil
}

// Get returns a job by ID.
func (s *Scheduler) Get(jobID string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return job, nil
}

// GetByName returns a job by name.
func (s *Scheduler) GetByName(name string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, job := range s.jobs {
		if job.Name == name {
			return job, nil
		}
	}
	return nil, fmt.Errorf("job not found: %s", name)
}

// List returns all jobs.
func (s *Scheduler) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// ListEnabled returns all enabled jobs.
func (s *Scheduler) ListEnabled() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*Job, 0)
	for _, job := range s.jobs {
		if job.Enabled {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

// Count returns the number of scheduled jobs.
func (s *Scheduler) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

func (s *Scheduler) createJobRunner(job *Job) func() {
	return func() {
		s.executeJob(job)
	}
}

func (s *Scheduler) executeJob(job *Job) {
	if !job.Enabled {
		return
	}

	start := time.Now()

	if s.hooks.BeforeRun != nil {
		s.hooks.BeforeRun(job)
	}

	if s.metrics != nil {
		s.metrics.JobStarted(job)
	}

	// Create context with timeout if specified
	ctx := context.Background()
	var cancel context.CancelFunc
	if job.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, job.Timeout)
		defer cancel()
	}

	// Execute with retries
	var err error
	for attempt := 0; attempt <= job.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(job.RetryDelay)
		}

		if s.executor != nil {
			err = s.executor.Execute(ctx, job)
		} else {
			err = job.Handler(ctx, job)
		}

		if err == nil {
			break
		}
	}

	duration := time.Since(start)

	// Update job stats
	s.mu.Lock()
	now := time.Now()
	job.LastRun = &now
	job.RunCount++
	if err != nil {
		job.ErrorCount++
	}
	// Calculate next run
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if sched, parseErr := parser.Parse(job.Schedule); parseErr == nil {
		next := sched.Next(now)
		job.NextRun = &next
	}
	s.mu.Unlock()

	// Callbacks and metrics
	if s.hooks.AfterRun != nil {
		s.hooks.AfterRun(job, duration, err)
	}

	if err != nil {
		if s.hooks.OnError != nil {
			s.hooks.OnError(job, err)
		}
		if s.errorHandler != nil {
			s.errorHandler(job, err)
		}
		if s.metrics != nil {
			s.metrics.JobFailed(job, err, duration)
		}
		s.logger.Error("job failed", "id", job.ID, "name", job.Name, "error", err.Error(), "duration", duration)
	} else {
		if s.metrics != nil {
			s.metrics.JobCompleted(job, duration)
		}
		s.logger.Debug("job completed", "id", job.ID, "name", job.Name, "duration", duration)
	}
}

// JobOption configures a job.
type JobOption func(*Job)

// WithJobTimeout sets the job timeout.
func WithJobTimeout(d time.Duration) JobOption {
	return func(j *Job) {
		j.Timeout = d
	}
}

// WithJobRetries sets retry configuration.
func WithJobRetries(maxRetries int, delay time.Duration) JobOption {
	return func(j *Job) {
		j.MaxRetries = maxRetries
		j.RetryDelay = delay
	}
}

// WithJobTags sets job tags.
func WithJobTags(tags map[string]string) JobOption {
	return func(j *Job) {
		j.Tags = tags
	}
}

// WithJobID sets a custom job ID.
func WithJobID(id string) JobOption {
	return func(j *Job) {
		j.ID = id
	}
}

// WithJobTimezone sets the job timezone.
func WithJobTimezone(tz string) JobOption {
	return func(j *Job) {
		j.Timezone = tz
	}
}

// WithJobDisabled creates the job in disabled state.
func WithJobDisabled() JobOption {
	return func(j *Job) {
		j.Enabled = false
	}
}

// defaultLogger is a simple default logger.
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, keysAndValues ...interface{}) {
	log.Printf("[DEBUG] %s %v", msg, keysAndValues)
}

func (l *defaultLogger) Info(msg string, keysAndValues ...interface{}) {
	log.Printf("[INFO] %s %v", msg, keysAndValues)
}

func (l *defaultLogger) Warn(msg string, keysAndValues ...interface{}) {
	log.Printf("[WARN] %s %v", msg, keysAndValues)
}

func (l *defaultLogger) Error(msg string, keysAndValues ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, keysAndValues)
}
