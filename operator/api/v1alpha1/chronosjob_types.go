// Package v1alpha1 contains API Schema definitions for Chronos resources.
// +kubebuilder:object:generate=true
// +groupName=chronos.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChronosJobSpec defines the desired state of ChronosJob.
type ChronosJobSpec struct {
	// Name is the human-readable name of the job.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description is an optional description of the job.
	// +optional
	Description string `json:"description,omitempty"`

	// Schedule is the cron expression for when the job should run.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-3])|\*\/([0-9]|1[0-9]|2[0-3])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$`
	Schedule string `json:"schedule"`

	// Timezone is the IANA timezone for the schedule.
	// +optional
	// +kubebuilder:default="UTC"
	Timezone string `json:"timezone,omitempty"`

	// Enabled determines if the job is active.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Webhook defines the HTTP endpoint to call when the job runs.
	// +kubebuilder:validation:Required
	Webhook WebhookSpec `json:"webhook"`

	// Timeout is the maximum duration for job execution.
	// +optional
	// +kubebuilder:default="30s"
	Timeout string `json:"timeout,omitempty"`

	// Concurrency defines how overlapping executions are handled.
	// +optional
	// +kubebuilder:validation:Enum=allow;forbid;replace
	// +kubebuilder:default="forbid"
	Concurrency string `json:"concurrency,omitempty"`

	// RetryPolicy defines retry behavior on failure.
	// +optional
	RetryPolicy *RetryPolicySpec `json:"retryPolicy,omitempty"`

	// ChronosRef optionally references an external Chronos server.
	// If not specified, uses the default Chronos instance.
	// +optional
	ChronosRef *ChronosReference `json:"chronosRef,omitempty"`
}

// WebhookSpec defines the webhook configuration.
type WebhookSpec struct {
	// URL is the webhook endpoint.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Method is the HTTP method to use.
	// +optional
	// +kubebuilder:validation:Enum=GET;POST;PUT;PATCH;DELETE
	// +kubebuilder:default="POST"
	Method string `json:"method,omitempty"`

	// Headers are additional HTTP headers to include.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the request body for POST/PUT/PATCH requests.
	// +optional
	Body string `json:"body,omitempty"`

	// SecretRef references a Secret containing sensitive headers.
	// +optional
	SecretRef *SecretKeySelector `json:"secretRef,omitempty"`
}

// RetryPolicySpec defines retry behavior.
type RetryPolicySpec struct {
	// MaxAttempts is the maximum number of retry attempts.
	// +optional
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	MaxAttempts int `json:"maxAttempts,omitempty"`

	// InitialInterval is the initial delay between retries.
	// +optional
	// +kubebuilder:default="1s"
	InitialInterval string `json:"initialInterval,omitempty"`

	// MaxInterval is the maximum delay between retries.
	// +optional
	// +kubebuilder:default="1m"
	MaxInterval string `json:"maxInterval,omitempty"`

	// Multiplier is the backoff multiplier.
	// +optional
	// +kubebuilder:default=2
	Multiplier float64 `json:"multiplier,omitempty"`
}

// ChronosReference references a Chronos server.
type ChronosReference struct {
	// Name is the name of the Chronos server.
	Name string `json:"name"`

	// Namespace is the namespace of the Chronos server.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// SecretKeySelector selects a key from a Secret.
type SecretKeySelector struct {
	// Name is the name of the Secret.
	Name string `json:"name"`

	// Key is the key in the Secret to select.
	Key string `json:"key"`
}

// ChronosJobStatus defines the observed state of ChronosJob.
type ChronosJobStatus struct {
	// Phase is the current phase of the job.
	// +optional
	Phase JobPhase `json:"phase,omitempty"`

	// ChronosJobID is the ID assigned by the Chronos server.
	// +optional
	ChronosJobID string `json:"chronosJobID,omitempty"`

	// LastScheduledTime is when the job was last scheduled.
	// +optional
	LastScheduledTime *metav1.Time `json:"lastScheduledTime,omitempty"`

	// LastSuccessfulTime is when the job last succeeded.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`

	// LastFailedTime is when the job last failed.
	// +optional
	LastFailedTime *metav1.Time `json:"lastFailedTime,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Message provides additional status information.
	// +optional
	Message string `json:"message,omitempty"`
}

// JobPhase represents the phase of a ChronosJob.
// +kubebuilder:validation:Enum=Pending;Synced;Failed;Disabled
type JobPhase string

const (
	JobPhasePending  JobPhase = "Pending"
	JobPhaseSynced   JobPhase = "Synced"
	JobPhaseFailed   JobPhase = "Failed"
	JobPhaseDisabled JobPhase = "Disabled"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ChronosJob is the Schema for the chronosjobs API.
type ChronosJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChronosJobSpec   `json:"spec,omitempty"`
	Status ChronosJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChronosJobList contains a list of ChronosJob.
type ChronosJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChronosJob `json:"items"`
}
