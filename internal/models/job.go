// Package models defines the core data structures for Chronos.
package models

import (
	"time"

	"github.com/chronos/chronos/pkg/duration"
)

// Duration is an alias for the shared duration.Duration type.
type Duration = duration.Duration

// Job represents a scheduled job.
type Job struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Schedule    string            `json:"schedule"` // Cron expression
	Timezone    string            `json:"timezone,omitempty"`

	// Execution config
	Webhook *WebhookConfig `json:"webhook,omitempty"`

	// Shadow/Canary execution
	ShadowWebhook *WebhookConfig `json:"shadow_webhook,omitempty"`
	CanaryPercent int            `json:"canary_percent,omitempty"` // 0-100, percentage routed to shadow

	// Retry policy
	RetryPolicy *RetryPolicy `json:"retry_policy,omitempty"`

	// Execution constraints
	Timeout     Duration          `json:"timeout,omitempty"`
	Concurrency ConcurrencyPolicy `json:"concurrency,omitempty"`

	// Metadata
	Tags      map[string]string `json:"tags,omitempty"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`

	// Versioning
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by,omitempty"`

	// Dependencies (DAG support)
	Dependencies *DependencyConfig `json:"dependencies,omitempty"`

	// Multi-tenant support
	Namespace string `json:"namespace,omitempty"`
}

// JobVersion represents a historical version of a job configuration.
type JobVersion struct {
	ID        string    `json:"id"`
	JobID     string    `json:"job_id"`
	Version   int       `json:"version"`
	Config    Job       `json:"config"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
	Comment   string    `json:"comment,omitempty"`
}

// WebhookConfig defines HTTP webhook execution.
type WebhookConfig struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	Auth         *AuthConfig       `json:"auth,omitempty"`
	SuccessCodes []int             `json:"success_codes,omitempty"` // Default: 200-299
	Assertions   *ResponseAssertions `json:"assertions,omitempty"`
}

// ResponseAssertions defines expected response characteristics.
type ResponseAssertions struct {
	// MaxResponseTime is the maximum acceptable response time.
	MaxResponseTime Duration `json:"max_response_time,omitempty"`
	// BodyContains checks that the response body contains all specified strings.
	BodyContains []string `json:"body_contains,omitempty"`
	// BodyNotContains checks that the response body does not contain any specified strings.
	BodyNotContains []string `json:"body_not_contains,omitempty"`
	// JSONPathMatchers checks specific JSON path values.
	JSONPathMatchers []JSONPathMatcher `json:"json_path,omitempty"`
	// Mode controls assertion behavior: "enforce" fails the execution, "warn" logs only.
	Mode AssertionMode `json:"mode,omitempty"`
}

// JSONPathMatcher asserts a value at a simple dot-notation path.
type JSONPathMatcher struct {
	Path     string `json:"path"`               // e.g. "status", "data.count"
	Expected string `json:"expected"`            // expected value as string
	Operator string `json:"operator,omitempty"`  // "equals" (default), "contains", "not_empty"
}

// AssertionMode controls how assertion failures are handled.
type AssertionMode string

const (
	AssertionModeEnforce AssertionMode = "enforce"
	AssertionModeWarn    AssertionMode = "warn"
)

// AssertionResult holds the outcome of response assertions.
type AssertionResult struct {
	Passed     bool               `json:"passed"`
	Violations []AssertionViolation `json:"violations,omitempty"`
}

// AssertionViolation describes a single assertion failure.
type AssertionViolation struct {
	Type     string `json:"type"`     // "response_time", "body_contains", "body_not_contains", "json_path"
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

// AuthConfig defines authentication for webhooks.
type AuthConfig struct {
	Type   AuthType `json:"type"`
	Token  string   `json:"token,omitempty"`  // For bearer
	APIKey string   `json:"api_key,omitempty"` // For api_key
	Header string   `json:"header,omitempty"`  // Header name for api_key

	// Basic auth
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// AuthType defines the authentication method.
type AuthType string

const (
	AuthTypeNone   AuthType = ""
	AuthTypeBasic  AuthType = "basic"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeAPIKey AuthType = "api_key"
)

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	MaxAttempts     int      `json:"max_attempts"`
	InitialInterval Duration `json:"initial_interval"`
	MaxInterval     Duration `json:"max_interval"`
	Multiplier      float64  `json:"multiplier"`
}

// DefaultRetryPolicy returns the default retry policy.
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:     3,
		InitialInterval: Duration(time.Second),
		MaxInterval:     Duration(time.Minute),
		Multiplier:      2.0,
	}
}

// ConcurrencyPolicy handles overlapping executions.
type ConcurrencyPolicy string

const (
	ConcurrencyAllow   ConcurrencyPolicy = "allow"   // Allow concurrent runs
	ConcurrencyForbid  ConcurrencyPolicy = "forbid"  // Skip if already running
	ConcurrencyReplace ConcurrencyPolicy = "replace" // Cancel existing, start new
)

// DependencyConfig defines job dependencies for DAG scheduling.
type DependencyConfig struct {
	// Jobs that must complete successfully before this job runs
	DependsOn []string `json:"depends_on,omitempty"`
	// Condition for triggering based on dependencies
	Condition DependencyCondition `json:"condition,omitempty"`
	// Timeout for waiting on dependencies
	Timeout Duration `json:"timeout,omitempty"`
}

// DependencyCondition specifies when a dependent job should trigger.
type DependencyCondition string

const (
	// DependencyConditionAllSuccess triggers when all dependencies succeed
	DependencyConditionAllSuccess DependencyCondition = "all_success"
	// DependencyConditionAllComplete triggers when all dependencies complete (success or failure)
	DependencyConditionAllComplete DependencyCondition = "all_complete"
	// DependencyConditionAnySuccess triggers when any dependency succeeds
	DependencyConditionAnySuccess DependencyCondition = "any_success"
	// DependencyConditionAnyComplete triggers when any dependency completes
	DependencyConditionAnyComplete DependencyCondition = "any_complete"
)

// DAGRun represents a single execution of a DAG (group of dependent jobs).
type DAGRun struct {
	ID        string            `json:"id"`
	Name      string            `json:"name,omitempty"`
	Jobs      []string          `json:"jobs"`
	Status    DAGRunStatus      `json:"status"`
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at,omitempty"`
	JobStates map[string]string `json:"job_states"` // jobID -> status
}

// DAGRunStatus represents the status of a DAG run.
type DAGRunStatus string

const (
	DAGRunPending   DAGRunStatus = "pending"
	DAGRunRunning   DAGRunStatus = "running"
	DAGRunSuccess   DAGRunStatus = "success"
	DAGRunFailed    DAGRunStatus = "failed"
	DAGRunCancelled DAGRunStatus = "cancelled"
)

// Namespace represents a tenant namespace for multi-tenancy.
type Namespace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Quota       *NamespaceQuota   `json:"quota,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// NamespaceQuota defines resource limits for a namespace.
type NamespaceQuota struct {
	MaxJobs              int `json:"max_jobs,omitempty"`
	MaxConcurrentRuns    int `json:"max_concurrent_runs,omitempty"`
	MaxExecutionsPerHour int `json:"max_executions_per_hour,omitempty"`
}

// Validate validates the job configuration.
func (j *Job) Validate() error {
	if j.Name == "" {
		return ErrJobNameRequired
	}
	if j.Schedule == "" {
		return ErrScheduleRequired
	}
	if j.Webhook == nil {
		return ErrWebhookRequired
	}
	if j.Webhook.URL == "" {
		return ErrWebhookURLRequired
	}
	if j.Webhook.Method == "" {
		j.Webhook.Method = "GET"
	}
	return nil
}
