// Package v1alpha1 contains API Schema definitions for Chronos resources.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ChronosTriggerSpec defines the desired state of ChronosTrigger.
type ChronosTriggerSpec struct {
	// Name is the human-readable name of the trigger.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type is the type of event source.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=kafka;sqs;pubsub;rabbitmq;webhook
	Type string `json:"type"`

	// JobRef references the ChronosJob to trigger.
	// +kubebuilder:validation:Required
	JobRef JobReference `json:"jobRef"`

	// Enabled determines if the trigger is active.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Kafka configures a Kafka trigger.
	// +optional
	Kafka *KafkaTriggerConfig `json:"kafka,omitempty"`

	// SQS configures an AWS SQS trigger.
	// +optional
	SQS *SQSTriggerConfig `json:"sqs,omitempty"`

	// PubSub configures a Google Cloud Pub/Sub trigger.
	// +optional
	PubSub *PubSubTriggerConfig `json:"pubsub,omitempty"`

	// RabbitMQ configures a RabbitMQ trigger.
	// +optional
	RabbitMQ *RabbitMQTriggerConfig `json:"rabbitmq,omitempty"`

	// Webhook configures a webhook trigger.
	// +optional
	Webhook *WebhookTriggerConfig `json:"webhook,omitempty"`

	// Filter defines event filtering rules.
	// +optional
	Filter *EventFilterSpec `json:"filter,omitempty"`

	// ChronosRef optionally references an external Chronos server.
	// +optional
	ChronosRef *ChronosReference `json:"chronosRef,omitempty"`
}

// KafkaTriggerConfig configures a Kafka trigger.
type KafkaTriggerConfig struct {
	// Brokers is the list of Kafka broker addresses.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Brokers []string `json:"brokers"`

	// Topic is the Kafka topic to consume from.
	// +kubebuilder:validation:Required
	Topic string `json:"topic"`

	// ConsumerGroup is the consumer group ID.
	// +optional
	ConsumerGroup string `json:"consumerGroup,omitempty"`

	// SASLSecretRef references a Secret containing SASL credentials.
	// +optional
	SASLSecretRef *SecretKeySelector `json:"saslSecretRef,omitempty"`
}

// SQSTriggerConfig configures an AWS SQS trigger.
type SQSTriggerConfig struct {
	// QueueURL is the URL of the SQS queue.
	// +kubebuilder:validation:Required
	QueueURL string `json:"queueURL"`

	// Region is the AWS region.
	// +kubebuilder:validation:Required
	Region string `json:"region"`

	// CredentialsSecretRef references a Secret containing AWS credentials.
	// +optional
	CredentialsSecretRef *SecretKeySelector `json:"credentialsSecretRef,omitempty"`
}

// PubSubTriggerConfig configures a Google Cloud Pub/Sub trigger.
type PubSubTriggerConfig struct {
	// Project is the GCP project ID.
	// +kubebuilder:validation:Required
	Project string `json:"project"`

	// Subscription is the Pub/Sub subscription name.
	// +kubebuilder:validation:Required
	Subscription string `json:"subscription"`

	// CredentialsSecretRef references a Secret containing GCP credentials.
	// +optional
	CredentialsSecretRef *SecretKeySelector `json:"credentialsSecretRef,omitempty"`
}

// RabbitMQTriggerConfig configures a RabbitMQ trigger.
type RabbitMQTriggerConfig struct {
	// URL is the RabbitMQ connection URL.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Queue is the queue name to consume from.
	// +kubebuilder:validation:Required
	Queue string `json:"queue"`

	// CredentialsSecretRef references a Secret containing credentials.
	// +optional
	CredentialsSecretRef *SecretKeySelector `json:"credentialsSecretRef,omitempty"`
}

// WebhookTriggerConfig configures a webhook trigger.
type WebhookTriggerConfig struct {
	// Path is the URL path to listen on.
	// +optional
	// +kubebuilder:default="/"
	Path string `json:"path,omitempty"`

	// SecretRef references a Secret containing the webhook secret.
	// +optional
	SecretRef *SecretKeySelector `json:"secretRef,omitempty"`
}

// EventFilterSpec defines event filtering rules.
type EventFilterSpec struct {
	// EventTypes filters by event type.
	// +optional
	EventTypes []string `json:"eventTypes,omitempty"`

	// Headers filters by header values.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Expression is a CEL expression for filtering.
	// +optional
	Expression string `json:"expression,omitempty"`
}

// ChronosTriggerStatus defines the observed state of ChronosTrigger.
type ChronosTriggerStatus struct {
	// Phase is the current phase of the trigger.
	// +optional
	Phase TriggerPhase `json:"phase,omitempty"`

	// ChronosTriggerID is the ID assigned by the Chronos server.
	// +optional
	ChronosTriggerID string `json:"chronosTriggerID,omitempty"`

	// Connected indicates if the trigger is connected to the event source.
	// +optional
	Connected bool `json:"connected,omitempty"`

	// LastEventTime is when the last event was received.
	// +optional
	LastEventTime *metav1.Time `json:"lastEventTime,omitempty"`

	// EventsReceived is the total number of events received.
	// +optional
	EventsReceived int64 `json:"eventsReceived,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Message provides additional status information.
	// +optional
	Message string `json:"message,omitempty"`
}

// TriggerPhase represents the phase of a ChronosTrigger.
// +kubebuilder:validation:Enum=Pending;Synced;Connected;Disconnected;Failed
type TriggerPhase string

const (
	TriggerPhasePending      TriggerPhase = "Pending"
	TriggerPhaseSynced       TriggerPhase = "Synced"
	TriggerPhaseConnected    TriggerPhase = "Connected"
	TriggerPhaseDisconnected TriggerPhase = "Disconnected"
	TriggerPhaseFailed       TriggerPhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ChronosTrigger is the Schema for the chronostriggers API.
type ChronosTrigger struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChronosTriggerSpec   `json:"spec,omitempty"`
	Status ChronosTriggerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ChronosTriggerList contains a list of ChronosTrigger.
type ChronosTriggerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ChronosTrigger `json:"items"`
}
