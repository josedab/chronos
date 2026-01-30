// Package v1alpha1 contains API Schema definitions for Chronos resources.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChronosDAGSpec defines the desired state of ChronosDAG.
type ChronosDAGSpec struct {
	// Name is the human-readable name of the DAG.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description is an optional description of the DAG.
	// +optional
	Description string `json:"description,omitempty"`

	// Schedule is the cron expression for when the DAG should run.
	// If not specified, the DAG must be triggered manually.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Timezone is the IANA timezone for the schedule.
	// +optional
	// +kubebuilder:default="UTC"
	Timezone string `json:"timezone,omitempty"`

	// Nodes defines the nodes in the DAG.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Nodes []DAGNodeSpec `json:"nodes"`

	// ChronosRef optionally references an external Chronos server.
	// +optional
	ChronosRef *ChronosReference `json:"chronosRef,omitempty"`
}

// DAGNodeSpec defines a node in the DAG.
type DAGNodeSpec struct {
	// ID is the unique identifier for this node within the DAG.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	ID string `json:"id"`

	// JobRef references a ChronosJob to execute.
	// Either JobRef or JobSpec must be provided.
	// +optional
	JobRef *JobReference `json:"jobRef,omitempty"`

	// JobSpec defines an inline job specification.
	// Either JobRef or JobSpec must be provided.
	// +optional
	JobSpec *ChronosJobSpec `json:"jobSpec,omitempty"`

	// Dependencies lists the node IDs that must complete before this node.
	// +optional
	Dependencies []string `json:"dependencies,omitempty"`

	// Condition determines when this node should trigger.
	// +optional
	// +kubebuilder:validation:Enum=all_success;all_complete;any_success;none
	// +kubebuilder:default="all_success"
	Condition string `json:"condition,omitempty"`

	// Timeout is the maximum duration to wait for this node.
	// +optional
	// +kubebuilder:default="5m"
	Timeout string `json:"timeout,omitempty"`
}

// JobReference references a ChronosJob.
type JobReference struct {
	// Name is the name of the ChronosJob.
	Name string `json:"name"`

	// Namespace is the namespace of the ChronosJob.
	// Defaults to the DAG's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ChronosDAGStatus defines the observed state of ChronosDAG.
type ChronosDAGStatus struct {
	// Phase is the current phase of the DAG.
	// +optional
	Phase DAGPhase `json:"phase,omitempty"`

	// ChronosDAGID is the ID assigned by the Chronos server.
	// +optional
	ChronosDAGID string `json:"chronosDAGID,omitempty"`

	// LastRunTime is when the DAG was last executed.
	// +optional
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`

	// LastRunStatus is the status of the last run.
	// +optional
	LastRunStatus string `json:"lastRunStatus,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// NodeStatuses tracks the status of each node.
	// +optional
	NodeStatuses map[string]NodeStatus `json:"nodeStatuses,omitempty"`

	// Message provides additional status information.
	// +optional
	Message string `json:"message,omitempty"`
}

// DAGPhase represents the phase of a ChronosDAG.
// +kubebuilder:validation:Enum=Pending;Valid;Invalid;Synced;Failed
type DAGPhase string

const (
	DAGPhasePending DAGPhase = "Pending"
	DAGPhaseValid   DAGPhase = "Valid"
	DAGPhaseInvalid DAGPhase = "Invalid"
	DAGPhaseSynced  DAGPhase = "Synced"
	DAGPhaseFailed  DAGPhase = "Failed"
)

// NodeStatus represents the status of a node in a DAG run.
type NodeStatus struct {
	// Phase is the current phase of the node.
	Phase string `json:"phase,omitempty"`

	// StartTime is when the node started executing.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// EndTime is when the node finished executing.
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.spec.nodes`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ChronosDAG is the Schema for the chronosdags API.
type ChronosDAG struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChronosDAGSpec   `json:"spec,omitempty"`
	Status ChronosDAGStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChronosDAGList contains a list of ChronosDAG.
type ChronosDAGList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChronosDAG `json:"items"`
}
