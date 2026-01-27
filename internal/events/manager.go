// Package events provides event-driven job triggers for Chronos.
package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common errors.
var (
	ErrTriggerNotFound    = errors.New("trigger not found")
	ErrTriggerExists      = errors.New("trigger already exists")
	ErrInvalidTriggerType = errors.New("invalid trigger type")
	ErrConnectionFailed   = errors.New("connection failed")
)

// TriggerType represents the type of event trigger.
type TriggerType string

const (
	TriggerKafka     TriggerType = "kafka"
	TriggerSQS       TriggerType = "sqs"
	TriggerPubSub    TriggerType = "pubsub"
	TriggerRabbitMQ  TriggerType = "rabbitmq"
	TriggerWebhook   TriggerType = "webhook"
	TriggerSchedule  TriggerType = "schedule"
)

// Event represents an incoming event that triggers a job.
type Event struct {
	ID        string                 `json:"id"`
	Source    TriggerType            `json:"source"`
	Type      string                 `json:"type"`
	Subject   string                 `json:"subject,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// Trigger defines an event trigger configuration.
type Trigger struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        TriggerType       `json:"type"`
	JobID       string            `json:"job_id"`
	Config      TriggerConfig     `json:"config"`
	Filter      *EventFilter      `json:"filter,omitempty"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TriggerConfig contains type-specific configuration.
type TriggerConfig struct {
	// Kafka configuration
	Brokers       []string `json:"brokers,omitempty" yaml:"brokers,omitempty"`
	Topic         string   `json:"topic,omitempty" yaml:"topic,omitempty"`
	GroupID       string   `json:"group_id,omitempty" yaml:"group_id,omitempty"`

	// SQS configuration
	QueueURL     string `json:"queue_url,omitempty" yaml:"queue_url,omitempty"`
	Region       string `json:"region,omitempty" yaml:"region,omitempty"`
	WaitTime     int    `json:"wait_time,omitempty" yaml:"wait_time,omitempty"`

	// Pub/Sub configuration
	ProjectID    string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	Subscription string `json:"subscription,omitempty" yaml:"subscription,omitempty"`

	// RabbitMQ configuration
	URL          string `json:"url,omitempty" yaml:"url,omitempty"`
	Queue        string `json:"queue,omitempty" yaml:"queue,omitempty"`
	Exchange     string `json:"exchange,omitempty" yaml:"exchange,omitempty"`
	RoutingKey   string `json:"routing_key,omitempty" yaml:"routing_key,omitempty"`

	// Webhook configuration
	Path         string   `json:"path,omitempty" yaml:"path,omitempty"`
	Methods      []string `json:"methods,omitempty" yaml:"methods,omitempty"`
	Secret       string   `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// EventFilter filters events before triggering.
type EventFilter struct {
	// EventTypes matches specific event types.
	EventTypes []string `json:"event_types,omitempty"`
	// Condition is a CEL expression for filtering.
	Condition string `json:"condition,omitempty"`
	// Headers matches specific headers.
	Headers map[string]string `json:"headers,omitempty"`
}

// Handler processes events and triggers jobs.
type Handler func(ctx context.Context, event *Event) error

// Source is a source of events (e.g., Kafka, SQS).
type Source interface {
	// Type returns the trigger type.
	Type() TriggerType
	// Connect establishes connection to the source.
	Connect(ctx context.Context) error
	// Disconnect closes the connection.
	Disconnect(ctx context.Context) error
	// Subscribe starts consuming events.
	Subscribe(ctx context.Context, config TriggerConfig, handler Handler) error
	// Unsubscribe stops consuming events.
	Unsubscribe(ctx context.Context) error
	// Health returns source health status.
	Health(ctx context.Context) error
}

// Manager manages event triggers.
type Manager struct {
	mu        sync.RWMutex
	triggers  map[string]*Trigger
	sources   map[TriggerType]Source
	handler   Handler
	active    map[string]context.CancelFunc
}

// NewManager creates a new event manager.
func NewManager(handler Handler) *Manager {
	return &Manager{
		triggers: make(map[string]*Trigger),
		sources:  make(map[TriggerType]Source),
		handler:  handler,
		active:   make(map[string]context.CancelFunc),
	}
}

// RegisterSource registers an event source.
func (m *Manager) RegisterSource(source Source) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[source.Type()] = source
}

// CreateTrigger creates a new event trigger.
func (m *Manager) CreateTrigger(trigger *Trigger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.triggers[trigger.ID]; exists {
		return ErrTriggerExists
	}

	now := time.Now().UTC()
	trigger.CreatedAt = now
	trigger.UpdatedAt = now

	m.triggers[trigger.ID] = trigger

	if trigger.Enabled {
		return m.startTrigger(trigger)
	}

	return nil
}

// GetTrigger returns a trigger by ID.
func (m *Manager) GetTrigger(id string) (*Trigger, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trigger, exists := m.triggers[id]
	if !exists {
		return nil, ErrTriggerNotFound
	}
	return trigger, nil
}

// ListTriggers returns all triggers.
func (m *Manager) ListTriggers() []*Trigger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	triggers := make([]*Trigger, 0, len(m.triggers))
	for _, t := range m.triggers {
		triggers = append(triggers, t)
	}
	return triggers
}

// UpdateTrigger updates a trigger.
func (m *Manager) UpdateTrigger(trigger *Trigger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.triggers[trigger.ID]
	if !exists {
		return ErrTriggerNotFound
	}

	// Stop existing if running
	if cancel, ok := m.active[trigger.ID]; ok {
		cancel()
		delete(m.active, trigger.ID)
	}

	trigger.CreatedAt = existing.CreatedAt
	trigger.UpdatedAt = time.Now().UTC()
	m.triggers[trigger.ID] = trigger

	if trigger.Enabled {
		return m.startTrigger(trigger)
	}

	return nil
}

// DeleteTrigger deletes a trigger.
func (m *Manager) DeleteTrigger(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.triggers[id]; !exists {
		return ErrTriggerNotFound
	}

	// Stop if running
	if cancel, ok := m.active[id]; ok {
		cancel()
		delete(m.active, id)
	}

	delete(m.triggers, id)
	return nil
}

// EnableTrigger enables a trigger.
func (m *Manager) EnableTrigger(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigger, exists := m.triggers[id]
	if !exists {
		return ErrTriggerNotFound
	}

	if trigger.Enabled {
		return nil
	}

	trigger.Enabled = true
	trigger.UpdatedAt = time.Now().UTC()

	return m.startTrigger(trigger)
}

// DisableTrigger disables a trigger.
func (m *Manager) DisableTrigger(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigger, exists := m.triggers[id]
	if !exists {
		return ErrTriggerNotFound
	}

	if !trigger.Enabled {
		return nil
	}

	trigger.Enabled = false
	trigger.UpdatedAt = time.Now().UTC()

	if cancel, ok := m.active[id]; ok {
		cancel()
		delete(m.active, id)
	}

	return nil
}

// startTrigger starts a trigger subscription.
func (m *Manager) startTrigger(trigger *Trigger) error {
	source, exists := m.sources[trigger.Type]
	if !exists {
		return fmt.Errorf("%w: %s", ErrInvalidTriggerType, trigger.Type)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.active[trigger.ID] = cancel

	// Create filtered handler
	handler := m.createFilteredHandler(trigger)

	go func() {
		if err := source.Subscribe(ctx, trigger.Config, handler); err != nil {
			// Log error
			_ = err
		}
	}()

	return nil
}

// createFilteredHandler wraps the handler with event filtering.
func (m *Manager) createFilteredHandler(trigger *Trigger) Handler {
	return func(ctx context.Context, event *Event) error {
		// Apply filter
		if trigger.Filter != nil {
			if !m.matchesFilter(event, trigger.Filter) {
				return nil
			}
		}

		return m.handler(ctx, event)
	}
}

// matchesFilter checks if an event matches the filter.
func (m *Manager) matchesFilter(event *Event, filter *EventFilter) bool {
	// Check event types
	if len(filter.EventTypes) > 0 {
		matched := false
		for _, t := range filter.EventTypes {
			if event.Type == t {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check headers
	if len(filter.Headers) > 0 {
		for k, v := range filter.Headers {
			if event.Metadata[k] != v {
				return false
			}
		}
	}

	return true
}

// Close shuts down all triggers.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel all active triggers
	for id, cancel := range m.active {
		cancel()
		delete(m.active, id)
	}

	// Disconnect all sources
	for _, source := range m.sources {
		if err := source.Disconnect(ctx); err != nil {
			// Log error but continue
			_ = err
		}
	}

	return nil
}

// Stats returns event manager statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ManagerStats{
		TotalTriggers:  len(m.triggers),
		ActiveTriggers: len(m.active),
		TriggersByType: make(map[TriggerType]int),
	}

	for _, t := range m.triggers {
		stats.TriggersByType[t.Type]++
		if t.Enabled {
			stats.EnabledTriggers++
		}
	}

	return stats
}

// ManagerStats contains event manager statistics.
type ManagerStats struct {
	TotalTriggers   int                   `json:"total_triggers"`
	EnabledTriggers int                   `json:"enabled_triggers"`
	ActiveTriggers  int                   `json:"active_triggers"`
	TriggersByType  map[TriggerType]int   `json:"triggers_by_type"`
}
