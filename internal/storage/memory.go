package storage

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
)

// MemoryStore implements the Store interface using in-memory data structures.
// Useful for testing and development.
type MemoryStore struct {
	jobs           map[string]*models.Job
	executions     map[string][]*models.Execution // jobID -> executions
	scheduleStates map[string]*models.ScheduleState
	locks          map[string]*lockInfo
	versions       map[string][]*models.JobVersion // jobID -> versions
	mu             sync.RWMutex
}

type lockInfo struct {
	nodeID    string
	expiresAt time.Time
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs:           make(map[string]*models.Job),
		executions:     make(map[string][]*models.Execution),
		scheduleStates: make(map[string]*models.ScheduleState),
		locks:          make(map[string]*lockInfo),
		versions:       make(map[string][]*models.JobVersion),
	}
}

// CreateJob stores a new job.
func (s *MemoryStore) CreateJob(job *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; exists {
		return models.ErrJobAlreadyExists
	}

	jobCopy := *job
	s.jobs[job.ID] = &jobCopy
	return nil
}

// UpdateJob updates an existing job.
func (s *MemoryStore) UpdateJob(job *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[job.ID]; !exists {
		return models.ErrJobNotFound
	}

	jobCopy := *job
	s.jobs[job.ID] = &jobCopy
	return nil
}

// GetJob retrieves a job by ID.
func (s *MemoryStore) GetJob(id string) (*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, exists := s.jobs[id]
	if !exists {
		return nil, models.ErrJobNotFound
	}

	jobCopy := *job
	return &jobCopy, nil
}

// DeleteJob deletes a job by ID.
func (s *MemoryStore) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.jobs[id]; !exists {
		return models.ErrJobNotFound
	}

	delete(s.jobs, id)
	return nil
}

// ListJobs returns all jobs.
func (s *MemoryStore) ListJobs() ([]*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	return jobs, nil
}

// ListJobsPaginated returns jobs with pagination support.
func (s *MemoryStore) ListJobsPaginated(offset, limit int) ([]*models.Job, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert to slice and sort by ID for consistent ordering
	allJobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobCopy := *job
		allJobs = append(allJobs, &jobCopy)
	}
	sort.Slice(allJobs, func(i, j int) bool {
		return allJobs[i].ID < allJobs[j].ID
	})

	total := len(allJobs)
	if offset >= total {
		return []*models.Job{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return allJobs[offset:end], total, nil
}

// RecordExecution stores an execution record.
func (s *MemoryStore) RecordExecution(exec *models.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	execCopy := *exec
	s.executions[exec.JobID] = append(s.executions[exec.JobID], &execCopy)
	return nil
}

// UpdateExecution updates an execution record.
func (s *MemoryStore) UpdateExecution(exec *models.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	execs, exists := s.executions[exec.JobID]
	if !exists {
		return nil
	}

	for i, e := range execs {
		if e.ID == exec.ID {
			execCopy := *exec
			s.executions[exec.JobID][i] = &execCopy
			return nil
		}
	}
	return nil
}

// GetExecution retrieves an execution by job ID and execution ID.
func (s *MemoryStore) GetExecution(jobID, execID string) (*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execs, exists := s.executions[jobID]
	if !exists {
		return nil, models.ErrExecutionNotFound
	}

	for _, exec := range execs {
		if exec.ID == execID {
			execCopy := *exec
			return &execCopy, nil
		}
	}
	return nil, models.ErrExecutionNotFound
}

// ListExecutions returns executions for a job, limited to the specified count.
func (s *MemoryStore) ListExecutions(jobID string, limit int) ([]*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execs, exists := s.executions[jobID]
	if !exists {
		return []*models.Execution{}, nil
	}

	// Sort by ScheduledTime descending (most recent first)
	sorted := make([]*models.Execution, len(execs))
	copy(sorted, execs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ScheduledTime.After(sorted[j].ScheduledTime)
	})

	if limit > len(sorted) {
		limit = len(sorted)
	}

	result := make([]*models.Execution, limit)
	for i := 0; i < limit; i++ {
		execCopy := *sorted[i]
		result[i] = &execCopy
	}
	return result, nil
}

// DeleteExecutions deletes all executions for a job.
func (s *MemoryStore) DeleteExecutions(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.executions, jobID)
	return nil
}

// SaveScheduleState saves the schedule state for a job.
func (s *MemoryStore) SaveScheduleState(state *models.ScheduleState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	stateCopy := *state
	s.scheduleStates[state.JobID] = &stateCopy
	return nil
}

// GetScheduleState retrieves the schedule state for a job.
func (s *MemoryStore) GetScheduleState(jobID string) (*models.ScheduleState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, exists := s.scheduleStates[jobID]
	if !exists {
		return nil, models.ErrScheduleStateNotFound
	}

	stateCopy := *state
	return &stateCopy, nil
}

// AcquireLock attempts to acquire a lock for a job execution.
func (s *MemoryStore) AcquireLock(jobID, nodeID string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Check if lock exists and is still valid
	if lock, exists := s.locks[jobID]; exists {
		if lock.expiresAt.After(now) && lock.nodeID != nodeID {
			return false, nil
		}
	}

	s.locks[jobID] = &lockInfo{
		nodeID:    nodeID,
		expiresAt: now.Add(ttl),
	}
	return true, nil
}

// ReleaseLock releases a lock for a job execution.
func (s *MemoryStore) ReleaseLock(jobID, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lock, exists := s.locks[jobID]; exists {
		if lock.nodeID == nodeID {
			delete(s.locks, jobID)
		}
	}
	return nil
}

// IsLocked checks if a job is locked.
func (s *MemoryStore) IsLocked(jobID string) (bool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lock, exists := s.locks[jobID]
	if !exists {
		return false, "", nil
	}

	if lock.expiresAt.Before(time.Now()) {
		return false, "", nil
	}

	return true, lock.nodeID, nil
}

// SaveJobVersion saves a version of a job configuration.
func (s *MemoryStore) SaveJobVersion(jobVersion *models.JobVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versionCopy := *jobVersion
	s.versions[jobVersion.JobID] = append(s.versions[jobVersion.JobID], &versionCopy)
	return nil
}

// ListJobVersions returns all versions of a job.
func (s *MemoryStore) ListJobVersions(jobID string) ([]*models.JobVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[jobID]
	if !exists {
		return []*models.JobVersion{}, nil
	}

	result := make([]*models.JobVersion, len(versions))
	for i, v := range versions {
		versionCopy := *v
		result[i] = &versionCopy
	}

	// Sort by version number descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version > result[j].Version
	})

	return result, nil
}

// GetJobVersion retrieves a specific version of a job.
func (s *MemoryStore) GetJobVersion(jobID string, version int) (*models.JobVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[jobID]
	if !exists {
		return nil, models.ErrVersionNotFound
	}

	for _, v := range versions {
		if v.Version == version {
			versionCopy := *v
			return &versionCopy, nil
		}
	}
	return nil, models.ErrVersionNotFound
}

// snapshotData holds the data for serialization during snapshot.
type snapshotData struct {
	Jobs           map[string]*models.Job           `json:"jobs"`
	Executions     map[string][]*models.Execution   `json:"executions"`
	ScheduleStates map[string]*models.ScheduleState `json:"schedule_states"`
	Versions       map[string][]*models.JobVersion  `json:"versions"`
}

// Snapshot creates a snapshot of all data.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := snapshotData{
		Jobs:           s.jobs,
		Executions:     s.executions,
		ScheduleStates: s.scheduleStates,
		Versions:       s.versions,
	}

	return json.Marshal(data)
}

// Restore restores data from a snapshot.
func (s *MemoryStore) Restore(snapshot []byte) error {
	var data snapshotData
	if err := json.Unmarshal(snapshot, &data); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs = data.Jobs
	s.executions = data.Executions
	s.scheduleStates = data.ScheduleStates
	s.versions = data.Versions

	if s.jobs == nil {
		s.jobs = make(map[string]*models.Job)
	}
	if s.executions == nil {
		s.executions = make(map[string][]*models.Execution)
	}
	if s.scheduleStates == nil {
		s.scheduleStates = make(map[string]*models.ScheduleState)
	}
	if s.versions == nil {
		s.versions = make(map[string][]*models.JobVersion)
	}

	return nil
}

// Close closes the store and releases resources.
func (s *MemoryStore) Close() error {
	return nil
}

// Reset clears all data (useful for testing).
func (s *MemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs = make(map[string]*models.Job)
	s.executions = make(map[string][]*models.Execution)
	s.scheduleStates = make(map[string]*models.ScheduleState)
	s.locks = make(map[string]*lockInfo)
	s.versions = make(map[string][]*models.JobVersion)
}
