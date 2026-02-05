// Package scheduler provides the core job scheduling engine for Chronos.
//
// The scheduler is responsible for tracking job schedules, determining when jobs
// should run, and dispatching executions to the dispatcher. It operates as a
// background service that ticks at regular intervals (default: 1 second) to check
// for due jobs.
//
// # Architecture
//
// The scheduler maintains several key data structures:
//
//   - Job cache: In-memory map of all jobs for fast lookup
//   - Schedule cache: Parsed cron expressions to avoid re-parsing
//   - Priority queue: Min-heap of next run times for efficient scheduling
//
// # Leader Election
//
// In a clustered deployment, only the Raft leader runs the scheduler actively.
// Follower nodes maintain schedule state but don't dispatch jobs. When leadership
// changes, the new leader's scheduler becomes active automatically.
//
// # Missed Run Handling
//
// The scheduler supports three policies for handling missed runs (when Chronos
// was down during a scheduled execution time):
//
//   - ignore: Skip missed runs entirely
//   - execute_one: Execute the job once to catch up
//   - execute_all: Execute all missed runs in order
//
// # Concurrency Control
//
// Each job can configure a concurrency policy that determines behavior when
// a new execution is due while a previous execution is still running:
//
//   - allow: Start the new execution (parallel execution)
//   - forbid: Skip the new execution until the current one completes
//   - replace: Cancel the current execution and start a new one
//
// # Usage
//
// The scheduler is typically created and started by the main Chronos server:
//
//	store := storage.NewBadgerStore(cfg.DataDir)
//	disp := dispatcher.New(dispatcher.DefaultConfig())
//	sched := scheduler.New(store, disp, logger, scheduler.DefaultConfig())
//
//	// Start the scheduler (blocks until context is cancelled)
//	go sched.Start(ctx)
//
//	// Add a job
//	job := &models.Job{
//	    Name:     "daily-report",
//	    Schedule: "0 9 * * *",
//	    Webhook:  models.Webhook{URL: "https://api.example.com/report"},
//	    Enabled:  true,
//	}
//	sched.AddJob(job)
//
// # Metrics
//
// The scheduler exposes Prometheus metrics for monitoring:
//
//   - chronos_scheduler_jobs_total: Total number of registered jobs
//   - chronos_scheduler_jobs_enabled: Number of enabled jobs
//   - chronos_scheduler_scheduled_runs: Total scheduled runs
//   - chronos_scheduler_missed_runs: Total missed runs detected
//   - chronos_scheduler_executions_total: Total executions dispatched
//
// # Thread Safety
//
// The scheduler is fully thread-safe. All internal state is protected by
// appropriate mutexes, and metrics use atomic counters.
package scheduler
