package scheduler

import (
	"fmt"
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

func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := NewRateLimiter(RateLimitConfig{DefaultRate: 360000, DefaultBurst: 1000})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rl.Allow("bench-ns")
		rl.Release("bench-ns")
	}
}

func BenchmarkRateLimiter_MultiNamespace(b *testing.B) {
	rl := NewRateLimiter(RateLimitConfig{DefaultRate: 360000, DefaultBurst: 100})
	ns := []string{"ns-1", "ns-2", "ns-3", "ns-4", "ns-5"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rl.Allow(ns[i%5])
		rl.Release(ns[i%5])
	}
}

func BenchmarkDependencyResolver_TopologicalSort(b *testing.B) {
	dr := NewDependencyResolver(nil, zerolog.Nop())
	ids := make([]string, 20)
	for i := 0; i < 20; i++ {
		ids[i] = fmt.Sprintf("job-%d", i)
		if i > 0 {
			dr.RegisterJob(&models.Job{
				ID:           ids[i],
				Dependencies: &models.DependencyConfig{DependsOn: []string{ids[i-1]}},
			})
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dr.TopologicalOrder(ids)
	}
}
