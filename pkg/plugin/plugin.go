// Package plugin defines the plugin architecture for Chronos extensibility.
package plugin

import (
	"context"
	"time"
)

// Type represents the type of plugin.
type Type string

const (
	// TypeExecutor is a plugin that executes jobs (e.g., gRPC, Lambda).
	TypeExecutor Type = "executor"
	// TypeTrigger is a plugin that triggers jobs (e.g., Kafka, SQS).
	TypeTrigger Type = "trigger"
	// TypeNotifier is a plugin that sends notifications (e.g., Slack, PagerDuty).
	TypeNotifier Type = "notifier"
	// TypeSecretProvider is a plugin that provides secrets (e.g., Vault, AWS Secrets Manager).
	TypeSecretProvider Type = "secret_provider"
	// TypeStorage is a plugin that provides storage backend.
	TypeStorage Type = "storage"
)

// Metadata contains plugin metadata.
type Metadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        Type   `json:"type"`
	Description string `json:"description"`
	Author      string `json:"author,omitempty"`
	License     string `json:"license,omitempty"`
}

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	// Metadata returns plugin metadata.
	Metadata() Metadata
	// Init initializes the plugin with configuration.
	Init(ctx context.Context, config map[string]interface{}) error
	// Close cleans up plugin resources.
	Close(ctx context.Context) error
	// Health returns plugin health status.
	Health(ctx context.Context) error
}

// ExecutionRequest contains the request for job execution.
type ExecutionRequest struct {
	JobID       string                 `json:"job_id"`
	JobName     string                 `json:"job_name"`
	ExecutionID string                 `json:"execution_id"`
	Payload     map[string]interface{} `json:"payload"`
	Timeout     time.Duration          `json:"timeout"`
	Metadata    map[string]string      `json:"metadata"`
}

// ExecutionResult contains the result of job execution.
type ExecutionResult struct {
	Success      bool                   `json:"success"`
	StatusCode   int                    `json:"status_code,omitempty"`
	Response     []byte                 `json:"response,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Executor is a plugin that executes jobs.
type Executor interface {
	Plugin
	// Execute runs a job and returns the result.
	Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)
}

// TriggerEvent represents an event that triggers a job.
type TriggerEvent struct {
	Source    string                 `json:"source"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]string      `json:"metadata"`
}

// TriggerHandler is called when a trigger event occurs.
type TriggerHandler func(ctx context.Context, event *TriggerEvent) error

// Trigger is a plugin that triggers jobs based on external events.
type Trigger interface {
	Plugin
	// Start begins listening for events.
	Start(ctx context.Context, handler TriggerHandler) error
	// Stop stops listening for events.
	Stop(ctx context.Context) error
}

// Notification represents a notification to be sent.
type Notification struct {
	JobID       string            `json:"job_id"`
	JobName     string            `json:"job_name"`
	ExecutionID string            `json:"execution_id"`
	Status      string            `json:"status"`
	Message     string            `json:"message"`
	Details     map[string]string `json:"details,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Notifier is a plugin that sends notifications.
type Notifier interface {
	Plugin
	// Notify sends a notification.
	Notify(ctx context.Context, notification *Notification) error
}

// SecretProvider is a plugin that provides secrets.
type SecretProvider interface {
	Plugin
	// GetSecret retrieves a secret by key.
	GetSecret(ctx context.Context, key string) (string, error)
	// ListSecrets lists available secrets (keys only).
	ListSecrets(ctx context.Context, prefix string) ([]string, error)
}

// Storage is a plugin that provides storage backend.
type Storage interface {
	Plugin
	// Get retrieves a value by key.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores a value.
	Set(ctx context.Context, key string, value []byte) error
	// Delete removes a value.
	Delete(ctx context.Context, key string) error
	// List lists keys with prefix.
	List(ctx context.Context, prefix string) ([]string, error)
}
