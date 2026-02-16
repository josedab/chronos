// Package notify provides the alert evaluation engine for Chronos.
package notify

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AlertRule defines when and how to send notifications.
type AlertRule struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Enabled      bool        `json:"enabled"`
	Events       []EventType `json:"events"`        // Which events trigger this rule
	ChannelIDs   []string    `json:"channel_ids"`    // Which channels to notify
	JobFilter    string      `json:"job_filter,omitempty"`    // Filter by job name pattern (glob)
	Severity     Severity    `json:"severity"`
	CooldownMins int         `json:"cooldown_mins,omitempty"` // Min minutes between alerts for same job
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// AlertEngine evaluates events against rules and dispatches notifications.
type AlertEngine struct {
	hub          *Hub
	rules        map[string]*AlertRule
	lastAlerted  map[string]time.Time // ruleID:jobID -> last alert time
	mu           sync.RWMutex
	deliveryLog  []DeliveryRecord
}

// DeliveryRecord tracks a notification delivery attempt.
type DeliveryRecord struct {
	ID             string    `json:"id"`
	RuleID         string    `json:"rule_id"`
	ChannelID      string    `json:"channel_id"`
	Event          EventType `json:"event"`
	JobID          string    `json:"job_id"`
	Success        bool      `json:"success"`
	Error          string    `json:"error,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewAlertEngine creates a new alert engine.
func NewAlertEngine(hub *Hub) *AlertEngine {
	return &AlertEngine{
		hub:         hub,
		rules:       make(map[string]*AlertRule),
		lastAlerted: make(map[string]time.Time),
	}
}

// AddRule registers an alert rule.
func (ae *AlertEngine) AddRule(rule *AlertRule) error {
	if rule.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if len(rule.Events) == 0 {
		return fmt.Errorf("at least one event type is required")
	}
	if len(rule.ChannelIDs) == 0 {
		return fmt.Errorf("at least one channel ID is required")
	}

	ae.mu.Lock()
	defer ae.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	ae.rules[rule.ID] = rule
	return nil
}

// RemoveRule removes an alert rule.
func (ae *AlertEngine) RemoveRule(ruleID string) error {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	if _, ok := ae.rules[ruleID]; !ok {
		return fmt.Errorf("rule not found: %s", ruleID)
	}
	delete(ae.rules, ruleID)
	return nil
}

// GetRule returns a rule by ID.
func (ae *AlertEngine) GetRule(ruleID string) (*AlertRule, error) {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	rule, ok := ae.rules[ruleID]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", ruleID)
	}
	return rule, nil
}

// ListRules returns all rules.
func (ae *AlertEngine) ListRules() []*AlertRule {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(ae.rules))
	for _, r := range ae.rules {
		rules = append(rules, r)
	}
	return rules
}

// Evaluate checks an event against all rules and dispatches notifications.
func (ae *AlertEngine) Evaluate(ctx context.Context, event EventType, jobID, jobName, executionID, errorMsg string) []DeliveryRecord {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	var records []DeliveryRecord

	for _, rule := range ae.rules {
		if !rule.Enabled {
			continue
		}

		if !ae.matchesEvent(rule, event) {
			continue
		}

		if rule.JobFilter != "" && !matchGlob(rule.JobFilter, jobName) {
			continue
		}

		// Cooldown check
		cooldownKey := rule.ID + ":" + jobID
		if rule.CooldownMins > 0 {
			if last, ok := ae.lastAlerted[cooldownKey]; ok {
				if time.Since(last) < time.Duration(rule.CooldownMins)*time.Minute {
					continue
				}
			}
		}

		// Build notification
		notif := &Notification{
			ID:          uuid.New().String(),
			Event:       event,
			Title:       fmt.Sprintf("[%s] %s: %s", rule.Severity, event, jobName),
			Message:     ae.buildMessage(event, jobName, executionID, errorMsg),
			Severity:    rule.Severity,
			JobID:       jobID,
			JobName:     jobName,
			ExecutionID: executionID,
			Error:       errorMsg,
			Timestamp:   time.Now(),
		}

		// Dispatch to each channel
		for _, channelID := range rule.ChannelIDs {
			notif.ChannelID = channelID
			_, err := ae.hub.Send(ctx, notif)

			record := DeliveryRecord{
				ID:        uuid.New().String(),
				RuleID:    rule.ID,
				ChannelID: channelID,
				Event:     event,
				JobID:     jobID,
				Success:   err == nil,
				Timestamp: time.Now(),
			}
			if err != nil {
				record.Error = err.Error()
			}
			records = append(records, record)
			ae.deliveryLog = append(ae.deliveryLog, record)
		}

		ae.lastAlerted[cooldownKey] = time.Now()
	}

	return records
}

// DeliveryHistory returns recent delivery records.
func (ae *AlertEngine) DeliveryHistory(limit int) []DeliveryRecord {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	if limit <= 0 || limit > len(ae.deliveryLog) {
		limit = len(ae.deliveryLog)
	}
	start := len(ae.deliveryLog) - limit
	if start < 0 {
		start = 0
	}
	result := make([]DeliveryRecord, limit)
	copy(result, ae.deliveryLog[start:])
	return result
}

func (ae *AlertEngine) matchesEvent(rule *AlertRule, event EventType) bool {
	for _, e := range rule.Events {
		if e == event {
			return true
		}
	}
	return false
}

func (ae *AlertEngine) buildMessage(event EventType, jobName, executionID, errorMsg string) string {
	msg := fmt.Sprintf("Job **%s** — %s", jobName, event)
	if executionID != "" {
		msg += fmt.Sprintf("\nExecution: `%s`", executionID)
	}
	if errorMsg != "" {
		msg += fmt.Sprintf("\nError: %s", errorMsg)
	}
	return msg
}

// matchGlob matches a simple glob pattern (supports * wildcards).
func matchGlob(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == s {
		return true
	}
	// Simple prefix/suffix matching for patterns like "prod-*" or "*-cleanup"
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(s) >= len(prefix) && s[:len(prefix)] == prefix
	}
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
	}
	return false
}
