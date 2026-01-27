// Package raft provides the Finite State Machine for Raft consensus.
package raft

import (
	"encoding/json"
	"io"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/storage"
	"github.com/hashicorp/raft"
	"github.com/rs/zerolog"
)

// FSMStore defines the storage operations needed by the FSM.
type FSMStore interface {
	storage.JobStore
	storage.ExecutionStore
	storage.LockStore
	storage.SnapshotStore
}

// FSM implements raft.FSM for job state.
type FSM struct {
	store  FSMStore
	logger zerolog.Logger
}

// NewFSM creates a new FSM.
func NewFSM(store FSMStore, logger zerolog.Logger) *FSM {
	return &FSM{
		store:  store,
		logger: logger.With().Str("component", "fsm").Logger(),
	}
}

// Apply applies a Raft log entry to the FSM.
func (f *FSM) Apply(log *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		f.logger.Error().Err(err).Msg("Failed to unmarshal command")
		return err
	}

	f.logger.Debug().
		Str("type", string(cmd.Type)).
		Uint64("index", log.Index).
		Msg("Applying command")

	switch cmd.Type {
	case CmdCreateJob:
		return f.applyCreateJob(cmd.Payload)
	case CmdUpdateJob:
		return f.applyUpdateJob(cmd.Payload)
	case CmdDeleteJob:
		return f.applyDeleteJob(cmd.Payload)
	case CmdRecordExecution:
		return f.applyRecordExecution(cmd.Payload)
	case CmdAcquireLock:
		return f.applyAcquireLock(cmd.Payload)
	case CmdReleaseLock:
		return f.applyReleaseLock(cmd.Payload)
	default:
		f.logger.Warn().Str("type", string(cmd.Type)).Msg("Unknown command type")
		return nil
	}
}

// Snapshot returns a snapshot of the FSM.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.logger.Debug().Msg("Creating snapshot")

	data, err := f.store.Snapshot()
	if err != nil {
		return nil, err
	}

	return &FSMSnapshot{data: data}, nil
}

// Restore restores the FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.logger.Info().Msg("Restoring from snapshot")
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	return f.store.Restore(data)
}

// Apply methods

func (f *FSM) applyCreateJob(payload json.RawMessage) interface{} {
	var job models.Job
	if err := json.Unmarshal(payload, &job); err != nil {
		return err
	}

	if err := f.store.CreateJob(&job); err != nil {
		return err
	}

	f.logger.Info().Str("job_id", job.ID).Str("job_name", job.Name).Msg("Job created")
	return nil
}

func (f *FSM) applyUpdateJob(payload json.RawMessage) interface{} {
	var job models.Job
	if err := json.Unmarshal(payload, &job); err != nil {
		return err
	}

	if err := f.store.UpdateJob(&job); err != nil {
		return err
	}

	f.logger.Info().Str("job_id", job.ID).Msg("Job updated")
	return nil
}

func (f *FSM) applyDeleteJob(payload json.RawMessage) interface{} {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	if err := f.store.DeleteJob(req.ID); err != nil {
		return err
	}

	f.logger.Info().Str("job_id", req.ID).Msg("Job deleted")
	return nil
}

func (f *FSM) applyRecordExecution(payload json.RawMessage) interface{} {
	var exec models.Execution
	if err := json.Unmarshal(payload, &exec); err != nil {
		return err
	}

	if err := f.store.RecordExecution(&exec); err != nil {
		return err
	}

	return nil
}

func (f *FSM) applyAcquireLock(payload json.RawMessage) interface{} {
	var req struct {
		JobID  string        `json:"job_id"`
		NodeID string        `json:"node_id"`
		TTL    time.Duration `json:"ttl"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	acquired, err := f.store.AcquireLock(req.JobID, req.NodeID, req.TTL)
	if err != nil {
		return err
	}

	if !acquired {
		return models.ErrLockNotAcquired
	}

	return nil
}

func (f *FSM) applyReleaseLock(payload json.RawMessage) interface{} {
	var req struct {
		JobID  string `json:"job_id"`
		NodeID string `json:"node_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return err
	}

	return f.store.ReleaseLock(req.JobID, req.NodeID)
}

// FSMSnapshot represents a snapshot of the FSM state.
type FSMSnapshot struct {
	data []byte
}

// Persist writes the snapshot to the given sink.
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	if _, err := sink.Write(s.data); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

// Release releases the snapshot resources.
func (s *FSMSnapshot) Release() {}
