// Package storage provides storage interfaces and implementations for Chronos.
package storage

import (
	"time"

	"github.com/chronos/chronos/internal/models"
)

// JobStore provides job persistence operations.
type JobStore interface {
	// CreateJob stores a new job. Returns ErrJobAlreadyExists if job ID exists.
	CreateJob(job *models.Job) error
	// UpdateJob updates an existing job. Returns ErrJobNotFound if job doesn't exist.
	UpdateJob(job *models.Job) error
	// GetJob retrieves a job by ID. Returns ErrJobNotFound if not found.
	GetJob(id string) (*models.Job, error)
	// DeleteJob deletes a job by ID. Returns ErrJobNotFound if not found.
	DeleteJob(id string) error
	// ListJobs returns all jobs.
	ListJobs() ([]*models.Job, error)
	// ListJobsPaginated returns jobs with pagination support.
	// offset is the starting index, limit is max jobs to return.
	// Returns jobs and total count.
	ListJobsPaginated(offset, limit int) ([]*models.Job, int, error)
}

// ExecutionStore provides execution record persistence operations.
type ExecutionStore interface {
	// RecordExecution stores an execution record.
	RecordExecution(exec *models.Execution) error
	// UpdateExecution updates an execution record.
	UpdateExecution(exec *models.Execution) error
	// GetExecution retrieves an execution by job ID and execution ID.
	GetExecution(jobID, execID string) (*models.Execution, error)
	// ListExecutions returns executions for a job, limited to the specified count.
	ListExecutions(jobID string, limit int) ([]*models.Execution, error)
	// DeleteExecutions deletes all executions for a job.
	DeleteExecutions(jobID string) error
}

// ScheduleStore provides schedule state persistence operations.
type ScheduleStore interface {
	// SaveScheduleState saves the schedule state for a job.
	SaveScheduleState(state *models.ScheduleState) error
	// GetScheduleState retrieves the schedule state for a job.
	GetScheduleState(jobID string) (*models.ScheduleState, error)
}

// LockStore provides distributed locking operations.
type LockStore interface {
	// AcquireLock attempts to acquire a lock for a job execution.
	// Returns true if the lock was acquired, false if held by another node.
	AcquireLock(jobID, nodeID string, ttl time.Duration) (bool, error)
	// ReleaseLock releases a lock for a job execution.
	ReleaseLock(jobID, nodeID string) error
	// IsLocked checks if a job is locked. Returns locked status and holder node ID.
	IsLocked(jobID string) (bool, string, error)
}

// VersionStore provides job versioning operations.
type VersionStore interface {
	// SaveJobVersion saves a version of a job configuration.
	SaveJobVersion(jobVersion *models.JobVersion) error
	// ListJobVersions returns all versions of a job.
	ListJobVersions(jobID string) ([]*models.JobVersion, error)
	// GetJobVersion retrieves a specific version of a job.
	GetJobVersion(jobID string, version int) (*models.JobVersion, error)
}

// SnapshotStore provides snapshot operations for Raft integration.
type SnapshotStore interface {
	// Snapshot creates a snapshot of all data.
	Snapshot() ([]byte, error)
	// Restore restores data from a snapshot.
	Restore(snapshot []byte) error
}

// Store combines all storage interfaces.
// This is the primary interface for components that need full storage access.
type Store interface {
	JobStore
	ExecutionStore
	ScheduleStore
	LockStore
	VersionStore
	SnapshotStore

	// Close closes the store and releases resources.
	Close() error
}
