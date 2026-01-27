// Package raft provides command types for Raft consensus.
package raft

import (
	"encoding/json"
	"time"

	"github.com/chronos/chronos/internal/models"
)

// CommandType represents the type of Raft command.
type CommandType string

const (
	CmdCreateJob       CommandType = "create_job"
	CmdUpdateJob       CommandType = "update_job"
	CmdDeleteJob       CommandType = "delete_job"
	CmdRecordExecution CommandType = "record_execution"
	CmdAcquireLock     CommandType = "acquire_lock"
	CmdReleaseLock     CommandType = "release_lock"
)

// Command represents a Raft command.
type Command struct {
	Type    CommandType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// NewCreateJobCommand creates a new create job command.
func NewCreateJobCommand(job *models.Job) (*Command, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	return &Command{
		Type:    CmdCreateJob,
		Payload: payload,
	}, nil
}

// NewUpdateJobCommand creates a new update job command.
func NewUpdateJobCommand(job *models.Job) (*Command, error) {
	payload, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	return &Command{
		Type:    CmdUpdateJob,
		Payload: payload,
	}, nil
}

// NewDeleteJobCommand creates a new delete job command.
func NewDeleteJobCommand(jobID string) (*Command, error) {
	payload, err := json.Marshal(struct {
		ID string `json:"id"`
	}{ID: jobID})
	if err != nil {
		return nil, err
	}
	return &Command{
		Type:    CmdDeleteJob,
		Payload: payload,
	}, nil
}

// NewRecordExecutionCommand creates a new record execution command.
func NewRecordExecutionCommand(exec *models.Execution) (*Command, error) {
	payload, err := json.Marshal(exec)
	if err != nil {
		return nil, err
	}
	return &Command{
		Type:    CmdRecordExecution,
		Payload: payload,
	}, nil
}

// NewAcquireLockCommand creates a new acquire lock command.
func NewAcquireLockCommand(jobID, nodeID string, ttl time.Duration) (*Command, error) {
	payload, err := json.Marshal(struct {
		JobID  string        `json:"job_id"`
		NodeID string        `json:"node_id"`
		TTL    time.Duration `json:"ttl"`
	}{
		JobID:  jobID,
		NodeID: nodeID,
		TTL:    ttl,
	})
	if err != nil {
		return nil, err
	}
	return &Command{
		Type:    CmdAcquireLock,
		Payload: payload,
	}, nil
}

// NewReleaseLockCommand creates a new release lock command.
func NewReleaseLockCommand(jobID, nodeID string) (*Command, error) {
	payload, err := json.Marshal(struct {
		JobID  string `json:"job_id"`
		NodeID string `json:"node_id"`
	}{
		JobID:  jobID,
		NodeID: nodeID,
	})
	if err != nil {
		return nil, err
	}
	return &Command{
		Type:    CmdReleaseLock,
		Payload: payload,
	}, nil
}
