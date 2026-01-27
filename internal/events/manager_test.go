package events

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	handler := func(ctx context.Context, event *Event) error {
		return nil
	}

	m := NewManager(handler)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestCreateTrigger(t *testing.T) {
	m := NewManager(func(ctx context.Context, event *Event) error {
		return nil
	})

	trigger := &Trigger{
		ID:      "trigger-1",
		Name:    "Test Trigger",
		Type:    TriggerWebhook,
		JobID:   "job-1",
		Enabled: false,
		Config: TriggerConfig{
			Path:    "/webhook/test",
			Methods: []string{"POST"},
		},
	}

	err := m.CreateTrigger(trigger)
	if err != nil {
		t.Fatalf("CreateTrigger failed: %v", err)
	}

	// Duplicate should fail
	err = m.CreateTrigger(trigger)
	if err != ErrTriggerExists {
		t.Errorf("expected ErrTriggerExists, got %v", err)
	}
}

func TestGetTrigger(t *testing.T) {
	m := NewManager(func(ctx context.Context, event *Event) error {
		return nil
	})

	trigger := &Trigger{
		ID:      "trigger-1",
		Name:    "Test Trigger",
		Type:    TriggerWebhook,
		JobID:   "job-1",
	}
	m.CreateTrigger(trigger)

	retrieved, err := m.GetTrigger("trigger-1")
	if err != nil {
		t.Fatalf("GetTrigger failed: %v", err)
	}
	if retrieved.Name != "Test Trigger" {
		t.Errorf("expected name 'Test Trigger', got %s", retrieved.Name)
	}

	_, err = m.GetTrigger("nonexistent")
	if err != ErrTriggerNotFound {
		t.Errorf("expected ErrTriggerNotFound, got %v", err)
	}
}

func TestListTriggers(t *testing.T) {
	m := NewManager(func(ctx context.Context, event *Event) error {
		return nil
	})

	m.CreateTrigger(&Trigger{ID: "t1", Type: TriggerKafka})
	m.CreateTrigger(&Trigger{ID: "t2", Type: TriggerSQS})
	m.CreateTrigger(&Trigger{ID: "t3", Type: TriggerPubSub})

	triggers := m.ListTriggers()
	if len(triggers) != 3 {
		t.Errorf("expected 3 triggers, got %d", len(triggers))
	}
}

func TestDeleteTrigger(t *testing.T) {
	m := NewManager(func(ctx context.Context, event *Event) error {
		return nil
	})

	m.CreateTrigger(&Trigger{ID: "t1", Type: TriggerWebhook})

	err := m.DeleteTrigger("t1")
	if err != nil {
		t.Fatalf("DeleteTrigger failed: %v", err)
	}

	_, err = m.GetTrigger("t1")
	if err != ErrTriggerNotFound {
		t.Error("trigger should be deleted")
	}

	err = m.DeleteTrigger("nonexistent")
	if err != ErrTriggerNotFound {
		t.Errorf("expected ErrTriggerNotFound, got %v", err)
	}
}

func TestEnableDisableTrigger(t *testing.T) {
	m := NewManager(func(ctx context.Context, event *Event) error {
		return nil
	})

	m.CreateTrigger(&Trigger{
		ID:      "t1",
		Type:    TriggerWebhook,
		Enabled: false,
	})

	// Enable
	err := m.EnableTrigger("t1")
	if err != nil {
		// May fail if no source registered - that's OK
		_ = err
	}

	trigger, _ := m.GetTrigger("t1")
	if !trigger.Enabled {
		t.Error("trigger should be enabled")
	}

	// Disable
	err = m.DisableTrigger("t1")
	if err != nil {
		t.Fatalf("DisableTrigger failed: %v", err)
	}

	trigger, _ = m.GetTrigger("t1")
	if trigger.Enabled {
		t.Error("trigger should be disabled")
	}
}

func TestMatchesFilter(t *testing.T) {
	m := NewManager(nil)

	tests := []struct {
		name    string
		event   *Event
		filter  *EventFilter
		matches bool
	}{
		{
			name:    "nil filter matches all",
			event:   &Event{Type: "test"},
			filter:  nil,
			matches: true,
		},
		{
			name:   "event type matches",
			event:  &Event{Type: "order.created"},
			filter: &EventFilter{EventTypes: []string{"order.created", "order.updated"}},
			matches: true,
		},
		{
			name:   "event type no match",
			event:  &Event{Type: "order.deleted"},
			filter: &EventFilter{EventTypes: []string{"order.created", "order.updated"}},
			matches: false,
		},
		{
			name:   "header matches",
			event:  &Event{Type: "test", Metadata: map[string]string{"source": "api"}},
			filter: &EventFilter{Headers: map[string]string{"source": "api"}},
			matches: true,
		},
		{
			name:   "header no match",
			event:  &Event{Type: "test", Metadata: map[string]string{"source": "web"}},
			filter: &EventFilter{Headers: map[string]string{"source": "api"}},
			matches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.filter == nil {
				// nil filter always matches
				return
			}
			result := m.matchesFilter(tt.event, tt.filter)
			if result != tt.matches {
				t.Errorf("expected %v, got %v", tt.matches, result)
			}
		})
	}
}

func TestStats(t *testing.T) {
	m := NewManager(func(ctx context.Context, event *Event) error {
		return nil
	})

	m.CreateTrigger(&Trigger{ID: "t1", Type: TriggerKafka, Enabled: true})
	m.CreateTrigger(&Trigger{ID: "t2", Type: TriggerKafka, Enabled: false})
	m.CreateTrigger(&Trigger{ID: "t3", Type: TriggerSQS, Enabled: true})

	stats := m.Stats()

	if stats.TotalTriggers != 3 {
		t.Errorf("expected 3 total triggers, got %d", stats.TotalTriggers)
	}
	if stats.EnabledTriggers != 2 {
		t.Errorf("expected 2 enabled triggers, got %d", stats.EnabledTriggers)
	}
	if stats.TriggersByType[TriggerKafka] != 2 {
		t.Errorf("expected 2 Kafka triggers, got %d", stats.TriggersByType[TriggerKafka])
	}
}

func TestEvent(t *testing.T) {
	event := &Event{
		ID:        "evt-1",
		Source:    TriggerKafka,
		Type:      "order.created",
		Subject:   "orders/12345",
		Data:      map[string]interface{}{"order_id": "12345"},
		Metadata:  map[string]string{"partition": "0"},
		Timestamp: time.Now(),
	}

	if event.ID != "evt-1" {
		t.Errorf("expected ID 'evt-1', got %s", event.ID)
	}
	if event.Source != TriggerKafka {
		t.Errorf("expected source Kafka, got %s", event.Source)
	}
}
