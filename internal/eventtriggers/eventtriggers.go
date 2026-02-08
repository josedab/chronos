// Package eventtriggers provides event-driven job triggering for Chronos.
package eventtriggers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/cloudevents"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// TriggerType represents the type of event trigger.
type TriggerType string

const (
	TriggerTypeWebhook     TriggerType = "webhook"
	TriggerTypeS3          TriggerType = "s3"
	TriggerTypeKafka       TriggerType = "kafka"
	TriggerTypeSQS         TriggerType = "sqs"
	TriggerTypeCloudEvent  TriggerType = "cloudevent"
	TriggerTypeGitHub      TriggerType = "github"
	TriggerTypeCustom      TriggerType = "custom"
)

// EventTrigger defines an event-based trigger for a job.
type EventTrigger struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        TriggerType       `json:"type"`
	JobID       string            `json:"job_id"`
	Enabled     bool              `json:"enabled"`
	Config      TriggerConfig     `json:"config"`
	Filters     []EventFilter     `json:"filters,omitempty"`
	Transform   *EventTransform   `json:"transform,omitempty"`
	RateLimit   *RateLimitConfig  `json:"rate_limit,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// TriggerConfig contains source-specific configuration.
type TriggerConfig struct {
	// Webhook
	SecretToken string `json:"secret_token,omitempty"`
	
	// S3
	Bucket      string   `json:"bucket,omitempty"`
	Prefix      string   `json:"prefix,omitempty"`
	Suffix      string   `json:"suffix,omitempty"`
	Events      []string `json:"events,omitempty"` // s3:ObjectCreated:*, etc.
	
	// Kafka
	Topic       string `json:"topic,omitempty"`
	GroupID     string `json:"group_id,omitempty"`
	Brokers     string `json:"brokers,omitempty"`
	
	// SQS
	QueueURL    string `json:"queue_url,omitempty"`
	Region      string `json:"region,omitempty"`
	
	// CloudEvent
	EventType   string `json:"event_type,omitempty"`
	Source      string `json:"source,omitempty"`
	
	// GitHub
	Repository  string   `json:"repository,omitempty"`
	EventTypes  []string `json:"event_types,omitempty"` // push, pull_request, etc.
	Branches    []string `json:"branches,omitempty"`
}

// EventFilter defines conditions for filtering events.
type EventFilter struct {
	Field    string      `json:"field"`     // JSONPath or field name
	Operator string      `json:"operator"`  // eq, ne, contains, matches, exists
	Value    interface{} `json:"value"`
}

// EventTransform defines how to transform event data into job parameters.
type EventTransform struct {
	// Parameter mappings: target param -> source path
	Mappings map[string]string `json:"mappings,omitempty"`
	// Static values to inject
	Static   map[string]interface{} `json:"static,omitempty"`
	// Template for constructing payload (Go template)
	Template string `json:"template,omitempty"`
}

// RateLimitConfig controls trigger rate limiting.
type RateLimitConfig struct {
	MaxPerMinute int           `json:"max_per_minute"`
	MaxPerHour   int           `json:"max_per_hour"`
	BurstSize    int           `json:"burst_size"`
	Window       time.Duration `json:"window,omitempty"`
}

// TriggerEvent represents an incoming event that may trigger a job.
type TriggerEvent struct {
	ID        string                 `json:"id"`
	Type      TriggerType            `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Raw       []byte                 `json:"raw,omitempty"`
}

// TriggerResult represents the result of processing a trigger.
type TriggerResult struct {
	TriggerID   string    `json:"trigger_id"`
	JobID       string    `json:"job_id"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Triggered   bool      `json:"triggered"`
	Reason      string    `json:"reason,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// JobTriggerFunc is called when a trigger fires to execute a job.
type JobTriggerFunc func(ctx context.Context, jobID string, params map[string]interface{}) (string, error)

// Manager manages event triggers.
type Manager struct {
	triggers     map[string]*EventTrigger
	jobTriggers  map[string][]*EventTrigger // jobID -> triggers
	triggerFunc  JobTriggerFunc
	rateLimiters map[string]*rateLimiter
	logger       zerolog.Logger
	mu           sync.RWMutex
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	tokens    int
	maxTokens int
	lastRefill time.Time
	refillRate float64 // tokens per second
	mu         sync.Mutex
}

// NewManager creates a new event trigger manager.
func NewManager(triggerFunc JobTriggerFunc, logger zerolog.Logger) *Manager {
	return &Manager{
		triggers:     make(map[string]*EventTrigger),
		jobTriggers:  make(map[string][]*EventTrigger),
		triggerFunc:  triggerFunc,
		rateLimiters: make(map[string]*rateLimiter),
		logger:       logger.With().Str("component", "event-triggers").Logger(),
	}
}

// RegisterTrigger registers a new event trigger.
func (m *Manager) RegisterTrigger(trigger *EventTrigger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if trigger.ID == "" {
		trigger.ID = uuid.New().String()
	}

	if trigger.JobID == "" {
		return fmt.Errorf("job_id is required")
	}

	trigger.CreatedAt = time.Now().UTC()
	trigger.UpdatedAt = trigger.CreatedAt

	m.triggers[trigger.ID] = trigger
	m.jobTriggers[trigger.JobID] = append(m.jobTriggers[trigger.JobID], trigger)

	// Set up rate limiter if configured
	if trigger.RateLimit != nil {
		m.rateLimiters[trigger.ID] = &rateLimiter{
			tokens:     trigger.RateLimit.BurstSize,
			maxTokens:  trigger.RateLimit.BurstSize,
			lastRefill: time.Now(),
			refillRate: float64(trigger.RateLimit.MaxPerMinute) / 60.0,
		}
	}

	m.logger.Info().
		Str("trigger_id", trigger.ID).
		Str("job_id", trigger.JobID).
		Str("type", string(trigger.Type)).
		Msg("Registered event trigger")

	return nil
}

// UnregisterTrigger removes an event trigger.
func (m *Manager) UnregisterTrigger(triggerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigger, ok := m.triggers[triggerID]
	if !ok {
		return fmt.Errorf("trigger not found: %s", triggerID)
	}

	delete(m.triggers, triggerID)
	delete(m.rateLimiters, triggerID)

	// Remove from job triggers
	triggers := m.jobTriggers[trigger.JobID]
	for i, t := range triggers {
		if t.ID == triggerID {
			m.jobTriggers[trigger.JobID] = append(triggers[:i], triggers[i+1:]...)
			break
		}
	}

	return nil
}

// GetTrigger returns a trigger by ID.
func (m *Manager) GetTrigger(triggerID string) (*EventTrigger, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	trigger, ok := m.triggers[triggerID]
	return trigger, ok
}

// ListTriggers returns all triggers.
func (m *Manager) ListTriggers() []*EventTrigger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	triggers := make([]*EventTrigger, 0, len(m.triggers))
	for _, t := range m.triggers {
		triggers = append(triggers, t)
	}
	return triggers
}

// GetTriggersForJob returns all triggers for a specific job.
func (m *Manager) GetTriggersForJob(jobID string) []*EventTrigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobTriggers[jobID]
}

// ProcessEvent processes an incoming event and triggers matching jobs.
func (m *Manager) ProcessEvent(ctx context.Context, event *TriggerEvent) ([]*TriggerResult, error) {
	m.mu.RLock()
	triggers := make([]*EventTrigger, 0, len(m.triggers))
	for _, t := range m.triggers {
		triggers = append(triggers, t)
	}
	m.mu.RUnlock()

	results := make([]*TriggerResult, 0)

	for _, trigger := range triggers {
		if !trigger.Enabled {
			continue
		}

		// Check if trigger type matches
		if !m.matchesTriggerType(trigger, event) {
			continue
		}

		// Apply filters
		if !m.passesFilters(trigger, event) {
			results = append(results, &TriggerResult{
				TriggerID: trigger.ID,
				JobID:     trigger.JobID,
				Triggered: false,
				Reason:    "filtered",
				Timestamp: time.Now().UTC(),
			})
			continue
		}

		// Check rate limit
		if !m.checkRateLimit(trigger.ID) {
			results = append(results, &TriggerResult{
				TriggerID: trigger.ID,
				JobID:     trigger.JobID,
				Triggered: false,
				Reason:    "rate_limited",
				Timestamp: time.Now().UTC(),
			})
			continue
		}

		// Transform event data to job parameters
		params := m.transformEvent(trigger, event)

		// Trigger the job
		execID, err := m.triggerFunc(ctx, trigger.JobID, params)
		if err != nil {
			m.logger.Error().
				Err(err).
				Str("trigger_id", trigger.ID).
				Str("job_id", trigger.JobID).
				Msg("Failed to trigger job")
			results = append(results, &TriggerResult{
				TriggerID: trigger.ID,
				JobID:     trigger.JobID,
				Triggered: false,
				Reason:    err.Error(),
				Timestamp: time.Now().UTC(),
			})
			continue
		}

		m.logger.Info().
			Str("trigger_id", trigger.ID).
			Str("job_id", trigger.JobID).
			Str("execution_id", execID).
			Msg("Triggered job from event")

		results = append(results, &TriggerResult{
			TriggerID:   trigger.ID,
			JobID:       trigger.JobID,
			ExecutionID: execID,
			Triggered:   true,
			Timestamp:   time.Now().UTC(),
		})
	}

	return results, nil
}

// ProcessCloudEvent processes a CloudEvent and triggers matching jobs.
func (m *Manager) ProcessCloudEvent(ctx context.Context, ce *cloudevents.Event) ([]*TriggerResult, error) {
	// Convert CloudEvent to TriggerEvent
	data := make(map[string]interface{})
	if ce.Data != nil {
		switch d := ce.Data.(type) {
		case map[string]interface{}:
			data = d
		case []byte:
			json.Unmarshal(d, &data)
		default:
			jsonData, _ := json.Marshal(d)
			json.Unmarshal(jsonData, &data)
		}
	}

	event := &TriggerEvent{
		ID:        ce.ID,
		Type:      TriggerTypeCloudEvent,
		Source:    ce.Source,
		Timestamp: ce.Time,
		Data:      data,
	}

	// Add CloudEvent metadata to data for filtering
	event.Data["_ce_type"] = ce.Type
	event.Data["_ce_source"] = ce.Source
	event.Data["_ce_subject"] = ce.Subject

	return m.ProcessEvent(ctx, event)
}

func (m *Manager) matchesTriggerType(trigger *EventTrigger, event *TriggerEvent) bool {
	// CloudEvent triggers match based on event type/source
	if trigger.Type == TriggerTypeCloudEvent {
		if trigger.Config.EventType != "" {
			ceType, _ := event.Data["_ce_type"].(string)
			if !m.matchesPattern(trigger.Config.EventType, ceType) {
				return false
			}
		}
		if trigger.Config.Source != "" {
			ceSource, _ := event.Data["_ce_source"].(string)
			if !m.matchesPattern(trigger.Config.Source, ceSource) {
				return false
			}
		}
		return true
	}

	// S3 triggers
	if trigger.Type == TriggerTypeS3 && event.Type == TriggerTypeS3 {
		bucket, _ := event.Data["bucket"].(string)
		key, _ := event.Data["key"].(string)
		eventName, _ := event.Data["event_name"].(string)

		if trigger.Config.Bucket != "" && trigger.Config.Bucket != bucket {
			return false
		}
		if trigger.Config.Prefix != "" && !strings.HasPrefix(key, trigger.Config.Prefix) {
			return false
		}
		if trigger.Config.Suffix != "" && !strings.HasSuffix(key, trigger.Config.Suffix) {
			return false
		}
		if len(trigger.Config.Events) > 0 {
			matched := false
			for _, e := range trigger.Config.Events {
				if m.matchesPattern(e, eventName) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
		return true
	}

	// Kafka triggers
	if trigger.Type == TriggerTypeKafka && event.Type == TriggerTypeKafka {
		topic, _ := event.Data["topic"].(string)
		return trigger.Config.Topic == "" || trigger.Config.Topic == topic
	}

	// GitHub triggers
	if trigger.Type == TriggerTypeGitHub && event.Type == TriggerTypeGitHub {
		repo, _ := event.Data["repository"].(string)
		eventType, _ := event.Data["event_type"].(string)
		branch, _ := event.Data["branch"].(string)

		if trigger.Config.Repository != "" && trigger.Config.Repository != repo {
			return false
		}
		if len(trigger.Config.EventTypes) > 0 {
			matched := false
			for _, et := range trigger.Config.EventTypes {
				if et == eventType {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
		if len(trigger.Config.Branches) > 0 && branch != "" {
			matched := false
			for _, b := range trigger.Config.Branches {
				if m.matchesPattern(b, branch) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
		return true
	}

	// Webhook triggers match any webhook event
	if trigger.Type == TriggerTypeWebhook && event.Type == TriggerTypeWebhook {
		return true
	}

	return trigger.Type == event.Type
}

func (m *Manager) matchesPattern(pattern, value string) bool {
	// Support glob-style wildcards
	if strings.Contains(pattern, "*") {
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		re, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			return pattern == value
		}
		return re.MatchString(value)
	}
	return pattern == value
}

func (m *Manager) passesFilters(trigger *EventTrigger, event *TriggerEvent) bool {
	if len(trigger.Filters) == 0 {
		return true
	}

	for _, filter := range trigger.Filters {
		if !m.evaluateFilter(filter, event.Data) {
			return false
		}
	}
	return true
}

func (m *Manager) evaluateFilter(filter EventFilter, data map[string]interface{}) bool {
	value := m.getFieldValue(filter.Field, data)

	switch filter.Operator {
	case "eq":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", filter.Value)
	case "ne":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", filter.Value)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", value), fmt.Sprintf("%v", filter.Value))
	case "matches":
		re, err := regexp.Compile(fmt.Sprintf("%v", filter.Value))
		if err != nil {
			return false
		}
		return re.MatchString(fmt.Sprintf("%v", value))
	case "exists":
		return value != nil
	case "gt":
		return compareNumeric(value, filter.Value) > 0
	case "lt":
		return compareNumeric(value, filter.Value) < 0
	case "gte":
		return compareNumeric(value, filter.Value) >= 0
	case "lte":
		return compareNumeric(value, filter.Value) <= 0
	default:
		return false
	}
}

func compareNumeric(a, b interface{}) int {
	aFloat := toFloat(a)
	bFloat := toFloat(b)
	if aFloat < bFloat {
		return -1
	} else if aFloat > bFloat {
		return 1
	}
	return 0
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case float32:
		return float64(n)
	default:
		return 0
	}
}

func (m *Manager) getFieldValue(field string, data map[string]interface{}) interface{} {
	// Support dot notation for nested fields
	parts := strings.Split(field, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return nil
		}
	}
	return current
}

func (m *Manager) checkRateLimit(triggerID string) bool {
	m.mu.RLock()
	rl, ok := m.rateLimiters[triggerID]
	m.mu.RUnlock()

	if !ok {
		return true // No rate limit configured
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	newTokens := int(elapsed * rl.refillRate)
	if newTokens > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+newTokens)
		rl.lastRefill = now
	}

	// Check if we have tokens
	if rl.tokens <= 0 {
		return false
	}

	rl.tokens--
	return true
}

func (m *Manager) transformEvent(trigger *EventTrigger, event *TriggerEvent) map[string]interface{} {
	params := make(map[string]interface{})

	// Copy event data as base
	params["_event"] = event.Data
	params["_event_id"] = event.ID
	params["_event_type"] = event.Type
	params["_event_source"] = event.Source
	params["_event_timestamp"] = event.Timestamp

	if trigger.Transform == nil {
		return params
	}

	// Apply static values
	for k, v := range trigger.Transform.Static {
		params[k] = v
	}

	// Apply mappings
	for target, source := range trigger.Transform.Mappings {
		value := m.getFieldValue(source, event.Data)
		if value != nil {
			params[target] = value
		}
	}

	return params
}

// EnableTrigger enables a trigger.
func (m *Manager) EnableTrigger(triggerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigger, ok := m.triggers[triggerID]
	if !ok {
		return fmt.Errorf("trigger not found: %s", triggerID)
	}

	trigger.Enabled = true
	trigger.UpdatedAt = time.Now().UTC()
	return nil
}

// DisableTrigger disables a trigger.
func (m *Manager) DisableTrigger(triggerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	trigger, ok := m.triggers[triggerID]
	if !ok {
		return fmt.Errorf("trigger not found: %s", triggerID)
	}

	trigger.Enabled = false
	trigger.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateTrigger updates an existing trigger.
func (m *Manager) UpdateTrigger(triggerID string, update *EventTrigger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.triggers[triggerID]
	if !ok {
		return fmt.Errorf("trigger not found: %s", triggerID)
	}

	// Update fields
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.Description != "" {
		existing.Description = update.Description
	}
	existing.Enabled = update.Enabled
	if update.Config.Topic != "" || update.Config.Bucket != "" || update.Config.QueueURL != "" {
		existing.Config = update.Config
	}
	if len(update.Filters) > 0 {
		existing.Filters = update.Filters
	}
	if update.Transform != nil {
		existing.Transform = update.Transform
	}
	if update.RateLimit != nil {
		existing.RateLimit = update.RateLimit
		// Update rate limiter
		m.rateLimiters[triggerID] = &rateLimiter{
			tokens:     update.RateLimit.BurstSize,
			maxTokens:  update.RateLimit.BurstSize,
			lastRefill: time.Now(),
			refillRate: float64(update.RateLimit.MaxPerMinute) / 60.0,
		}
	}

	existing.UpdatedAt = time.Now().UTC()
	return nil
}

// CreateS3Trigger is a helper to create an S3 event trigger.
func CreateS3Trigger(name, jobID, bucket, prefix, suffix string, events []string) *EventTrigger {
	if len(events) == 0 {
		events = []string{"s3:ObjectCreated:*"}
	}
	return &EventTrigger{
		ID:      uuid.New().String(),
		Name:    name,
		Type:    TriggerTypeS3,
		JobID:   jobID,
		Enabled: true,
		Config: TriggerConfig{
			Bucket: bucket,
			Prefix: prefix,
			Suffix: suffix,
			Events: events,
		},
	}
}

// CreateKafkaTrigger is a helper to create a Kafka event trigger.
func CreateKafkaTrigger(name, jobID, topic, groupID, brokers string) *EventTrigger {
	return &EventTrigger{
		ID:      uuid.New().String(),
		Name:    name,
		Type:    TriggerTypeKafka,
		JobID:   jobID,
		Enabled: true,
		Config: TriggerConfig{
			Topic:   topic,
			GroupID: groupID,
			Brokers: brokers,
		},
	}
}

// CreateGitHubTrigger is a helper to create a GitHub event trigger.
func CreateGitHubTrigger(name, jobID, repository string, eventTypes, branches []string) *EventTrigger {
	return &EventTrigger{
		ID:      uuid.New().String(),
		Name:    name,
		Type:    TriggerTypeGitHub,
		JobID:   jobID,
		Enabled: true,
		Config: TriggerConfig{
			Repository: repository,
			EventTypes: eventTypes,
			Branches:   branches,
		},
	}
}

// CreateWebhookTrigger is a helper to create a webhook trigger.
func CreateWebhookTrigger(name, jobID, secretToken string) *EventTrigger {
	return &EventTrigger{
		ID:      uuid.New().String(),
		Name:    name,
		Type:    TriggerTypeWebhook,
		JobID:   jobID,
		Enabled: true,
		Config: TriggerConfig{
			SecretToken: secretToken,
		},
	}
}

// NewTriggerEvent creates a TriggerEvent from raw data.
func NewTriggerEvent(triggerType TriggerType, source string, data map[string]interface{}) *TriggerEvent {
	return &TriggerEvent{
		ID:        uuid.New().String(),
		Type:      triggerType,
		Source:    source,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
}

// S3EventFromNotification parses an S3 notification into a TriggerEvent.
func S3EventFromNotification(notification map[string]interface{}) *TriggerEvent {
	records, _ := notification["Records"].([]interface{})
	if len(records) == 0 {
		return nil
	}

	record, _ := records[0].(map[string]interface{})
	s3Data, _ := record["s3"].(map[string]interface{})
	bucket, _ := s3Data["bucket"].(map[string]interface{})
	object, _ := s3Data["object"].(map[string]interface{})

	return &TriggerEvent{
		ID:        uuid.New().String(),
		Type:      TriggerTypeS3,
		Source:    "aws:s3",
		Timestamp: time.Now().UTC(),
		Data: map[string]interface{}{
			"bucket":     bucket["name"],
			"key":        object["key"],
			"size":       object["size"],
			"event_name": record["eventName"],
			"region":     record["awsRegion"],
		},
	}
}

// KafkaEventFromMessage parses a Kafka message into a TriggerEvent.
func KafkaEventFromMessage(topic string, key, value []byte, headers map[string]string) *TriggerEvent {
	var data map[string]interface{}
	if err := json.Unmarshal(value, &data); err != nil {
		data = map[string]interface{}{"raw": string(value)}
	}

	data["topic"] = topic
	data["key"] = string(key)

	return &TriggerEvent{
		ID:        uuid.New().String(),
		Type:      TriggerTypeKafka,
		Source:    "kafka:" + topic,
		Timestamp: time.Now().UTC(),
		Data:      data,
		Headers:   headers,
	}
}

// GitHubEventFromWebhook parses a GitHub webhook into a TriggerEvent.
func GitHubEventFromWebhook(eventType string, payload map[string]interface{}) *TriggerEvent {
	data := map[string]interface{}{
		"event_type": eventType,
	}

	// Extract common fields
	if repo, ok := payload["repository"].(map[string]interface{}); ok {
		data["repository"] = repo["full_name"]
	}
	if sender, ok := payload["sender"].(map[string]interface{}); ok {
		data["sender"] = sender["login"]
	}

	// Extract branch for push events
	if ref, ok := payload["ref"].(string); ok {
		if strings.HasPrefix(ref, "refs/heads/") {
			data["branch"] = strings.TrimPrefix(ref, "refs/heads/")
		}
	}

	// Extract PR number for pull_request events
	if pr, ok := payload["pull_request"].(map[string]interface{}); ok {
		data["pr_number"] = pr["number"]
		data["pr_title"] = pr["title"]
		if head, ok := pr["head"].(map[string]interface{}); ok {
			data["branch"] = head["ref"]
		}
	}

	// Add full payload for custom filtering
	data["payload"] = payload

	return &TriggerEvent{
		ID:        uuid.New().String(),
		Type:      TriggerTypeGitHub,
		Source:    "github.com",
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
}

// TriggerStats provides statistics about trigger activity.
type TriggerStats struct {
	TotalTriggers  int            `json:"total_triggers"`
	EnabledCount   int            `json:"enabled_count"`
	DisabledCount  int            `json:"disabled_count"`
	ByType         map[string]int `json:"by_type"`
	RecentActivity int            `json:"recent_activity_24h"`
}

// GetStats returns statistics about registered triggers.
func (m *Manager) GetStats() *TriggerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &TriggerStats{
		TotalTriggers: len(m.triggers),
		ByType:        make(map[string]int),
	}

	for _, t := range m.triggers {
		if t.Enabled {
			stats.EnabledCount++
		} else {
			stats.DisabledCount++
		}
		stats.ByType[string(t.Type)]++
	}

	return stats
}
