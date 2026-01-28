package notify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("expected hub, got nil")
	}
	if hub.channels == nil {
		t.Error("expected channels map to be initialized")
	}
}

func TestHub_AddChannel(t *testing.T) {
	hub := NewHub()

	channel := &Channel{
		ID:      "ch-1",
		Name:    "Test Channel",
		Type:    ChannelSlack,
		Enabled: true,
		Config: ChannelConfig{
			SlackWebhookURL: "https://hooks.slack.com/test",
		},
	}

	err := hub.AddChannel(context.Background(), channel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify timestamps were set
	if channel.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestHub_AddChannel_Duplicate(t *testing.T) {
	hub := NewHub()

	channel := &Channel{
		ID:      "ch-1",
		Name:    "Test Channel",
		Type:    ChannelSlack,
		Enabled: true,
	}

	hub.AddChannel(context.Background(), channel)
	err := hub.AddChannel(context.Background(), channel)

	if err != ErrChannelExists {
		t.Errorf("expected ErrChannelExists, got %v", err)
	}
}

func TestHub_AddChannel_Invalid(t *testing.T) {
	hub := NewHub()

	tests := []struct {
		name    string
		channel *Channel
	}{
		{"missing ID", &Channel{Name: "test", Type: ChannelSlack}},
		{"missing Name", &Channel{ID: "ch-1", Type: ChannelSlack}},
		{"missing Type", &Channel{ID: "ch-1", Name: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hub.AddChannel(context.Background(), tt.channel)
			if err != ErrInvalidChannel {
				t.Errorf("expected ErrInvalidChannel, got %v", err)
			}
		})
	}
}

func TestHub_GetChannel(t *testing.T) {
	hub := NewHub()

	channel := &Channel{
		ID:      "ch-1",
		Name:    "Test Channel",
		Type:    ChannelSlack,
		Enabled: true,
	}

	hub.AddChannel(context.Background(), channel)

	retrieved, err := hub.GetChannel(context.Background(), "ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retrieved.Name != "Test Channel" {
		t.Errorf("expected name 'Test Channel', got %q", retrieved.Name)
	}
}

func TestHub_GetChannel_NotFound(t *testing.T) {
	hub := NewHub()

	_, err := hub.GetChannel(context.Background(), "nonexistent")
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestHub_UpdateChannel(t *testing.T) {
	hub := NewHub()

	channel := &Channel{
		ID:      "ch-1",
		Name:    "Test Channel",
		Type:    ChannelSlack,
		Enabled: true,
	}

	hub.AddChannel(context.Background(), channel)

	channel.Name = "Updated Channel"
	err := hub.UpdateChannel(context.Background(), channel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, _ := hub.GetChannel(context.Background(), "ch-1")
	if retrieved.Name != "Updated Channel" {
		t.Errorf("expected name 'Updated Channel', got %q", retrieved.Name)
	}
}

func TestHub_UpdateChannel_NotFound(t *testing.T) {
	hub := NewHub()

	channel := &Channel{ID: "nonexistent", Name: "test", Type: ChannelSlack}
	err := hub.UpdateChannel(context.Background(), channel)
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestHub_DeleteChannel(t *testing.T) {
	hub := NewHub()

	channel := &Channel{
		ID:      "ch-1",
		Name:    "Test Channel",
		Type:    ChannelSlack,
		Enabled: true,
	}

	hub.AddChannel(context.Background(), channel)
	err := hub.DeleteChannel(context.Background(), "ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = hub.GetChannel(context.Background(), "ch-1")
	if err != ErrChannelNotFound {
		t.Error("expected channel to be deleted")
	}
}

func TestHub_DeleteChannel_NotFound(t *testing.T) {
	hub := NewHub()

	err := hub.DeleteChannel(context.Background(), "nonexistent")
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestHub_ListChannels(t *testing.T) {
	hub := NewHub()

	for i := 0; i < 5; i++ {
		channel := &Channel{
			ID:      fmt.Sprintf("ch-%d", i),
			Name:    fmt.Sprintf("Channel %d", i),
			Type:    ChannelSlack,
			Enabled: true,
		}
		hub.AddChannel(context.Background(), channel)
	}

	channels, err := hub.ListChannels(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channels) != 5 {
		t.Errorf("expected 5 channels, got %d", len(channels))
	}
}

func TestHub_ListChannels_ByTenant(t *testing.T) {
	hub := NewHub()

	hub.AddChannel(context.Background(), &Channel{
		ID:       "ch-1",
		Name:     "Tenant A Channel",
		Type:     ChannelSlack,
		TenantID: "tenant-a",
		Enabled:  true,
	})
	hub.AddChannel(context.Background(), &Channel{
		ID:       "ch-2",
		Name:     "Tenant B Channel",
		Type:     ChannelSlack,
		TenantID: "tenant-b",
		Enabled:  true,
	})

	channelsA, err := hub.ListChannels(context.Background(), "tenant-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(channelsA) != 1 {
		t.Errorf("expected 1 channel for tenant-a, got %d", len(channelsA))
	}
}

func TestHub_Send_WebhookChannel(t *testing.T) {
	receivedRequest := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hub := NewHub()

	channel := &Channel{
		ID:      "ch-webhook",
		Name:    "Webhook Channel",
		Type:    ChannelWebhook,
		Enabled: true,
		Config: ChannelConfig{
			WebhookURL: server.URL,
		},
	}
	hub.AddChannel(context.Background(), channel)

	notification := &Notification{
		ID:        "notif-1",
		ChannelID: "ch-webhook",
		Event:     EventJobFailed,
		Title:     "Job Failed",
		Message:   "Job xyz failed",
		Severity:  SeverityError,
		Timestamp: time.Now(),
	}

	status, err := hub.Send(context.Background(), notification)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !receivedRequest {
		t.Error("expected webhook request to be sent")
	}
	if status.Status != "delivered" {
		t.Errorf("expected status 'delivered', got %q", status.Status)
	}
}

func TestHub_Send_DisabledChannel(t *testing.T) {
	hub := NewHub()

	channel := &Channel{
		ID:      "ch-disabled",
		Name:    "Disabled Channel",
		Type:    ChannelSlack,
		Enabled: false,
	}
	hub.AddChannel(context.Background(), channel)

	notification := &Notification{
		ID:        "notif-1",
		ChannelID: "ch-disabled",
		Event:     EventJobFailed,
		Title:     "Test",
		Message:   "Test message",
		Timestamp: time.Now(),
	}

	status, err := hub.Send(context.Background(), notification)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != "skipped" {
		t.Errorf("expected status 'skipped', got %q", status.Status)
	}
}

func TestHub_Send_ChannelNotFound(t *testing.T) {
	hub := NewHub()

	notification := &Notification{
		ID:        "notif-1",
		ChannelID: "nonexistent",
		Event:     EventJobFailed,
		Title:     "Test",
		Message:   "Test message",
		Timestamp: time.Now(),
	}

	_, err := hub.Send(context.Background(), notification)
	if err != ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound, got %v", err)
	}
}

func TestHub_GetHistory(t *testing.T) {
	hub := NewHub()

	// Send to test webhook to populate history
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel := &Channel{
		ID:      "ch-1",
		Name:    "Test",
		Type:    ChannelWebhook,
		Enabled: true,
		Config:  ChannelConfig{WebhookURL: server.URL},
	}
	hub.AddChannel(context.Background(), channel)

	notification := &Notification{
		ID:        "notif-1",
		ChannelID: "ch-1",
		Event:     EventJobFailed,
		Title:     "Test",
		Message:   "Test",
		Timestamp: time.Now(),
	}

	hub.Send(context.Background(), notification)

	history := hub.GetHistory(context.Background(), 10)
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestChannelTypes(t *testing.T) {
	types := []ChannelType{
		ChannelSlack,
		ChannelPagerDuty,
		ChannelTeams,
		ChannelDiscord,
		ChannelEmail,
		ChannelWebhook,
		ChannelOpsgenie,
	}

	for _, ct := range types {
		if ct == "" {
			t.Error("channel type should not be empty")
		}
	}
}

func TestSeverityLevels(t *testing.T) {
	levels := []Severity{
		SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityCritical,
	}

	for _, s := range levels {
		if s == "" {
			t.Error("severity should not be empty")
		}
	}
}

func TestEventTypes(t *testing.T) {
	events := []EventType{
		EventJobFailed,
		EventJobSucceeded,
		EventJobStarted,
		EventDAGFailed,
		EventDAGSucceeded,
		EventQuotaWarning,
		EventQuotaExceeded,
		EventSystemAlert,
	}

	for _, e := range events {
		if e == "" {
			t.Error("event type should not be empty")
		}
	}
}


