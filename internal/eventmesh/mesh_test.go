package eventmesh

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewEventMesh(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		mesh, err := NewEventMesh(EventMeshConfig{})
		if err != nil {
			t.Fatalf("NewEventMesh failed: %v", err)
		}
		if mesh == nil {
			t.Fatal("mesh should not be nil")
		}
		if len(mesh.publishers) != 0 {
			t.Errorf("expected 0 publishers, got %d", len(mesh.publishers))
		}
	})

	t.Run("with EventBridge config", func(t *testing.T) {
		config := EventMeshConfig{
			EventBridge: &EventBridgeConfig{
				Region:       "us-east-1",
				EventBusName: "default",
			},
		}
		mesh, err := NewEventMesh(config)
		if err != nil {
			t.Fatalf("NewEventMesh failed: %v", err)
		}
		if len(mesh.publishers) != 1 {
			t.Errorf("expected 1 publisher, got %d", len(mesh.publishers))
		}
	})

	t.Run("with PubSub config", func(t *testing.T) {
		config := EventMeshConfig{
			PubSub: &PubSubConfig{
				ProjectID: "test-project",
				TopicID:   "test-topic",
			},
		}
		mesh, err := NewEventMesh(config)
		if err != nil {
			t.Fatalf("NewEventMesh failed: %v", err)
		}
		if len(mesh.publishers) != 1 {
			t.Errorf("expected 1 publisher, got %d", len(mesh.publishers))
		}
	})

	t.Run("with EventGrid config", func(t *testing.T) {
		config := EventMeshConfig{
			EventGrid: &EventGridConfig{
				TopicEndpoint: "https://test.eventgrid.azure.net",
				TopicKey:      "key",
			},
		}
		mesh, err := NewEventMesh(config)
		if err != nil {
			t.Fatalf("NewEventMesh failed: %v", err)
		}
		if len(mesh.publishers) != 1 {
			t.Errorf("expected 1 publisher, got %d", len(mesh.publishers))
		}
	})
}

func TestEventMesh_Subscribe(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	handler := func(ctx context.Context, event *ChronosEvent) error {
		return nil
	}

	mesh.Subscribe(EventJobCreated, handler)

	if len(mesh.handlers[EventJobCreated]) != 1 {
		t.Error("handler not registered")
	}
}

func TestEventMesh_HandleIncoming(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	var receivedEvent *ChronosEvent
	handler := func(ctx context.Context, event *ChronosEvent) error {
		receivedEvent = event
		return nil
	}

	mesh.Subscribe(EventJobCreated, handler)

	event := &ChronosEvent{
		ID:   "test-1",
		Type: EventJobCreated,
		Data: map[string]interface{}{"job_id": "job-1"},
	}

	err := mesh.HandleIncoming(context.Background(), event)
	if err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	if receivedEvent == nil {
		t.Error("handler not called")
	}
	if receivedEvent.ID != "test-1" {
		t.Errorf("event ID = %s, want test-1", receivedEvent.ID)
	}
}

func TestEventMesh_HandleIncoming_NoHandlers(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	event := &ChronosEvent{
		ID:   "test-1",
		Type: EventJobCreated,
	}

	err := mesh.HandleIncoming(context.Background(), event)
	if err != nil {
		t.Errorf("HandleIncoming should not error with no handlers: %v", err)
	}
}

func TestChronosEvent_DefaultValues(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	event := &ChronosEvent{
		Type: EventJobCreated,
		Data: map[string]interface{}{"test": "data"},
	}

	// Use publish to fill in defaults (won't actually publish since no publishers)
	mesh.Publish(context.Background(), event)

	if event.ID == "" {
		t.Error("ID should be set")
	}
	if event.Time.IsZero() {
		t.Error("Time should be set")
	}
	if event.Source == "" {
		t.Error("Source should be set")
	}
	if event.DataVersion == "" {
		t.Error("DataVersion should be set")
	}
}

func TestEventTypeConstants(t *testing.T) {
	types := map[EventType]string{
		EventJobCreated:    "chronos.job.created",
		EventJobUpdated:    "chronos.job.updated",
		EventJobDeleted:    "chronos.job.deleted",
		EventJobTriggered:  "chronos.job.triggered",
		EventJobSucceeded:  "chronos.job.succeeded",
		EventJobFailed:     "chronos.job.failed",
		EventJobRetrying:   "chronos.job.retrying",
		EventJobTimeout:    "chronos.job.timeout",
		EventWorkflowStart: "chronos.workflow.started",
		EventWorkflowEnd:   "chronos.workflow.completed",
		EventClusterJoin:   "chronos.cluster.node_joined",
		EventClusterLeave:  "chronos.cluster.node_left",
		EventAlertFired:    "chronos.alert.fired",
	}

	for eventType, expected := range types {
		if string(eventType) != expected {
			t.Errorf("EventType %s = %s, want %s", eventType, eventType, expected)
		}
	}
}

func TestEventBridgePublisher(t *testing.T) {
	config := &EventBridgeConfig{
		Region:       "us-east-1",
		EventBusName: "default",
	}

	pub, err := NewEventBridgePublisher(config)
	if err != nil {
		t.Fatalf("NewEventBridgePublisher failed: %v", err)
	}

	if pub.Name() != "aws-eventbridge" {
		t.Errorf("Name = %s, want aws-eventbridge", pub.Name())
	}

	if err := pub.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestEventBridgeSubscriber(t *testing.T) {
	config := &EventBridgeConfig{
		Region:       "us-east-1",
		EventBusName: "default",
	}

	sub := NewEventBridgeSubscriber(config, "test-rule", "https://sqs.us-east-1.amazonaws.com/123/queue")

	if sub.Name() != "aws-eventbridge" {
		t.Errorf("Name = %s, want aws-eventbridge", sub.Name())
	}

	// Test stop
	go func() {
		time.Sleep(100 * time.Millisecond)
		sub.Stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	sub.Start(ctx, func(ctx context.Context, event *ChronosEvent) error {
		return nil
	})
}

func TestPubSubPublisher(t *testing.T) {
	config := &PubSubConfig{
		ProjectID: "test-project",
		TopicID:   "test-topic",
	}

	pub, err := NewPubSubPublisher(config)
	if err != nil {
		t.Fatalf("NewPubSubPublisher failed: %v", err)
	}

	if pub.Name() != "gcp-pubsub" {
		t.Errorf("Name = %s, want gcp-pubsub", pub.Name())
	}

	if err := pub.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestPubSubSubscriber(t *testing.T) {
	config := &PubSubConfig{
		ProjectID:      "test-project",
		SubscriptionID: "test-subscription",
	}

	sub := NewPubSubSubscriber(config)

	if sub.Name() != "gcp-pubsub" {
		t.Errorf("Name = %s, want gcp-pubsub", sub.Name())
	}

	// Test stop
	go func() {
		time.Sleep(100 * time.Millisecond)
		sub.Stop()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	sub.Start(ctx, func(ctx context.Context, event *ChronosEvent) error {
		return nil
	})
}

func TestEventGridPublisher(t *testing.T) {
	config := &EventGridConfig{
		TopicEndpoint: "https://test.eventgrid.azure.net",
		TopicKey:      "key",
	}

	pub, err := NewEventGridPublisher(config)
	if err != nil {
		t.Fatalf("NewEventGridPublisher failed: %v", err)
	}

	if pub.Name() != "azure-eventgrid" {
		t.Errorf("Name = %s, want azure-eventgrid", pub.Name())
	}

	if err := pub.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestEventGridSubscriber(t *testing.T) {
	config := &EventGridConfig{
		TopicEndpoint: "https://test.eventgrid.azure.net",
		TopicKey:      "key",
	}

	sub := NewEventGridSubscriber(config, "127.0.0.1:0")

	if sub.Name() != "azure-eventgrid" {
		t.Errorf("Name = %s, want azure-eventgrid", sub.Name())
	}
}

func TestTriggerManager_AddRule(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})
	tm := NewTriggerManager(mesh, nil)

	rule := &TriggerRule{
		Name:       "Test Rule",
		EventTypes: []EventType{EventJobSucceeded},
		JobID:      "job-1",
		Enabled:    true,
	}

	err := tm.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	if rule.ID == "" {
		t.Error("ID should be set")
	}
	if rule.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestTriggerManager_RemoveRule(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})
	tm := NewTriggerManager(mesh, nil)

	rule := &TriggerRule{
		ID:         "rule-1",
		Name:       "Test Rule",
		EventTypes: []EventType{EventJobSucceeded},
		JobID:      "job-1",
	}
	tm.AddRule(rule)

	err := tm.RemoveRule("rule-1")
	if err != nil {
		t.Fatalf("RemoveRule failed: %v", err)
	}

	rules := tm.ListRules()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestTriggerManager_ListRules(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})
	tm := NewTriggerManager(mesh, nil)

	for i := 0; i < 3; i++ {
		tm.AddRule(&TriggerRule{
			Name:       "Test Rule",
			EventTypes: []EventType{EventJobSucceeded},
			JobID:      "job-1",
		})
	}

	rules := tm.ListRules()
	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}
}

func TestTriggerManager_HandleEvent(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	var triggeredJobID string
	var triggeredInputs map[string]string
	var mu sync.Mutex

	jobFunc := func(ctx context.Context, jobID string, inputs map[string]string) error {
		mu.Lock()
		triggeredJobID = jobID
		triggeredInputs = inputs
		mu.Unlock()
		return nil
	}

	tm := NewTriggerManager(mesh, jobFunc)

	rule := &TriggerRule{
		Name:       "Test Rule",
		EventTypes: []EventType{EventJobSucceeded},
		JobID:      "target-job",
		Enabled:    true,
		JobInputs: map[string]string{
			"result_id": "job_id",
		},
	}
	tm.AddRule(rule)

	event := &ChronosEvent{
		ID:   "event-1",
		Type: EventJobSucceeded,
		Data: map[string]interface{}{
			"job_id": "completed-job",
		},
	}

	tm.handleEvent(context.Background(), event)

	// Wait for async job trigger
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if triggeredJobID != "target-job" {
		t.Errorf("triggered job ID = %s, want target-job", triggeredJobID)
	}
	if triggeredInputs["result_id"] != "completed-job" {
		t.Errorf("triggered input result_id = %s, want completed-job", triggeredInputs["result_id"])
	}
}

func TestTriggerManager_HandleEvent_DisabledRule(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	triggered := false
	jobFunc := func(ctx context.Context, jobID string, inputs map[string]string) error {
		triggered = true
		return nil
	}

	tm := NewTriggerManager(mesh, jobFunc)

	rule := &TriggerRule{
		Name:       "Disabled Rule",
		EventTypes: []EventType{EventJobSucceeded},
		JobID:      "target-job",
		Enabled:    false,
	}
	tm.AddRule(rule)

	event := &ChronosEvent{
		ID:   "event-1",
		Type: EventJobSucceeded,
	}

	tm.handleEvent(context.Background(), event)
	time.Sleep(100 * time.Millisecond)

	if triggered {
		t.Error("disabled rule should not trigger job")
	}
}

func TestTriggerManager_HandleEvent_WithFilter(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	triggeredCount := 0
	var mu sync.Mutex

	jobFunc := func(ctx context.Context, jobID string, inputs map[string]string) error {
		mu.Lock()
		triggeredCount++
		mu.Unlock()
		return nil
	}

	tm := NewTriggerManager(mesh, jobFunc)

	rule := &TriggerRule{
		Name:       "Filtered Rule",
		EventTypes: []EventType{EventJobSucceeded},
		JobID:      "target-job",
		Enabled:    true,
		Filter: &EventFilter{
			Source: "specific-source",
			DataMatches: map[string]string{
				"environment": "production",
			},
		},
	}
	tm.AddRule(rule)

	// Event that doesn't match filter
	event1 := &ChronosEvent{
		ID:     "event-1",
		Type:   EventJobSucceeded,
		Source: "other-source",
		Data:   map[string]interface{}{"environment": "production"},
	}
	tm.handleEvent(context.Background(), event1)

	// Event that matches filter
	event2 := &ChronosEvent{
		ID:     "event-2",
		Type:   EventJobSucceeded,
		Source: "specific-source",
		Data:   map[string]interface{}{"environment": "production"},
	}
	tm.handleEvent(context.Background(), event2)

	// Event with wrong data
	event3 := &ChronosEvent{
		ID:     "event-3",
		Type:   EventJobSucceeded,
		Source: "specific-source",
		Data:   map[string]interface{}{"environment": "staging"},
	}
	tm.handleEvent(context.Background(), event3)

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if triggeredCount != 1 {
		t.Errorf("expected 1 trigger, got %d", triggeredCount)
	}
}

func TestTriggerManager_HandleEvent_WrongEventType(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	triggered := false
	jobFunc := func(ctx context.Context, jobID string, inputs map[string]string) error {
		triggered = true
		return nil
	}

	tm := NewTriggerManager(mesh, jobFunc)

	rule := &TriggerRule{
		Name:       "Test Rule",
		EventTypes: []EventType{EventJobSucceeded},
		JobID:      "target-job",
		Enabled:    true,
	}
	tm.AddRule(rule)

	// Send wrong event type
	event := &ChronosEvent{
		ID:   "event-1",
		Type: EventJobFailed,
	}

	tm.handleEvent(context.Background(), event)
	time.Sleep(100 * time.Millisecond)

	if triggered {
		t.Error("wrong event type should not trigger")
	}
}

func TestConcurrentEventMeshAccess(t *testing.T) {
	mesh, _ := NewEventMesh(EventMeshConfig{})

	done := make(chan bool)

	// Subscribe concurrently
	go func() {
		for i := 0; i < 100; i++ {
			mesh.Subscribe(EventJobCreated, func(ctx context.Context, event *ChronosEvent) error {
				return nil
			})
		}
		done <- true
	}()

	// Handle events concurrently
	go func() {
		for i := 0; i < 100; i++ {
			mesh.HandleIncoming(context.Background(), &ChronosEvent{
				ID:   "test",
				Type: EventJobCreated,
			})
		}
		done <- true
	}()

	<-done
	<-done
}
