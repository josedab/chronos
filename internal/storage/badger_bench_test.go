package storage

import (
	"os"
	"testing"

	"github.com/chronos/chronos/internal/models"
)

func setupBenchStore(b *testing.B) (*BadgerStore, func()) {
	b.Helper()

	tmpDir, err := os.MkdirTemp("", "storage-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		b.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func BenchmarkStore_CreateJob(b *testing.B) {
	store, cleanup := setupBenchStore(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &models.Job{
			ID:       "job-" + string(rune(i)),
			Name:     "bench-job",
			Schedule: "* * * * *",
			Enabled:  true,
		}
		_ = store.CreateJob(job)
	}
}

func BenchmarkStore_GetJob(b *testing.B) {
	store, cleanup := setupBenchStore(b)
	defer cleanup()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		job := &models.Job{
			ID:       "job-" + string(rune(i)),
			Name:     "bench-job",
			Schedule: "* * * * *",
			Enabled:  true,
		}
		_ = store.CreateJob(job)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetJob("job-500")
	}
}

func BenchmarkStore_ListJobs(b *testing.B) {
	store, cleanup := setupBenchStore(b)
	defer cleanup()

	// Pre-populate
	for i := 0; i < 100; i++ {
		job := &models.Job{
			ID:       "job-" + string(rune(i)),
			Name:     "bench-job",
			Schedule: "* * * * *",
			Enabled:  true,
		}
		_ = store.CreateJob(job)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.ListJobs()
	}
}
