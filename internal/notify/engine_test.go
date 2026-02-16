package notify

import (
	"context"
	"testing"
)

func TestAlertEngine_AddRule(t *testing.T) {
	hub := NewHub()
	engine := NewAlertEngine(hub)

	rule := &AlertRule{
		Name:       "failure-alert",
		Enabled:    true,
		Events:     []EventType{EventJobFailed},
		ChannelIDs: []string{"ch-1"},
		Severity:   SeverityError,
	}

	if err := engine.AddRule(rule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.ID == "" {
		t.Error("expected auto-generated ID")
	}

	found, err := engine.GetRule(rule.ID)
	if err != nil || found.Name != "failure-alert" {
		t.Error("expected rule to be retrievable")
	}
}

func TestAlertEngine_AddRule_Validation(t *testing.T) {
	engine := NewAlertEngine(NewHub())

	if err := engine.AddRule(&AlertRule{Events: []EventType{EventJobFailed}, ChannelIDs: []string{"ch"}}); err == nil {
		t.Error("expected error for missing name")
	}
	if err := engine.AddRule(&AlertRule{Name: "r", ChannelIDs: []string{"ch"}}); err == nil {
		t.Error("expected error for missing events")
	}
	if err := engine.AddRule(&AlertRule{Name: "r", Events: []EventType{EventJobFailed}}); err == nil {
		t.Error("expected error for missing channels")
	}
}

func TestAlertEngine_Evaluate(t *testing.T) {
	hub := NewHub()
	// Add a webhook channel (won't actually send in test)
	hub.AddChannel(context.Background(), &Channel{
		ID: "ch-1", Name: "test", Type: ChannelWebhook, Enabled: true,
		Config: ChannelConfig{WebhookURL: "http://localhost:9999/hook"},
	})

	engine := NewAlertEngine(hub)
	engine.AddRule(&AlertRule{
		Name:       "all-failures",
		Enabled:    true,
		Events:     []EventType{EventJobFailed},
		ChannelIDs: []string{"ch-1"},
		Severity:   SeverityError,
	})

	// Should match
	records := engine.Evaluate(context.Background(), EventJobFailed, "job-1", "daily-backup", "exec-1", "connection refused")
	if len(records) != 1 {
		t.Fatalf("expected 1 delivery record, got %d", len(records))
	}
	if records[0].Event != EventJobFailed {
		t.Errorf("expected job_failed event, got %s", records[0].Event)
	}

	// Should not match (different event)
	records = engine.Evaluate(context.Background(), EventJobSucceeded, "job-1", "daily-backup", "exec-2", "")
	if len(records) != 0 {
		t.Errorf("expected 0 records for non-matching event, got %d", len(records))
	}
}

func TestAlertEngine_Cooldown(t *testing.T) {
	hub := NewHub()
	hub.AddChannel(context.Background(), &Channel{
		ID: "ch-1", Name: "test", Type: ChannelWebhook, Enabled: true,
		Config: ChannelConfig{WebhookURL: "http://localhost:9999/hook"},
	})

	engine := NewAlertEngine(hub)
	engine.AddRule(&AlertRule{
		Name:         "cooldown-rule",
		Enabled:      true,
		Events:       []EventType{EventJobFailed},
		ChannelIDs:   []string{"ch-1"},
		Severity:     SeverityWarning,
		CooldownMins: 60, // 60 minute cooldown
	})

	// First alert fires
	records1 := engine.Evaluate(context.Background(), EventJobFailed, "job-1", "test", "exec-1", "err")
	if len(records1) != 1 {
		t.Fatalf("expected 1 record on first fire, got %d", len(records1))
	}

	// Second alert within cooldown should be suppressed
	records2 := engine.Evaluate(context.Background(), EventJobFailed, "job-1", "test", "exec-2", "err")
	if len(records2) != 0 {
		t.Errorf("expected 0 records during cooldown, got %d", len(records2))
	}
}

func TestAlertEngine_JobFilter(t *testing.T) {
	hub := NewHub()
	hub.AddChannel(context.Background(), &Channel{
		ID: "ch-1", Name: "test", Type: ChannelWebhook, Enabled: true,
		Config: ChannelConfig{WebhookURL: "http://localhost:9999/hook"},
	})

	engine := NewAlertEngine(hub)
	engine.AddRule(&AlertRule{
		Name:       "prod-only",
		Enabled:    true,
		Events:     []EventType{EventJobFailed},
		ChannelIDs: []string{"ch-1"},
		JobFilter:  "prod-*",
		Severity:   SeverityCritical,
	})

	// Should match
	records := engine.Evaluate(context.Background(), EventJobFailed, "j1", "prod-backup", "e1", "err")
	if len(records) != 1 {
		t.Errorf("expected 1 for prod-backup, got %d", len(records))
	}

	// Should not match
	records = engine.Evaluate(context.Background(), EventJobFailed, "j2", "dev-backup", "e2", "err")
	if len(records) != 0 {
		t.Errorf("expected 0 for dev-backup, got %d", len(records))
	}
}

func TestAlertEngine_DisabledRule(t *testing.T) {
	engine := NewAlertEngine(NewHub())
	engine.AddRule(&AlertRule{
		Name:       "disabled",
		Enabled:    false,
		Events:     []EventType{EventJobFailed},
		ChannelIDs: []string{"ch-1"},
	})

	records := engine.Evaluate(context.Background(), EventJobFailed, "j1", "test", "e1", "err")
	if len(records) != 0 {
		t.Errorf("expected 0 for disabled rule, got %d", len(records))
	}
}

func TestAlertEngine_DeliveryHistory(t *testing.T) {
	hub := NewHub()
	hub.AddChannel(context.Background(), &Channel{
		ID: "ch-1", Name: "test", Type: ChannelWebhook, Enabled: true,
		Config: ChannelConfig{WebhookURL: "http://localhost:9999/hook"},
	})

	engine := NewAlertEngine(hub)
	engine.AddRule(&AlertRule{
		Name: "hist-rule", Enabled: true,
		Events: []EventType{EventJobFailed}, ChannelIDs: []string{"ch-1"},
	})

	engine.Evaluate(context.Background(), EventJobFailed, "j1", "test", "e1", "err1")
	engine.Evaluate(context.Background(), EventJobFailed, "j2", "test2", "e2", "err2")

	history := engine.DeliveryHistory(10)
	if len(history) != 2 {
		t.Errorf("expected 2 delivery records, got %d", len(history))
	}
}

func TestAlertEngine_RemoveRule(t *testing.T) {
	engine := NewAlertEngine(NewHub())
	engine.AddRule(&AlertRule{
		ID: "r1", Name: "temp", Enabled: true,
		Events: []EventType{EventJobFailed}, ChannelIDs: []string{"ch"},
	})

	if err := engine.RemoveRule("r1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := engine.RemoveRule("r1"); err == nil {
		t.Error("expected error for removing non-existent rule")
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "anything", true},
		{"prod-*", "prod-backup", true},
		{"prod-*", "dev-backup", false},
		{"*-cleanup", "db-cleanup", true},
		{"*-cleanup", "db-backup", false},
		{"exact", "exact", true},
		{"exact", "other", false},
	}

	for _, tc := range tests {
		got := matchGlob(tc.pattern, tc.s)
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}
