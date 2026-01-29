// Package eventmesh provides cloud event mesh integration for Chronos.
package eventmesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventMeshConfig configures the event mesh.
type EventMeshConfig struct {
	EventBridge *EventBridgeConfig `json:"event_bridge,omitempty"`
	PubSub      *PubSubConfig      `json:"pub_sub,omitempty"`
	EventGrid   *EventGridConfig   `json:"event_grid,omitempty"`
}

// EventBridgeConfig configures AWS EventBridge.
type EventBridgeConfig struct {
	Region          string `json:"region"`
	EventBusName    string `json:"event_bus_name"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	RoleARN         string `json:"role_arn,omitempty"`
}

// PubSubConfig configures Google Cloud Pub/Sub.
type PubSubConfig struct {
	ProjectID         string `json:"project_id"`
	CredentialsJSON   string `json:"credentials_json,omitempty"`
	TopicID           string `json:"topic_id"`
	SubscriptionID    string `json:"subscription_id,omitempty"`
}

// EventGridConfig configures Azure Event Grid.
type EventGridConfig struct {
	TopicEndpoint string `json:"topic_endpoint"`
	TopicKey      string `json:"topic_key"`
	SubscriptionID string `json:"subscription_id,omitempty"`
	ResourceGroup string `json:"resource_group,omitempty"`
}

// ChronosEvent represents a Chronos event.
type ChronosEvent struct {
	ID          string                 `json:"id"`
	Type        EventType              `json:"type"`
	Source      string                 `json:"source"`
	Subject     string                 `json:"subject,omitempty"`
	Time        time.Time              `json:"time"`
	Data        map[string]interface{} `json:"data"`
	DataVersion string                 `json:"data_version"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// EventType represents Chronos event types.
type EventType string

const (
	EventJobCreated    EventType = "chronos.job.created"
	EventJobUpdated    EventType = "chronos.job.updated"
	EventJobDeleted    EventType = "chronos.job.deleted"
	EventJobTriggered  EventType = "chronos.job.triggered"
	EventJobSucceeded  EventType = "chronos.job.succeeded"
	EventJobFailed     EventType = "chronos.job.failed"
	EventJobRetrying   EventType = "chronos.job.retrying"
	EventJobTimeout    EventType = "chronos.job.timeout"
	EventWorkflowStart EventType = "chronos.workflow.started"
	EventWorkflowEnd   EventType = "chronos.workflow.completed"
	EventClusterJoin   EventType = "chronos.cluster.node_joined"
	EventClusterLeave  EventType = "chronos.cluster.node_left"
	EventAlertFired    EventType = "chronos.alert.fired"
)

// EventMesh manages event publishing and subscription.
type EventMesh struct {
	config      EventMeshConfig
	publishers  []EventPublisher
	subscribers []EventSubscriber
	handlers    map[EventType][]EventHandler
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NewEventMesh creates a new event mesh.
func NewEventMesh(config EventMeshConfig) (*EventMesh, error) {
	mesh := &EventMesh{
		config:     config,
		publishers: make([]EventPublisher, 0),
		subscribers: make([]EventSubscriber, 0),
		handlers:   make(map[EventType][]EventHandler),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Initialize publishers based on config
	if config.EventBridge != nil {
		pub, err := NewEventBridgePublisher(config.EventBridge)
		if err != nil {
			return nil, fmt.Errorf("failed to create EventBridge publisher: %w", err)
		}
		mesh.publishers = append(mesh.publishers, pub)
	}

	if config.PubSub != nil {
		pub, err := NewPubSubPublisher(config.PubSub)
		if err != nil {
			return nil, fmt.Errorf("failed to create Pub/Sub publisher: %w", err)
		}
		mesh.publishers = append(mesh.publishers, pub)
	}

	if config.EventGrid != nil {
		pub, err := NewEventGridPublisher(config.EventGrid)
		if err != nil {
			return nil, fmt.Errorf("failed to create Event Grid publisher: %w", err)
		}
		mesh.publishers = append(mesh.publishers, pub)
	}

	return mesh, nil
}

// Publish publishes an event to all configured providers.
func (m *EventMesh) Publish(ctx context.Context, event *ChronosEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	if event.Source == "" {
		event.Source = "chronos"
	}
	if event.DataVersion == "" {
		event.DataVersion = "1.0"
	}

	var errs []error
	for _, pub := range m.publishers {
		if err := pub.Publish(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", pub.Name(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("publish errors: %v", errs)
	}

	return nil
}

// Subscribe registers an event handler.
func (m *EventMesh) Subscribe(eventType EventType, handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers[eventType] = append(m.handlers[eventType], handler)
}

// HandleIncoming processes incoming events from subscriptions.
func (m *EventMesh) HandleIncoming(ctx context.Context, event *ChronosEvent) error {
	m.mu.RLock()
	handlers, ok := m.handlers[event.Type]
	m.mu.RUnlock()

	if !ok || len(handlers) == 0 {
		return nil // No handlers registered
	}

	var errs []error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("handler errors: %v", errs)
	}

	return nil
}

// StartSubscribers starts all event subscribers.
func (m *EventMesh) StartSubscribers(ctx context.Context) error {
	for _, sub := range m.subscribers {
		go func(s EventSubscriber) {
			s.Start(ctx, m.HandleIncoming)
		}(sub)
	}
	return nil
}

// EventPublisher publishes events to external systems.
type EventPublisher interface {
	Name() string
	Publish(ctx context.Context, event *ChronosEvent) error
	Close() error
}

// EventSubscriber subscribes to events from external systems.
type EventSubscriber interface {
	Name() string
	Start(ctx context.Context, handler func(context.Context, *ChronosEvent) error) error
	Stop() error
}

// EventHandler handles incoming events.
type EventHandler func(ctx context.Context, event *ChronosEvent) error

// EventBridgePublisher publishes to AWS EventBridge.
type EventBridgePublisher struct {
	config     *EventBridgeConfig
	httpClient *http.Client
}

// NewEventBridgePublisher creates an EventBridge publisher.
func NewEventBridgePublisher(config *EventBridgeConfig) (*EventBridgePublisher, error) {
	return &EventBridgePublisher{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Name returns the publisher name.
func (p *EventBridgePublisher) Name() string {
	return "aws-eventbridge"
}

// Publish publishes an event to EventBridge.
func (p *EventBridgePublisher) Publish(ctx context.Context, event *ChronosEvent) error {
	// Convert to EventBridge event format
	ebEvent := map[string]interface{}{
		"Source":       event.Source,
		"DetailType":   string(event.Type),
		"Detail":       event.Data,
		"EventBusName": p.config.EventBusName,
		"Time":         event.Time.Format(time.RFC3339),
	}

	entries := []map[string]interface{}{ebEvent}
	payload := map[string]interface{}{
		"Entries": entries,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://events.%s.amazonaws.com", p.config.Region)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSEvents.PutEvents")
	// In production, add AWS SigV4 signing here

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("EventBridge error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Close closes the publisher.
func (p *EventBridgePublisher) Close() error {
	return nil
}

// EventBridgeSubscriber subscribes to EventBridge events.
type EventBridgeSubscriber struct {
	config      *EventBridgeConfig
	ruleName    string
	sqsQueueURL string
	stopCh      chan struct{}
}

// NewEventBridgeSubscriber creates an EventBridge subscriber.
func NewEventBridgeSubscriber(config *EventBridgeConfig, ruleName, sqsQueueURL string) *EventBridgeSubscriber {
	return &EventBridgeSubscriber{
		config:      config,
		ruleName:    ruleName,
		sqsQueueURL: sqsQueueURL,
		stopCh:      make(chan struct{}),
	}
}

// Name returns the subscriber name.
func (s *EventBridgeSubscriber) Name() string {
	return "aws-eventbridge"
}

// Start starts polling SQS for EventBridge events.
func (s *EventBridgeSubscriber) Start(ctx context.Context, handler func(context.Context, *ChronosEvent) error) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-ticker.C:
			// Poll SQS and process messages
			// In production: use AWS SDK to receive and delete messages
		}
	}
}

// Stop stops the subscriber.
func (s *EventBridgeSubscriber) Stop() error {
	close(s.stopCh)
	return nil
}

// PubSubPublisher publishes to Google Cloud Pub/Sub.
type PubSubPublisher struct {
	config     *PubSubConfig
	httpClient *http.Client
}

// NewPubSubPublisher creates a Pub/Sub publisher.
func NewPubSubPublisher(config *PubSubConfig) (*PubSubPublisher, error) {
	return &PubSubPublisher{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Name returns the publisher name.
func (p *PubSubPublisher) Name() string {
	return "gcp-pubsub"
}

// Publish publishes an event to Pub/Sub.
func (p *PubSubPublisher) Publish(ctx context.Context, event *ChronosEvent) error {
	// Convert to Pub/Sub message format
	messageData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	message := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"data": messageData, // Should be base64 encoded
				"attributes": map[string]string{
					"eventType":   string(event.Type),
					"eventId":     event.ID,
					"source":      event.Source,
					"dataVersion": event.DataVersion,
				},
			},
		},
	}

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics/%s:publish",
		p.config.ProjectID, p.config.TopicID)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	// In production: add OAuth2 token here

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Pub/Sub error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Close closes the publisher.
func (p *PubSubPublisher) Close() error {
	return nil
}

// PubSubSubscriber subscribes to Pub/Sub messages.
type PubSubSubscriber struct {
	config *PubSubConfig
	stopCh chan struct{}
}

// NewPubSubSubscriber creates a Pub/Sub subscriber.
func NewPubSubSubscriber(config *PubSubConfig) *PubSubSubscriber {
	return &PubSubSubscriber{
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Name returns the subscriber name.
func (s *PubSubSubscriber) Name() string {
	return "gcp-pubsub"
}

// Start starts pulling messages from Pub/Sub.
func (s *PubSubSubscriber) Start(ctx context.Context, handler func(context.Context, *ChronosEvent) error) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopCh:
			return nil
		case <-ticker.C:
			// Pull messages from subscription
			// In production: use GCP Pub/Sub client library
		}
	}
}

// Stop stops the subscriber.
func (s *PubSubSubscriber) Stop() error {
	close(s.stopCh)
	return nil
}

// EventGridPublisher publishes to Azure Event Grid.
type EventGridPublisher struct {
	config     *EventGridConfig
	httpClient *http.Client
}

// NewEventGridPublisher creates an Event Grid publisher.
func NewEventGridPublisher(config *EventGridConfig) (*EventGridPublisher, error) {
	return &EventGridPublisher{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Name returns the publisher name.
func (p *EventGridPublisher) Name() string {
	return "azure-eventgrid"
}

// Publish publishes an event to Event Grid.
func (p *EventGridPublisher) Publish(ctx context.Context, event *ChronosEvent) error {
	// Convert to Event Grid event format
	egEvent := []map[string]interface{}{
		{
			"id":          event.ID,
			"eventType":   string(event.Type),
			"subject":     event.Subject,
			"eventTime":   event.Time.Format(time.RFC3339),
			"data":        event.Data,
			"dataVersion": event.DataVersion,
		},
	}

	body, err := json.Marshal(egEvent)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.TopicEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("aeg-sas-key", p.config.TopicKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Event Grid error: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// Close closes the publisher.
func (p *EventGridPublisher) Close() error {
	return nil
}

// EventGridSubscriber subscribes via webhook.
type EventGridSubscriber struct {
	config  *EventGridConfig
	handler func(context.Context, *ChronosEvent) error
	server  *http.Server
}

// NewEventGridSubscriber creates an Event Grid subscriber.
func NewEventGridSubscriber(config *EventGridConfig, listenAddr string) *EventGridSubscriber {
	sub := &EventGridSubscriber{config: config}
	
	mux := http.NewServeMux()
	mux.HandleFunc("/events", sub.handleWebhook)
	
	sub.server = &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}
	
	return sub
}

// Name returns the subscriber name.
func (s *EventGridSubscriber) Name() string {
	return "azure-eventgrid"
}

// Start starts the webhook server.
func (s *EventGridSubscriber) Start(ctx context.Context, handler func(context.Context, *ChronosEvent) error) error {
	s.handler = handler
	
	go func() {
		<-ctx.Done()
		s.server.Shutdown(context.Background())
	}()
	
	return s.server.ListenAndServe()
}

// Stop stops the subscriber.
func (s *EventGridSubscriber) Stop() error {
	return s.server.Close()
}

func (s *EventGridSubscriber) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Handle Event Grid validation
	if r.Header.Get("aeg-event-type") == "SubscriptionValidation" {
		var events []struct {
			Data struct {
				ValidationCode string `json:"validationCode"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&events); err == nil && len(events) > 0 {
			response := map[string]string{
				"validationResponse": events[0].Data.ValidationCode,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Process events
	var events []struct {
		ID        string                 `json:"id"`
		EventType string                 `json:"eventType"`
		Subject   string                 `json:"subject"`
		EventTime time.Time              `json:"eventTime"`
		Data      map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, e := range events {
		chronosEvent := &ChronosEvent{
			ID:      e.ID,
			Type:    EventType(e.EventType),
			Subject: e.Subject,
			Time:    e.EventTime,
			Data:    e.Data,
		}

		if s.handler != nil {
			s.handler(r.Context(), chronosEvent)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// TriggerRule defines when to trigger a job from an event.
type TriggerRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	EventTypes  []EventType       `json:"event_types"`
	Filter      *EventFilter      `json:"filter,omitempty"`
	JobID       string            `json:"job_id"`
	JobInputs   map[string]string `json:"job_inputs,omitempty"` // Event data mapping
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// EventFilter filters events.
type EventFilter struct {
	Source      string            `json:"source,omitempty"`
	Subject     string            `json:"subject,omitempty"`
	DataMatches map[string]string `json:"data_matches,omitempty"`
}

// TriggerManager manages event-to-job triggers.
type TriggerManager struct {
	rules    map[string]*TriggerRule
	mesh     *EventMesh
	jobFunc  func(ctx context.Context, jobID string, inputs map[string]string) error
	mu       sync.RWMutex
}

// NewTriggerManager creates a trigger manager.
func NewTriggerManager(mesh *EventMesh, jobFunc func(context.Context, string, map[string]string) error) *TriggerManager {
	tm := &TriggerManager{
		rules:   make(map[string]*TriggerRule),
		mesh:    mesh,
		jobFunc: jobFunc,
	}

	// Subscribe to all event types
	for _, eventType := range []EventType{
		EventJobSucceeded, EventJobFailed, EventWorkflowEnd,
		EventClusterJoin, EventClusterLeave, EventAlertFired,
	} {
		mesh.Subscribe(eventType, tm.handleEvent)
	}

	return tm
}

// AddRule adds a trigger rule.
func (m *TriggerManager) AddRule(rule *TriggerRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule
	return nil
}

// RemoveRule removes a trigger rule.
func (m *TriggerManager) RemoveRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.rules, ruleID)
	return nil
}

// ListRules returns all trigger rules.
func (m *TriggerManager) ListRules() []*TriggerRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*TriggerRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

func (m *TriggerManager) handleEvent(ctx context.Context, event *ChronosEvent) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		// Check event type match
		typeMatch := false
		for _, et := range rule.EventTypes {
			if et == event.Type {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			continue
		}

		// Check filter
		if rule.Filter != nil {
			if rule.Filter.Source != "" && event.Source != rule.Filter.Source {
				continue
			}
			if rule.Filter.Subject != "" && event.Subject != rule.Filter.Subject {
				continue
			}
			// Check data matches
			if len(rule.Filter.DataMatches) > 0 {
				match := true
				for key, value := range rule.Filter.DataMatches {
					if eventValue, ok := event.Data[key]; !ok || fmt.Sprintf("%v", eventValue) != value {
						match = false
						break
					}
				}
				if !match {
					continue
				}
			}
		}

		// Build job inputs from event data
		inputs := make(map[string]string)
		for inputKey, eventPath := range rule.JobInputs {
			if value, ok := event.Data[eventPath]; ok {
				inputs[inputKey] = fmt.Sprintf("%v", value)
			}
		}

		// Trigger the job
		if m.jobFunc != nil {
			go m.jobFunc(ctx, rule.JobID, inputs)
		}
	}

	return nil
}
