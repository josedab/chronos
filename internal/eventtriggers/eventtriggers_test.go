package eventtriggers

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestEventTriggerManager(t *testing.T) {
	logger := zerolog.Nop()
	triggered := make(map[string]int)

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		triggered[jobID]++
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	// Register a trigger
	trigger := &EventTrigger{
		Name:    "test-trigger",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
	}

	err := manager.RegisterTrigger(trigger)
	if err != nil {
		t.Fatalf("failed to register trigger: %v", err)
	}

	if trigger.ID == "" {
		t.Error("trigger ID should be generated")
	}

	// List triggers
	triggers := manager.ListTriggers()
	if len(triggers) != 1 {
		t.Errorf("expected 1 trigger, got %d", len(triggers))
	}

	// Process event
	event := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data:   map[string]interface{}{"key": "value"},
	}

	results, err := manager.ProcessEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("failed to process event: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if !results[0].Triggered {
		t.Error("expected job to be triggered")
	}

	if triggered["job-1"] != 1 {
		t.Errorf("expected job-1 to be triggered once, got %d", triggered["job-1"])
	}
}

func TestEventTriggerFilters(t *testing.T) {
	logger := zerolog.Nop()
	triggered := make(map[string]int)

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		triggered[jobID]++
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	// Register a trigger with filters
	trigger := &EventTrigger{
		Name:    "filtered-trigger",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
		Filters: []EventFilter{
			{Field: "status", Operator: "eq", Value: "success"},
		},
	}

	manager.RegisterTrigger(trigger)

	// Event that passes filter
	event1 := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data:   map[string]interface{}{"status": "success"},
	}

	results, _ := manager.ProcessEvent(context.Background(), event1)
	if !results[0].Triggered {
		t.Error("expected job to be triggered for matching event")
	}

	// Event that fails filter
	event2 := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data:   map[string]interface{}{"status": "failure"},
	}

	results, _ = manager.ProcessEvent(context.Background(), event2)
	if results[0].Triggered {
		t.Error("expected job NOT to be triggered for non-matching event")
	}
}

func TestS3TriggerMatching(t *testing.T) {
	logger := zerolog.Nop()
	triggered := make(map[string]int)

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		triggered[jobID]++
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	// Register S3 trigger
	trigger := CreateS3Trigger("s3-trigger", "job-1", "my-bucket", "uploads/", ".csv", nil)
	manager.RegisterTrigger(trigger)

	tests := []struct {
		name      string
		bucket    string
		key       string
		eventName string
		shouldTrigger bool
	}{
		{
			name:          "matching event",
			bucket:        "my-bucket",
			key:           "uploads/data.csv",
			eventName:     "s3:ObjectCreated:Put",
			shouldTrigger: true,
		},
		{
			name:          "wrong bucket",
			bucket:        "other-bucket",
			key:           "uploads/data.csv",
			eventName:     "s3:ObjectCreated:Put",
			shouldTrigger: false,
		},
		{
			name:          "wrong prefix",
			bucket:        "my-bucket",
			key:           "downloads/data.csv",
			eventName:     "s3:ObjectCreated:Put",
			shouldTrigger: false,
		},
		{
			name:          "wrong suffix",
			bucket:        "my-bucket",
			key:           "uploads/data.json",
			eventName:     "s3:ObjectCreated:Put",
			shouldTrigger: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			triggered = make(map[string]int)

			event := &TriggerEvent{
				Type:   TriggerTypeS3,
				Source: "aws:s3",
				Data: map[string]interface{}{
					"bucket":     tc.bucket,
					"key":        tc.key,
					"event_name": tc.eventName,
				},
			}

			results, _ := manager.ProcessEvent(context.Background(), event)
			
			var wasTriggered bool
			for _, r := range results {
				if r.Triggered {
					wasTriggered = true
					break
				}
			}

			if wasTriggered != tc.shouldTrigger {
				t.Errorf("expected shouldTrigger=%v, got %v", tc.shouldTrigger, wasTriggered)
			}
		})
	}
}

func TestRateLimiting(t *testing.T) {
	logger := zerolog.Nop()
	triggerCount := 0

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		triggerCount++
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	// Register trigger with rate limit
	trigger := &EventTrigger{
		Name:    "rate-limited",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
		RateLimit: &RateLimitConfig{
			MaxPerMinute: 2,
			BurstSize:    2,
		},
	}

	manager.RegisterTrigger(trigger)

	event := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data:   map[string]interface{}{},
	}

	// First two should succeed
	manager.ProcessEvent(context.Background(), event)
	manager.ProcessEvent(context.Background(), event)

	// Third should be rate limited
	results, _ := manager.ProcessEvent(context.Background(), event)
	
	if len(results) > 0 && results[0].Triggered {
		t.Error("expected third request to be rate limited")
	}

	if triggerCount != 2 {
		t.Errorf("expected 2 triggers, got %d", triggerCount)
	}
}

func TestTriggerEnableDisable(t *testing.T) {
	logger := zerolog.Nop()
	triggerCount := 0

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		triggerCount++
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	trigger := &EventTrigger{
		Name:    "toggle-trigger",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
	}

	manager.RegisterTrigger(trigger)

	event := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data:   map[string]interface{}{},
	}

	// Should trigger when enabled
	manager.ProcessEvent(context.Background(), event)
	if triggerCount != 1 {
		t.Errorf("expected 1 trigger, got %d", triggerCount)
	}

	// Disable and verify no trigger
	manager.DisableTrigger(trigger.ID)
	manager.ProcessEvent(context.Background(), event)
	if triggerCount != 1 {
		t.Errorf("expected still 1 trigger after disable, got %d", triggerCount)
	}

	// Re-enable and verify trigger
	manager.EnableTrigger(trigger.ID)
	manager.ProcessEvent(context.Background(), event)
	if triggerCount != 2 {
		t.Errorf("expected 2 triggers after re-enable, got %d", triggerCount)
	}
}

func TestEventTransform(t *testing.T) {
	logger := zerolog.Nop()
	var capturedParams map[string]interface{}

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		capturedParams = params
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	trigger := &EventTrigger{
		Name:    "transform-trigger",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
		Transform: &EventTransform{
			Mappings: map[string]string{
				"file_path": "object.key",
			},
			Static: map[string]interface{}{
				"env": "production",
			},
		},
	}

	manager.RegisterTrigger(trigger)

	event := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data: map[string]interface{}{
			"object": map[string]interface{}{
				"key": "/data/file.csv",
			},
		},
	}

	manager.ProcessEvent(context.Background(), event)

	// Check transformed params
	if capturedParams["file_path"] != "/data/file.csv" {
		t.Errorf("expected file_path=/data/file.csv, got %v", capturedParams["file_path"])
	}
	if capturedParams["env"] != "production" {
		t.Errorf("expected env=production, got %v", capturedParams["env"])
	}
}

func TestGitHubTrigger(t *testing.T) {
	logger := zerolog.Nop()

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	trigger := CreateGitHubTrigger("gh-trigger", "job-1", "owner/repo", []string{"push"}, []string{"main", "develop"})
	manager.RegisterTrigger(trigger)

	// Test matching push event on main
	event := GitHubEventFromWebhook("push", map[string]interface{}{
		"repository": map[string]interface{}{"full_name": "owner/repo"},
		"ref":        "refs/heads/main",
	})

	results, _ := manager.ProcessEvent(context.Background(), event)
	
	if len(results) == 0 || !results[0].Triggered {
		t.Error("expected GitHub push to main to trigger")
	}

	// Test non-matching branch
	// Test non-matching branch
	event2 := GitHubEventFromWebhook("push", map[string]interface{}{
		"repository": map[string]interface{}{"full_name": "owner/repo"},
		"ref":        "refs/heads/feature/test",
	})

	results, _ = manager.ProcessEvent(context.Background(), event2)
	
	wasTriggered := false
	for _, r := range results {
		if r.Triggered {
			wasTriggered = true
			break
		}
	}

	if wasTriggered {
		t.Error("expected GitHub push to feature branch NOT to trigger")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("CreateS3Trigger", func(t *testing.T) {
		trigger := CreateS3Trigger("test", "job-1", "bucket", "prefix/", ".csv", nil)
		if trigger.Type != TriggerTypeS3 {
			t.Errorf("expected type S3, got %s", trigger.Type)
		}
		if trigger.Config.Bucket != "bucket" {
			t.Errorf("expected bucket=bucket, got %s", trigger.Config.Bucket)
		}
		if len(trigger.Config.Events) == 0 {
			t.Error("expected default events to be set")
		}
	})

	t.Run("CreateKafkaTrigger", func(t *testing.T) {
		trigger := CreateKafkaTrigger("test", "job-1", "topic", "group", "localhost:9092")
		if trigger.Type != TriggerTypeKafka {
			t.Errorf("expected type Kafka, got %s", trigger.Type)
		}
		if trigger.Config.Topic != "topic" {
			t.Errorf("expected topic=topic, got %s", trigger.Config.Topic)
		}
	})

	t.Run("CreateWebhookTrigger", func(t *testing.T) {
		trigger := CreateWebhookTrigger("test", "job-1", "secret123")
		if trigger.Type != TriggerTypeWebhook {
			t.Errorf("expected type Webhook, got %s", trigger.Type)
		}
		if trigger.Config.SecretToken != "secret123" {
			t.Errorf("expected secret=secret123, got %s", trigger.Config.SecretToken)
		}
	})
}

func TestKafkaEventFromMessage(t *testing.T) {
	headers := map[string]string{"X-Header": "value"}
	event := KafkaEventFromMessage("my-topic", []byte("key"), []byte(`{"name":"test"}`), headers)

	if event.Type != TriggerTypeKafka {
		t.Errorf("expected type Kafka, got %s", event.Type)
	}
	if event.Data["topic"] != "my-topic" {
		t.Errorf("expected topic=my-topic, got %v", event.Data["topic"])
	}
	if event.Data["name"] != "test" {
		t.Errorf("expected name=test, got %v", event.Data["name"])
	}
}

func TestGetStats(t *testing.T) {
	logger := zerolog.Nop()
	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	// Add some triggers
	manager.RegisterTrigger(&EventTrigger{
		Name:    "t1",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
	})
	manager.RegisterTrigger(&EventTrigger{
		Name:    "t2",
		Type:    TriggerTypeS3,
		JobID:   "job-2",
		Enabled: false,
	})
	manager.RegisterTrigger(&EventTrigger{
		Name:    "t3",
		Type:    TriggerTypeWebhook,
		JobID:   "job-3",
		Enabled: true,
	})

	stats := manager.GetStats()

	if stats.TotalTriggers != 3 {
		t.Errorf("expected 3 total triggers, got %d", stats.TotalTriggers)
	}
	if stats.EnabledCount != 2 {
		t.Errorf("expected 2 enabled, got %d", stats.EnabledCount)
	}
	if stats.DisabledCount != 1 {
		t.Errorf("expected 1 disabled, got %d", stats.DisabledCount)
	}
	if stats.ByType["webhook"] != 2 {
		t.Errorf("expected 2 webhook triggers, got %d", stats.ByType["webhook"])
	}
}

func TestNestedFieldFilter(t *testing.T) {
	logger := zerolog.Nop()

	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	trigger := &EventTrigger{
		Name:    "nested-filter",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
		Filters: []EventFilter{
			{Field: "user.role", Operator: "eq", Value: "admin"},
		},
	}

	manager.RegisterTrigger(trigger)

	// Matching nested field
	event := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"role": "admin",
			},
		},
	}

	results, _ := manager.ProcessEvent(context.Background(), event)
	if !results[0].Triggered {
		t.Error("expected nested field filter to match")
	}

	// Non-matching nested field
	event2 := &TriggerEvent{
		Type:   TriggerTypeWebhook,
		Source: "test",
		Data: map[string]interface{}{
			"user": map[string]interface{}{
				"role": "viewer",
			},
		},
	}

	results, _ = manager.ProcessEvent(context.Background(), event2)
	if results[0].Triggered {
		t.Error("expected nested field filter NOT to match")
	}
}

func TestTriggerTimestamps(t *testing.T) {
	logger := zerolog.Nop()
	triggerFunc := func(ctx context.Context, jobID string, params map[string]interface{}) (string, error) {
		return "exec-123", nil
	}

	manager := NewManager(triggerFunc, logger)

	before := time.Now().UTC()
	trigger := &EventTrigger{
		Name:    "timestamp-test",
		Type:    TriggerTypeWebhook,
		JobID:   "job-1",
		Enabled: true,
	}

	manager.RegisterTrigger(trigger)
	after := time.Now().UTC()

	if trigger.CreatedAt.Before(before) || trigger.CreatedAt.After(after) {
		t.Error("CreatedAt should be between before and after registration")
	}
	if trigger.UpdatedAt != trigger.CreatedAt {
		t.Error("UpdatedAt should equal CreatedAt on creation")
	}

	// Update trigger
	time.Sleep(time.Millisecond)
	manager.UpdateTrigger(trigger.ID, &EventTrigger{Name: "updated-name"})

	updated, _ := manager.GetTrigger(trigger.ID)
	if updated.UpdatedAt == updated.CreatedAt {
		t.Error("UpdatedAt should be updated after modification")
	}
}
