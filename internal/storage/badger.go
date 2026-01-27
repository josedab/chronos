// Package storage provides persistent storage using BadgerDB.
package storage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/dgraph-io/badger/v4"
)

// Compile-time check that BadgerStore implements all storage interfaces.
var (
	_ JobStore       = (*BadgerStore)(nil)
	_ ExecutionStore = (*BadgerStore)(nil)
	_ ScheduleStore  = (*BadgerStore)(nil)
	_ LockStore      = (*BadgerStore)(nil)
	_ VersionStore   = (*BadgerStore)(nil)
	_ SnapshotStore  = (*BadgerStore)(nil)
)

// BadgerStore provides persistent storage using BadgerDB.
type BadgerStore struct {
	db     *badger.DB
	prefix map[string][]byte
	mu     sync.RWMutex
	stopCh chan struct{}
}

// Prefix keys for different data types.
const (
	prefixJobs       = "jobs/"
	prefixExecutions = "executions/"
	prefixSchedule   = "schedule/"
	prefixLocks      = "locks/"
	prefixVersions   = "versions/"
)

// NewStore creates a new BadgerDB store.
func NewStore(dataDir string) (*BadgerStore, error) {
	dbPath := filepath.Join(dataDir, "chronos.db")

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Use custom logger
	opts.SyncWrites = true
	opts.ValueLogFileSize = 64 << 20 // 64MB

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &BadgerStore{
		db: db,
		prefix: map[string][]byte{
			"jobs":       []byte(prefixJobs),
			"executions": []byte(prefixExecutions),
			"schedule":   []byte(prefixSchedule),
			"locks":      []byte(prefixLocks),
		},
		stopCh: make(chan struct{}),
	}

	// Start garbage collection
	go s.runGC()

	return s, nil
}

// Close closes the database and stops background goroutines.
func (s *BadgerStore) Close() error {
	close(s.stopCh)
	return s.db.Close()
}

// runGC runs periodic garbage collection.
func (s *BadgerStore) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			for {
				err := s.db.RunValueLogGC(0.5)
				if err == badger.ErrNoRewrite {
					break
				}
				if err != nil {
					break
				}
			}
		}
	}
}

// Job Operations

// CreateJob stores a new job.
func (s *BadgerStore) CreateJob(job *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.jobKey(job.ID)

		// Check if job already exists
		_, err := txn.Get(key)
		if err == nil {
			return models.ErrJobAlreadyExists
		}
		if err != badger.ErrKeyNotFound {
			return err
		}

		data, err := json.Marshal(job)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// UpdateJob updates an existing job.
func (s *BadgerStore) UpdateJob(job *models.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.jobKey(job.ID)

		// Check if job exists
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return models.ErrJobNotFound
		}
		if err != nil {
			return err
		}

		data, err := json.Marshal(job)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// GetJob retrieves a job by ID.
func (s *BadgerStore) GetJob(id string) (*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var job models.Job

	err := s.db.View(func(txn *badger.Txn) error {
		key := s.jobKey(id)

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return models.ErrJobNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &job)
		})
	})

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// DeleteJob deletes a job by ID.
func (s *BadgerStore) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.jobKey(id)

		// Check if job exists
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return models.ErrJobNotFound
		}
		if err != nil {
			return err
		}

		return txn.Delete(key)
	})
}

// ListJobs returns all jobs.
func (s *BadgerStore) ListJobs() ([]*models.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobs []*models.Job

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.prefix["jobs"]

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var job models.Job
				if err := json.Unmarshal(val, &job); err != nil {
					return err
				}
				jobs = append(jobs, &job)
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return jobs, err
}

// ListJobsPaginated returns jobs with pagination support.
func (s *BadgerStore) ListJobsPaginated(offset, limit int) ([]*models.Job, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobs []*models.Job
	total := 0
	skipped := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = s.prefix["jobs"]

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			total++

			// Skip until offset
			if skipped < offset {
				skipped++
				continue
			}

			// Stop after limit
			if limit > 0 && len(jobs) >= limit {
				// Continue counting total
				continue
			}

			item := it.Item()
			err := item.Value(func(val []byte) error {
				var job models.Job
				if err := json.Unmarshal(val, &job); err != nil {
					return err
				}
				jobs = append(jobs, &job)
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return jobs, total, err
}

// Execution Operations

// RecordExecution stores an execution record.
func (s *BadgerStore) RecordExecution(exec *models.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.executionKey(exec.JobID, exec.ID)

		data, err := json.Marshal(exec)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// UpdateExecution updates an execution record.
func (s *BadgerStore) UpdateExecution(exec *models.Execution) error {
	return s.RecordExecution(exec)
}

// GetExecution retrieves an execution by ID.
func (s *BadgerStore) GetExecution(jobID, execID string) (*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var exec models.Execution

	err := s.db.View(func(txn *badger.Txn) error {
		key := s.executionKey(jobID, execID)

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return models.ErrExecutionNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &exec)
		})
	})

	if err != nil {
		return nil, err
	}

	return &exec, nil
}

// ListExecutions returns executions for a job.
func (s *BadgerStore) ListExecutions(jobID string, limit int) ([]*models.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var executions []*models.Execution

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(prefixExecutions + jobID + "/")

		opts := badger.DefaultIteratorOptions
		opts.Reverse = true // Newest first
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Seek(append(prefix, 0xFF)); it.ValidForPrefix(prefix) && count < limit; it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var exec models.Execution
				if err := json.Unmarshal(val, &exec); err != nil {
					return err
				}
				executions = append(executions, &exec)
				return nil
			})

			if err != nil {
				return err
			}
			count++
		}

		return nil
	})

	return executions, err
}

// DeleteExecutions deletes all executions for a job.
func (s *BadgerStore) DeleteExecutions(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		prefix := []byte(prefixExecutions + jobID + "/")

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		var keysToDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysToDelete = append(keysToDelete, key)
		}

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// Schedule State Operations

// SaveScheduleState saves the schedule state for a job.
func (s *BadgerStore) SaveScheduleState(state *models.ScheduleState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.scheduleKey(state.JobID)

		data, err := json.Marshal(state)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// GetScheduleState retrieves the schedule state for a job.
func (s *BadgerStore) GetScheduleState(jobID string) (*models.ScheduleState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var state models.ScheduleState

	err := s.db.View(func(txn *badger.Txn) error {
		key := s.scheduleKey(jobID)

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil // No state yet
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &state)
		})
	})

	if err != nil {
		return nil, err
	}

	if state.JobID == "" {
		return nil, nil
	}

	return &state, nil
}

// Lock Operations

// AcquireLock attempts to acquire a lock for a job execution.
func (s *BadgerStore) AcquireLock(jobID, nodeID string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acquired := false

	err := s.db.Update(func(txn *badger.Txn) error {
		key := s.lockKey(jobID)

		item, err := txn.Get(key)
		if err == nil {
			// Lock exists, check if expired
			var lock lockEntry
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &lock)
			}); err != nil {
				return err
			}

			if time.Now().Before(lock.ExpiresAt) {
				// Lock is still held
				return nil
			}
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		// Acquire the lock
		lock := lockEntry{
			JobID:     jobID,
			NodeID:    nodeID,
			AcquiredAt: time.Now(),
			ExpiresAt: time.Now().Add(ttl),
		}

		data, err := json.Marshal(lock)
		if err != nil {
			return err
		}

		if err := txn.Set(key, data); err != nil {
			return err
		}

		acquired = true
		return nil
	})

	return acquired, err
}

// ReleaseLock releases a lock for a job execution.
func (s *BadgerStore) ReleaseLock(jobID, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.lockKey(jobID)

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil // Lock doesn't exist
		}
		if err != nil {
			return err
		}

		var lock lockEntry
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &lock)
		}); err != nil {
			return err
		}

		// Only release if we own the lock
		if lock.NodeID != nodeID {
			return nil
		}

		return txn.Delete(key)
	})
}

// IsLocked checks if a job is locked.
func (s *BadgerStore) IsLocked(jobID string) (bool, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var locked bool
	var nodeID string

	err := s.db.View(func(txn *badger.Txn) error {
		key := s.lockKey(jobID)

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		var lock lockEntry
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &lock)
		}); err != nil {
			return err
		}

		if time.Now().Before(lock.ExpiresAt) {
			locked = true
			nodeID = lock.NodeID
		}

		return nil
	})

	return locked, nodeID, err
}

// lockEntry represents a lock in the store.
type lockEntry struct {
	JobID      string    `json:"job_id"`
	NodeID     string    `json:"node_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Key helpers

func (s *BadgerStore) jobKey(id string) []byte {
	return []byte(prefixJobs + id)
}

func (s *BadgerStore) executionKey(jobID, execID string) []byte {
	return []byte(prefixExecutions + jobID + "/" + execID)
}

func (s *BadgerStore) scheduleKey(jobID string) []byte {
	return []byte(prefixSchedule + jobID)
}

func (s *BadgerStore) lockKey(jobID string) []byte {
	return []byte(prefixLocks + jobID)
}

func (s *BadgerStore) versionKey(jobID string, version int) []byte {
	return []byte(fmt.Sprintf("%s%s/%d", prefixVersions, jobID, version))
}

// Job Versioning Operations

// SaveJobVersion saves a version of a job configuration.
func (s *BadgerStore) SaveJobVersion(jobVersion *models.JobVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		key := s.versionKey(jobVersion.JobID, jobVersion.Version)

		data, err := json.Marshal(jobVersion)
		if err != nil {
			return err
		}

		return txn.Set(key, data)
	})
}

// ListJobVersions returns all versions of a job.
func (s *BadgerStore) ListJobVersions(jobID string) ([]*models.JobVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var versions []*models.JobVersion

	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(prefixVersions + jobID + "/")

		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var version models.JobVersion
				if err := json.Unmarshal(val, &version); err != nil {
					return err
				}
				versions = append(versions, &version)
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return versions, err
}

// GetJobVersion retrieves a specific version of a job.
func (s *BadgerStore) GetJobVersion(jobID string, version int) (*models.JobVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var jobVersion models.JobVersion

	err := s.db.View(func(txn *badger.Txn) error {
		key := s.versionKey(jobID, version)

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return models.ErrJobVersionNotFound
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &jobVersion)
		})
	})

	if err != nil {
		return nil, err
	}

	return &jobVersion, nil
}

// Snapshot creates a snapshot of all data for Raft.
func (s *BadgerStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := make(map[string]json.RawMessage)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())

			err := item.Value(func(val []byte) error {
				data[key] = val
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

// Restore restores data from a snapshot.
func (s *BadgerStore) Restore(snapshot []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var data map[string]json.RawMessage
	if err := json.Unmarshal(snapshot, &data); err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Clear existing data
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		var keysToDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			keysToDelete = append(keysToDelete, it.Item().KeyCopy(nil))
		}
		it.Close()

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		// Restore data
		for key, val := range data {
			if err := txn.Set([]byte(key), val); err != nil {
				return err
			}
		}

		return nil
	})
}
