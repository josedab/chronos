package scheduler

import (
	"os"
	"testing"

	"github.com/chronos/chronos/internal/dispatcher"
	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/storage"
	"github.com/rs/zerolog"
)

func setupBenchScheduler(b *testing.B) (*Scheduler, func()) {
	b.Helper()

	tmpDir, err := os.MkdirTemp("", "scheduler-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		b.Fatalf("failed to create store: %v", err)
	}

	disp := dispatcher.New(dispatcher.DefaultConfig())
	logger := zerolog.Nop()

	sched := New(store, disp, logger, nil)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return sched, cleanup
}

func BenchmarkScheduler_AddJob(b *testing.B) {
	sched, cleanup := setupBenchScheduler(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &models.Job{
			ID:       "job-" + string(rune(i)),
			Name:     "bench-job",
			Schedule: "* * * * *",
			Timezone: "UTC",
			Enabled:  true,
		}
		_ = sched.AddJob(job)
	}
}

func BenchmarkScheduler_GetJob(b *testing.B) {
	sched, cleanup := setupBenchScheduler(b)
	defer cleanup()

	// Pre-populate with jobs
	for i := 0; i < 1000; i++ {
		job := &models.Job{
			ID:       "job-" + string(rune(i)),
			Name:     "bench-job",
			Schedule: "* * * * *",
			Timezone: "UTC",
			Enabled:  true,
		}
		_ = sched.AddJob(job)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sched.GetJob("job-500")
	}
}

func BenchmarkScheduler_GetMetrics(b *testing.B) {
	sched, cleanup := setupBenchScheduler(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sched.GetMetrics()
	}
}
