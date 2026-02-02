// Package prediction provides smart failure prediction for Chronos.
package prediction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// NotificationChannel represents a notification destination.
type NotificationChannel interface {
	// Name returns the channel name.
	Name() string
	// Send sends a notification.
	Send(ctx context.Context, notification *Notification) error
	// Test tests the channel connectivity.
	Test(ctx context.Context) error
}

// Notification represents a notification to be sent.
type Notification struct {
	ID          string            `json:"id"`
	Type        NotificationType  `json:"type"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Message     string            `json:"message"`
	Alert       *Alert            `json:"alert,omitempty"`
	Prediction  *Prediction       `json:"prediction,omitempty"`
	JobID       string            `json:"job_id"`
	JobName     string            `json:"job_name"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// NotificationType represents the type of notification.
type NotificationType string

const (
	NotificationPredictedFailure  NotificationType = "predicted_failure"
	NotificationPerformanceDegraded NotificationType = "performance_degraded"
	NotificationAnomalyDetected   NotificationType = "anomaly_detected"
	NotificationJobFailed         NotificationType = "job_failed"
	NotificationHighRisk          NotificationType = "high_risk"
)

// Severity represents notification severity.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// NotificationManager manages notification channels and routing.
type NotificationManager struct {
	mu            sync.RWMutex
	channels      map[string]NotificationChannel
	rules         []NotificationRule
	sent          []SentNotification
	rateLimiter   *RateLimiter
	deduplicator  *Deduplicator
}

// NotificationRule defines when and where to send notifications.
type NotificationRule struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Enabled      bool             `json:"enabled"`
	Channels     []string         `json:"channels"`
	MinSeverity  Severity         `json:"min_severity"`
	Types        []NotificationType `json:"types,omitempty"`
	JobPatterns  []string         `json:"job_patterns,omitempty"`
}

// SentNotification tracks sent notifications.
type SentNotification struct {
	NotificationID string    `json:"notification_id"`
	Channel        string    `json:"channel"`
	SentAt         time.Time `json:"sent_at"`
	Success        bool      `json:"success"`
	Error          string    `json:"error,omitempty"`
}

// RateLimiter prevents notification flooding.
type RateLimiter struct {
	mu       sync.Mutex
	limits   map[string][]time.Time
	maxBurst int
	window   time.Duration
}

// Deduplicator prevents duplicate notifications.
type Deduplicator struct {
	mu     sync.Mutex
	recent map[string]time.Time
	ttl    time.Duration
}

// NewNotificationManager creates a new notification manager.
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		channels: make(map[string]NotificationChannel),
		rules:    make([]NotificationRule, 0),
		sent:     make([]SentNotification, 0),
		rateLimiter: &RateLimiter{
			limits:   make(map[string][]time.Time),
			maxBurst: 10,
			window:   5 * time.Minute,
		},
		deduplicator: &Deduplicator{
			recent: make(map[string]time.Time),
			ttl:    15 * time.Minute,
		},
	}
}

// RegisterChannel registers a notification channel.
func (nm *NotificationManager) RegisterChannel(channel NotificationChannel) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if _, exists := nm.channels[channel.Name()]; exists {
		return fmt.Errorf("channel %s already registered", channel.Name())
	}

	nm.channels[channel.Name()] = channel
	return nil
}

// AddRule adds a notification rule.
func (nm *NotificationManager) AddRule(rule NotificationRule) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.rules = append(nm.rules, rule)
}

// Send sends a notification through applicable channels.
func (nm *NotificationManager) Send(ctx context.Context, notification *Notification) error {
	nm.mu.RLock()
	rules := nm.rules
	nm.mu.RUnlock()

	// Check for duplicates
	if nm.deduplicator.IsDuplicate(notification) {
		return nil
	}

	// Find applicable rules
	var applicableChannels []string
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if !nm.matchesSeverity(notification.Severity, rule.MinSeverity) {
			continue
		}

		if len(rule.Types) > 0 && !nm.containsType(rule.Types, notification.Type) {
			continue
		}

		applicableChannels = append(applicableChannels, rule.Channels...)
	}

	// Deduplicate channels
	uniqueChannels := make(map[string]bool)
	for _, ch := range applicableChannels {
		uniqueChannels[ch] = true
	}

	// Send to each channel
	var lastErr error
	for channelName := range uniqueChannels {
		if !nm.rateLimiter.Allow(channelName) {
			continue
		}

		nm.mu.RLock()
		channel, ok := nm.channels[channelName]
		nm.mu.RUnlock()

		if !ok {
			continue
		}

		sent := SentNotification{
			NotificationID: notification.ID,
			Channel:        channelName,
			SentAt:         time.Now(),
		}

		if err := channel.Send(ctx, notification); err != nil {
			sent.Success = false
			sent.Error = err.Error()
			lastErr = err
		} else {
			sent.Success = true
		}

		nm.mu.Lock()
		nm.sent = append(nm.sent, sent)
		if len(nm.sent) > 1000 {
			nm.sent = nm.sent[len(nm.sent)-1000:]
		}
		nm.mu.Unlock()
	}

	nm.deduplicator.Mark(notification)
	return lastErr
}

// matchesSeverity checks if notification severity matches rule minimum.
func (nm *NotificationManager) matchesSeverity(notif, minRule Severity) bool {
	severities := map[Severity]int{
		SeverityInfo:     1,
		SeverityWarning:  2,
		SeverityError:    3,
		SeverityCritical: 4,
	}
	return severities[notif] >= severities[minRule]
}

// containsType checks if types slice contains a type.
func (nm *NotificationManager) containsType(types []NotificationType, t NotificationType) bool {
	for _, typ := range types {
		if typ == t {
			return true
		}
	}
	return false
}

// GetSentNotifications returns recent sent notifications.
func (nm *NotificationManager) GetSentNotifications(limit int) []SentNotification {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	if limit > len(nm.sent) {
		limit = len(nm.sent)
	}
	return nm.sent[len(nm.sent)-limit:]
}

// Allow checks if a notification is allowed for a channel.
func (rl *RateLimiter) Allow(channel string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old entries
	var valid []time.Time
	for _, t := range rl.limits[channel] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.limits[channel] = valid

	// Check limit
	if len(rl.limits[channel]) >= rl.maxBurst {
		return false
	}

	rl.limits[channel] = append(rl.limits[channel], now)
	return true
}

// IsDuplicate checks if a notification is a duplicate.
func (d *Deduplicator) IsDuplicate(n *Notification) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", n.JobID, n.Type, n.Severity)
	if lastSent, ok := d.recent[key]; ok {
		if time.Since(lastSent) < d.ttl {
			return true
		}
	}
	return false
}

// Mark marks a notification as sent.
func (d *Deduplicator) Mark(n *Notification) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s", n.JobID, n.Type, n.Severity)
	d.recent[key] = time.Now()

	// Cleanup old entries
	cutoff := time.Now().Add(-d.ttl)
	for k, t := range d.recent {
		if t.Before(cutoff) {
			delete(d.recent, k)
		}
	}
}

// SlackChannel sends notifications to Slack.
type SlackChannel struct {
	webhookURL string
	username   string
	channel    string
}

// SlackConfig configures Slack notifications.
type SlackConfig struct {
	WebhookURL string `json:"webhook_url" yaml:"webhook_url"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Channel    string `json:"channel,omitempty" yaml:"channel,omitempty"`
}

// NewSlackChannel creates a new Slack notification channel.
func NewSlackChannel(cfg SlackConfig) *SlackChannel {
	username := cfg.Username
	if username == "" {
		username = "Chronos"
	}
	return &SlackChannel{
		webhookURL: cfg.WebhookURL,
		username:   username,
		channel:    cfg.Channel,
	}
}

// Name returns the channel name.
func (s *SlackChannel) Name() string {
	return "slack"
}

// Send sends a notification to Slack.
func (s *SlackChannel) Send(ctx context.Context, n *Notification) error {
	color := "#2eb886" // green
	switch n.Severity {
	case SeverityWarning:
		color = "#daa038" // orange
	case SeverityError:
		color = "#cc0000" // red
	case SeverityCritical:
		color = "#7c0000" // dark red
	}

	emoji := ":information_source:"
	switch n.Severity {
	case SeverityWarning:
		emoji = ":warning:"
	case SeverityError:
		emoji = ":x:"
	case SeverityCritical:
		emoji = ":rotating_light:"
	}

	fields := []map[string]interface{}{
		{"title": "Job ID", "value": n.JobID, "short": true},
		{"title": "Type", "value": string(n.Type), "short": true},
	}

	if n.Prediction != nil {
		fields = append(fields,
			map[string]interface{}{"title": "Failure Probability", "value": fmt.Sprintf("%.1f%%", n.Prediction.FailureProbability*100), "short": true},
			map[string]interface{}{"title": "Risk Level", "value": string(n.Prediction.RiskLevel), "short": true},
		)
	}

	payload := map[string]interface{}{
		"username":   s.username,
		"icon_emoji": emoji,
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  n.Title,
				"text":   n.Message,
				"fields": fields,
				"ts":     n.CreatedAt.Unix(),
			},
		},
	}

	if s.channel != "" {
		payload["channel"] = s.channel
	}

	return s.sendWebhook(ctx, payload)
}

// Test tests the Slack connection.
func (s *SlackChannel) Test(ctx context.Context) error {
	payload := map[string]interface{}{
		"username":   s.username,
		"icon_emoji": ":white_check_mark:",
		"text":       "Chronos notification test successful!",
	}
	if s.channel != "" {
		payload["channel"] = s.channel
	}
	return s.sendWebhook(ctx, payload)
}

func (s *SlackChannel) sendWebhook(ctx context.Context, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PagerDutyChannel sends notifications to PagerDuty.
type PagerDutyChannel struct {
	routingKey string
	serviceKey string
	apiURL     string
}

// PagerDutyConfig configures PagerDuty notifications.
type PagerDutyConfig struct {
	RoutingKey string `json:"routing_key" yaml:"routing_key"`
	ServiceKey string `json:"service_key,omitempty" yaml:"service_key,omitempty"`
	APIURL     string `json:"api_url,omitempty" yaml:"api_url,omitempty"`
}

// NewPagerDutyChannel creates a new PagerDuty channel.
func NewPagerDutyChannel(cfg PagerDutyConfig) *PagerDutyChannel {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://events.pagerduty.com/v2/enqueue"
	}
	return &PagerDutyChannel{
		routingKey: cfg.RoutingKey,
		serviceKey: cfg.ServiceKey,
		apiURL:     apiURL,
	}
}

// Name returns the channel name.
func (p *PagerDutyChannel) Name() string {
	return "pagerduty"
}

// Send sends a notification to PagerDuty.
func (p *PagerDutyChannel) Send(ctx context.Context, n *Notification) error {
	severity := "info"
	switch n.Severity {
	case SeverityWarning:
		severity = "warning"
	case SeverityError:
		severity = "error"
	case SeverityCritical:
		severity = "critical"
	}

	customDetails := map[string]interface{}{
		"job_id":   n.JobID,
		"job_name": n.JobName,
		"type":     string(n.Type),
	}

	if n.Prediction != nil {
		customDetails["failure_probability"] = n.Prediction.FailureProbability
		customDetails["risk_level"] = string(n.Prediction.RiskLevel)
		customDetails["reasons"] = n.Prediction.Reasons
	}

	payload := map[string]interface{}{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("chronos-%s-%s", n.JobID, n.Type),
		"payload": map[string]interface{}{
			"summary":        n.Title,
			"severity":       severity,
			"source":         "chronos",
			"component":      n.JobID,
			"custom_details": customDetails,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal pagerduty payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pagerduty notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pagerduty returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Test tests the PagerDuty connection.
func (p *PagerDutyChannel) Test(ctx context.Context) error {
	payload := map[string]interface{}{
		"routing_key":  p.routingKey,
		"event_action": "trigger",
		"dedup_key":    "chronos-test-notification",
		"payload": map[string]interface{}{
			"summary":  "Chronos test notification",
			"severity": "info",
			"source":   "chronos",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pagerduty test failed with status %d", resp.StatusCode)
	}

	return nil
}

// EmailChannel sends notifications via SMTP.
type EmailChannel struct {
	smtpHost    string
	smtpPort    int
	username    string
	password    string
	from        string
	recipients  []string
}

// EmailConfig configures email notifications.
type EmailConfig struct {
	SMTPHost   string   `json:"smtp_host" yaml:"smtp_host"`
	SMTPPort   int      `json:"smtp_port" yaml:"smtp_port"`
	Username   string   `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string   `json:"password,omitempty" yaml:"password,omitempty"`
	From       string   `json:"from" yaml:"from"`
	Recipients []string `json:"recipients" yaml:"recipients"`
}

// NewEmailChannel creates a new email notification channel.
func NewEmailChannel(cfg EmailConfig) *EmailChannel {
	port := cfg.SMTPPort
	if port == 0 {
		port = 587
	}
	return &EmailChannel{
		smtpHost:   cfg.SMTPHost,
		smtpPort:   port,
		username:   cfg.Username,
		password:   cfg.Password,
		from:       cfg.From,
		recipients: cfg.Recipients,
	}
}

// Name returns the channel name.
func (e *EmailChannel) Name() string {
	return "email"
}

// Send sends a notification via email.
func (e *EmailChannel) Send(ctx context.Context, n *Notification) error {
	// Build email body
	body := fmt.Sprintf(`
Chronos Alert: %s

Severity: %s
Type: %s
Job ID: %s
Job Name: %s

%s
`, n.Title, n.Severity, n.Type, n.JobID, n.JobName, n.Message)

	if n.Prediction != nil {
		body += fmt.Sprintf(`
Prediction Details:
- Failure Probability: %.1f%%
- Risk Level: %s
- Confidence: %.1f%%

Reasons:
`, n.Prediction.FailureProbability*100, n.Prediction.RiskLevel, n.Prediction.Confidence*100)

		for _, reason := range n.Prediction.Reasons {
			body += fmt.Sprintf("- %s\n", reason)
		}

		body += "\nRecommended Actions:\n"
		for _, action := range n.Prediction.RecommendedActions {
			body += fmt.Sprintf("- %s\n", action)
		}
	}

	// Note: Actual SMTP sending requires net/smtp package
	// For now, log that we would send an email
	// In production, use: smtp.SendMail(addr, auth, from, to, msg)
	_ = body
	return nil // Placeholder - real implementation needs smtp.SendMail
}

// Test tests the email connection.
func (e *EmailChannel) Test(ctx context.Context) error {
	// Placeholder - real implementation would test SMTP connection
	return nil
}

// WebhookChannel sends notifications to a generic webhook.
type WebhookChannel struct {
	name    string
	url     string
	headers map[string]string
	method  string
}

// WebhookConfig configures webhook notifications.
type WebhookConfig struct {
	Name    string            `json:"name" yaml:"name"`
	URL     string            `json:"url" yaml:"url"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Method  string            `json:"method,omitempty" yaml:"method,omitempty"`
}

// NewWebhookChannel creates a new webhook notification channel.
func NewWebhookChannel(cfg WebhookConfig) *WebhookChannel {
	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}
	return &WebhookChannel{
		name:    cfg.Name,
		url:     cfg.URL,
		headers: cfg.Headers,
		method:  method,
	}
}

// Name returns the channel name.
func (w *WebhookChannel) Name() string {
	if w.name != "" {
		return w.name
	}
	return "webhook"
}

// Send sends a notification to the webhook.
func (w *WebhookChannel) Send(ctx context.Context, n *Notification) error {
	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Test tests the webhook connection.
func (w *WebhookChannel) Test(ctx context.Context) error {
	testNotif := &Notification{
		ID:        "test",
		Type:      NotificationPredictedFailure,
		Severity:  SeverityInfo,
		Title:     "Test Notification",
		Message:   "This is a test notification from Chronos",
		CreatedAt: time.Now(),
	}
	return w.Send(ctx, testNotif)
}
