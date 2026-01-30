// Package marketplace provides workflow templates for Chronos.
package marketplace

import "time"

// WorkflowTemplate represents a complete workflow template.
type WorkflowTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    WorkflowCategory       `json:"category"`
	Author      string                 `json:"author"`
	Version     string                 `json:"version"`
	Tags        []string               `json:"tags"`
	Icon        string                 `json:"icon"`
	Steps       []WorkflowStepTemplate `json:"steps"`
	Triggers    []TriggerTemplate      `json:"triggers"`
	Variables   []TemplateVariable     `json:"variables"`
	Outputs     []OutputDefinition     `json:"outputs"`
	Metadata    map[string]string      `json:"metadata,omitempty"`

	// Community fields
	Downloads   int       `json:"downloads"`
	Rating      float64   `json:"rating"`
	RatingCount int       `json:"rating_count"`
	Verified    bool      `json:"verified"`
	Featured    bool      `json:"featured"`
	License     string    `json:"license"`
	Repository  string    `json:"repository,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Readme      string    `json:"readme"`
}

// WorkflowCategory represents workflow categories.
type WorkflowCategory string

const (
	WorkflowCategoryETL          WorkflowCategory = "etl"
	WorkflowCategoryCI           WorkflowCategory = "ci_cd"
	WorkflowCategoryMonitoring   WorkflowCategory = "monitoring"
	WorkflowCategoryMaintenance  WorkflowCategory = "maintenance"
	WorkflowCategoryReporting    WorkflowCategory = "reporting"
	WorkflowCategoryIncident     WorkflowCategory = "incident_response"
	WorkflowCategoryData         WorkflowCategory = "data_pipeline"
	WorkflowCategoryNotification WorkflowCategory = "notification"
)

// WorkflowStepTemplate represents a step in a workflow template.
type WorkflowStepTemplate struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         StepType               `json:"type"`
	Config       map[string]interface{} `json:"config"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Condition    string                 `json:"condition,omitempty"`
	Timeout      string                 `json:"timeout,omitempty"`
	RetryPolicy  *RetryPolicyTemplate   `json:"retry_policy,omitempty"`
	OnSuccess    []string               `json:"on_success,omitempty"`
	OnFailure    []string               `json:"on_failure,omitempty"`
}

// StepType represents the type of workflow step.
type StepType string

const (
	StepTypeJob          StepType = "job"
	StepTypeHTTP         StepType = "http"
	StepTypeCondition    StepType = "condition"
	StepTypeParallel     StepType = "parallel"
	StepTypeDelay        StepType = "delay"
	StepTypeApproval     StepType = "approval"
	StepTypeNotification StepType = "notification"
	StepTypeScript       StepType = "script"
	StepTypeTransform    StepType = "transform"
)

// TriggerTemplate represents a workflow trigger.
type TriggerTemplate struct {
	Type    TriggerType            `json:"type"`
	Name    string                 `json:"name"`
	Config  map[string]interface{} `json:"config"`
	Enabled bool                   `json:"enabled"`
}

// TriggerType represents trigger types.
type TriggerType string

const (
	TriggerTypeCron    TriggerType = "cron"
	TriggerTypeWebhook TriggerType = "webhook"
	TriggerTypeManual  TriggerType = "manual"
	TriggerTypeEvent   TriggerType = "event"
)

// OutputDefinition defines workflow outputs.
type OutputDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Source      string `json:"source"` // Step.output path
}

// WorkflowFilter defines filtering criteria.
type WorkflowFilter struct {
	Category   WorkflowCategory `json:"category,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Author     string           `json:"author,omitempty"`
	Verified   *bool            `json:"verified,omitempty"`
	Featured   *bool            `json:"featured,omitempty"`
	MinRating  float64          `json:"min_rating,omitempty"`
	SearchTerm string           `json:"search_term,omitempty"`
}

// TemplateSubmission represents a submitted template.
type TemplateSubmission struct {
	ID          string            `json:"id"`
	Template    *WorkflowTemplate `json:"template"`
	AuthorID    string            `json:"author_id"`
	Status      SubmissionStatus  `json:"status"`
	ReviewNotes string            `json:"review_notes,omitempty"`
	SubmittedAt time.Time         `json:"submitted_at"`
	ReviewedAt  *time.Time        `json:"reviewed_at,omitempty"`
	ReviewerID  string            `json:"reviewer_id,omitempty"`
}

// SubmissionStatus represents submission review status.
type SubmissionStatus string

const (
	SubmissionPending  SubmissionStatus = "pending"
	SubmissionApproved SubmissionStatus = "approved"
	SubmissionRejected SubmissionStatus = "rejected"
	SubmissionRevision SubmissionStatus = "needs_revision"
)

// WorkflowInstance is an instantiated workflow.
type WorkflowInstance struct {
	ID         string                 `json:"id"`
	TemplateID string                 `json:"template_id"`
	Name       string                 `json:"name"`
	Steps      []WorkflowStepInstance `json:"steps"`
	Variables  map[string]string      `json:"variables"`
	CreatedAt  time.Time              `json:"created_at"`
}

// WorkflowStepInstance is an instantiated step.
type WorkflowStepInstance struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         StepType               `json:"type"`
	Config       map[string]interface{} `json:"config"`
	Dependencies []string               `json:"dependencies"`
}
