// Package notify provides native notification capabilities for Chronos.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Common errors.
var (
	ErrChannelNotFound    = errors.New("notification channel not found")
	ErrChannelExists      = errors.New("notification channel already exists")
	ErrInvalidChannel     = errors.New("invalid channel configuration")
	ErrDeliveryFailed     = errors.New("notification delivery failed")
)

// ChannelType represents the type of notification channel.
type ChannelType string

const (
	ChannelSlack      ChannelType = "slack"
	ChannelPagerDuty  ChannelType = "pagerduty"
	ChannelTeams      ChannelType = "teams"
	ChannelDiscord    ChannelType = "discord"
	ChannelEmail      ChannelType = "email"
	ChannelWebhook    ChannelType = "webhook"
	ChannelOpsgenie   ChannelType = "opsgenie"
)

// Channel represents a notification channel.
type Channel struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        ChannelType       `json:"type"`
	Config      ChannelConfig     `json:"config"`
	Enabled     bool              `json:"enabled"`
	TenantID    string            `json:"tenant_id,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ChannelConfig contains type-specific configuration.
type ChannelConfig struct {
	// Slack
	SlackWebhookURL string `json:"slack_webhook_url,omitempty"`
	SlackChannel    string `json:"slack_channel,omitempty"`
	
	// PagerDuty
	PagerDutyKey      string `json:"pagerduty_key,omitempty"`
	PagerDutySeverity string `json:"pagerduty_severity,omitempty"`
	
	// Teams
	TeamsWebhookURL string `json:"teams_webhook_url,omitempty"`
	
	// Discord
	DiscordWebhookURL string `json:"discord_webhook_url,omitempty"`
	
	// Email
	EmailSMTPHost     string   `json:"email_smtp_host,omitempty"`
	EmailSMTPPort     int      `json:"email_smtp_port,omitempty"`
	EmailFrom         string   `json:"email_from,omitempty"`
	EmailTo           []string `json:"email_to,omitempty"`
	
	// Webhook
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookHeaders map[string]string `json:"webhook_headers,omitempty"`
	
	// Opsgenie
	OpsgenieKey string `json:"opsgenie_key,omitempty"`
}

// Notification represents a notification to be sent.
type Notification struct {
	ID          string                 `json:"id"`
	ChannelID   string                 `json:"channel_id"`
	Event       EventType              `json:"event"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Severity    Severity               `json:"severity"`
	JobID       string                 `json:"job_id,omitempty"`
	JobName     string                 `json:"job_name,omitempty"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// EventType represents the type of event that triggered the notification.
type EventType string

const (
	EventJobFailed      EventType = "job_failed"
	EventJobSucceeded   EventType = "job_succeeded"
	EventJobStarted     EventType = "job_started"
	EventDAGFailed      EventType = "dag_failed"
	EventDAGSucceeded   EventType = "dag_succeeded"
	EventQuotaWarning   EventType = "quota_warning"
	EventQuotaExceeded  EventType = "quota_exceeded"
	EventSystemAlert    EventType = "system_alert"
)

// Severity represents notification severity.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// DeliveryStatus represents the status of a notification delivery.
type DeliveryStatus struct {
	NotificationID string    `json:"notification_id"`
	ChannelID      string    `json:"channel_id"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
	DeliveredAt    time.Time `json:"delivered_at,omitempty"`
	Attempts       int       `json:"attempts"`
}

// Hub manages notification channels and delivery.
type Hub struct {
	mu        sync.RWMutex
	channels  map[string]*Channel
	history   []DeliveryStatus
	client    *http.Client
}

// NewHub creates a new notification hub.
func NewHub() *Hub {
	return &Hub{
		channels: make(map[string]*Channel),
		history:  make([]DeliveryStatus, 0),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AddChannel adds a notification channel.
func (h *Hub) AddChannel(ctx context.Context, c *Channel) error {
	if c.ID == "" || c.Name == "" || c.Type == "" {
		return ErrInvalidChannel
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.channels[c.ID]; exists {
		return ErrChannelExists
	}

	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	h.channels[c.ID] = c
	return nil
}

// GetChannel retrieves a channel by ID.
func (h *Hub) GetChannel(ctx context.Context, id string) (*Channel, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	c, ok := h.channels[id]
	if !ok {
		return nil, ErrChannelNotFound
	}
	return c, nil
}

// UpdateChannel updates a channel.
func (h *Hub) UpdateChannel(ctx context.Context, c *Channel) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.channels[c.ID]; !ok {
		return ErrChannelNotFound
	}

	c.UpdatedAt = time.Now()
	h.channels[c.ID] = c
	return nil
}

// DeleteChannel deletes a channel.
func (h *Hub) DeleteChannel(ctx context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.channels[id]; !ok {
		return ErrChannelNotFound
	}

	delete(h.channels, id)
	return nil
}

// ListChannels lists all channels.
func (h *Hub) ListChannels(ctx context.Context, tenantID string) ([]*Channel, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*Channel
	for _, c := range h.channels {
		if tenantID == "" || c.TenantID == tenantID {
			result = append(result, c)
		}
	}
	return result, nil
}

// Send sends a notification to a channel.
func (h *Hub) Send(ctx context.Context, n *Notification) (*DeliveryStatus, error) {
	channel, err := h.GetChannel(ctx, n.ChannelID)
	if err != nil {
		return nil, err
	}

	if !channel.Enabled {
		return &DeliveryStatus{
			NotificationID: n.ID,
			ChannelID:      n.ChannelID,
			Status:         "skipped",
		}, nil
	}

	status := &DeliveryStatus{
		NotificationID: n.ID,
		ChannelID:      n.ChannelID,
		Attempts:       1,
	}

	var sendErr error
	switch channel.Type {
	case ChannelSlack:
		sendErr = h.sendSlack(ctx, channel, n)
	case ChannelPagerDuty:
		sendErr = h.sendPagerDuty(ctx, channel, n)
	case ChannelTeams:
		sendErr = h.sendTeams(ctx, channel, n)
	case ChannelDiscord:
		sendErr = h.sendDiscord(ctx, channel, n)
	case ChannelWebhook:
		sendErr = h.sendWebhook(ctx, channel, n)
	case ChannelOpsgenie:
		sendErr = h.sendOpsgenie(ctx, channel, n)
	default:
		sendErr = fmt.Errorf("unsupported channel type: %s", channel.Type)
	}

	if sendErr != nil {
		status.Status = "failed"
		status.Error = sendErr.Error()
	} else {
		status.Status = "delivered"
		status.DeliveredAt = time.Now()
	}

	h.mu.Lock()
	h.history = append(h.history, *status)
	h.mu.Unlock()

	return status, sendErr
}

// SendToAll sends a notification to all enabled channels matching criteria.
func (h *Hub) SendToAll(ctx context.Context, n *Notification, tenantID string) ([]*DeliveryStatus, error) {
	channels, err := h.ListChannels(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var results []*DeliveryStatus
	for _, c := range channels {
		if !c.Enabled {
			continue
		}

		notif := *n
		notif.ChannelID = c.ID
		status, _ := h.Send(ctx, &notif)
		results = append(results, status)
	}

	return results, nil
}

// sendSlack sends a Slack notification.
func (h *Hub) sendSlack(ctx context.Context, c *Channel, n *Notification) error {
	payload := map[string]interface{}{
		"text": n.Title,
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": n.Title,
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": n.Message,
				},
			},
			{
				"type": "context",
				"elements": []map[string]string{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Job:* %s | *Severity:* %s", n.JobName, n.Severity),
					},
				},
			},
		},
	}

	if c.Config.SlackChannel != "" {
		payload["channel"] = c.Config.SlackChannel
	}

	return h.postJSON(ctx, c.Config.SlackWebhookURL, payload)
}

// sendPagerDuty sends a PagerDuty alert.
func (h *Hub) sendPagerDuty(ctx context.Context, c *Channel, n *Notification) error {
	severity := "warning"
	if n.Severity == SeverityCritical || n.Severity == SeverityError {
		severity = "critical"
	}

	payload := map[string]interface{}{
		"routing_key":  c.Config.PagerDutyKey,
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":  n.Title,
			"severity": severity,
			"source":   "chronos",
			"custom_details": map[string]interface{}{
				"job_id":       n.JobID,
				"job_name":     n.JobName,
				"execution_id": n.ExecutionID,
				"message":      n.Message,
			},
		},
	}

	return h.postJSON(ctx, "https://events.pagerduty.com/v2/enqueue", payload)
}

// sendTeams sends a Microsoft Teams notification.
func (h *Hub) sendTeams(ctx context.Context, c *Channel, n *Notification) error {
	color := "0076D7" // Blue
	switch n.Severity {
	case SeverityWarning:
		color = "FFA500" // Orange
	case SeverityError, SeverityCritical:
		color = "FF0000" // Red
	}

	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": color,
		"summary":    n.Title,
		"sections": []map[string]interface{}{
			{
				"activityTitle": n.Title,
				"facts": []map[string]string{
					{"name": "Job", "value": n.JobName},
					{"name": "Severity", "value": string(n.Severity)},
					{"name": "Event", "value": string(n.Event)},
				},
				"text": n.Message,
			},
		},
	}

	return h.postJSON(ctx, c.Config.TeamsWebhookURL, payload)
}

// sendDiscord sends a Discord notification.
func (h *Hub) sendDiscord(ctx context.Context, c *Channel, n *Notification) error {
	color := 0x0076D7 // Blue
	switch n.Severity {
	case SeverityWarning:
		color = 0xFFA500 // Orange
	case SeverityError, SeverityCritical:
		color = 0xFF0000 // Red
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       n.Title,
				"description": n.Message,
				"color":       color,
				"fields": []map[string]interface{}{
					{"name": "Job", "value": n.JobName, "inline": true},
					{"name": "Severity", "value": string(n.Severity), "inline": true},
				},
				"timestamp": n.Timestamp.Format(time.RFC3339),
			},
		},
	}

	return h.postJSON(ctx, c.Config.DiscordWebhookURL, payload)
}

// sendWebhook sends a generic webhook notification.
func (h *Hub) sendWebhook(ctx context.Context, c *Channel, n *Notification) error {
	return h.postJSON(ctx, c.Config.WebhookURL, n)
}

// sendOpsgenie sends an Opsgenie alert.
func (h *Hub) sendOpsgenie(ctx context.Context, c *Channel, n *Notification) error {
	priority := "P3"
	switch n.Severity {
	case SeverityCritical:
		priority = "P1"
	case SeverityError:
		priority = "P2"
	case SeverityWarning:
		priority = "P3"
	}

	payload := map[string]interface{}{
		"message":  n.Title,
		"priority": priority,
		"details": map[string]interface{}{
			"job_id":       n.JobID,
			"job_name":     n.JobName,
			"execution_id": n.ExecutionID,
			"message":      n.Message,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.opsgenie.com/v2/alerts", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "GenieKey "+c.Config.OpsgenieKey)

	body, _ := json.Marshal(payload)
	req.Body = http.NoBody
	req.Body = newReadCloser(body)

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("opsgenie error: status %d", resp.StatusCode)
	}

	return nil
}

// postJSON posts JSON to a URL.
func (h *Hub) postJSON(ctx context.Context, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error: status %d", resp.StatusCode)
	}

	return nil
}

// GetHistory returns notification delivery history.
func (h *Hub) GetHistory(ctx context.Context, limit int) []DeliveryStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if limit <= 0 || limit > len(h.history) {
		limit = len(h.history)
	}

	// Return most recent first
	start := len(h.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]DeliveryStatus, limit)
	copy(result, h.history[start:])
	return result
}

// newReadCloser creates a ReadCloser from bytes.
func newReadCloser(b []byte) *bytesReadCloser {
	return &bytesReadCloser{Reader: bytes.NewReader(b)}
}

type bytesReadCloser struct {
	*bytes.Reader
}

func (b *bytesReadCloser) Close() error {
	return nil
}
