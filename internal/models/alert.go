// Package models defines the core data structures for Chronos.
package models

import "time"

// AlertChannel represents a notification channel for alerts.
type AlertChannel struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      AlertChannelType  `json:"type"`
	Enabled   bool              `json:"enabled"`
	Config    map[string]string `json:"config"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// AlertChannelType defines the type of notification channel.
type AlertChannelType string

const (
	AlertChannelSlack     AlertChannelType = "slack"
	AlertChannelEmail     AlertChannelType = "email"
	AlertChannelPagerDuty AlertChannelType = "pagerduty"
	AlertChannelWebhook   AlertChannelType = "webhook"
)

// AlertRule defines when and how to send alerts.
type AlertRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Enabled     bool           `json:"enabled"`
	Conditions  AlertCondition `json:"conditions"`
	ChannelIDs  []string       `json:"channel_ids"`
	JobFilter   *JobFilter     `json:"job_filter,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// AlertCondition defines the conditions that trigger an alert.
type AlertCondition struct {
	OnFailure        bool `json:"on_failure"`
	OnSuccess        bool `json:"on_success"`
	OnTimeout        bool `json:"on_timeout"`
	OnRecovery       bool `json:"on_recovery"` // Alert when job recovers from failure
	ConsecutiveFails int  `json:"consecutive_fails,omitempty"`
}

// JobFilter filters which jobs a rule applies to.
type JobFilter struct {
	JobIDs     []string          `json:"job_ids,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Namespaces []string          `json:"namespaces,omitempty"`
}

// Validate validates the alert channel configuration.
func (c *AlertChannel) Validate() error {
	if c.Name == "" {
		return ErrAlertChannelNameRequired
	}
	if c.Type == "" {
		return ErrAlertChannelTypeRequired
	}
	switch c.Type {
	case AlertChannelSlack:
		if c.Config["webhook_url"] == "" {
			return ErrSlackWebhookRequired
		}
	case AlertChannelEmail:
		if c.Config["recipients"] == "" {
			return ErrEmailRecipientsRequired
		}
	case AlertChannelPagerDuty:
		if c.Config["routing_key"] == "" {
			return ErrPagerDutyKeyRequired
		}
	case AlertChannelWebhook:
		if c.Config["url"] == "" {
			return ErrWebhookURLRequired
		}
	default:
		return ErrInvalidAlertChannelType
	}
	return nil
}

// Validate validates the alert rule configuration.
func (r *AlertRule) Validate() error {
	if r.Name == "" {
		return ErrAlertRuleNameRequired
	}
	if len(r.ChannelIDs) == 0 {
		return ErrAlertRuleChannelsRequired
	}
	if !r.Conditions.OnFailure && !r.Conditions.OnSuccess && !r.Conditions.OnTimeout && !r.Conditions.OnRecovery {
		return ErrAlertRuleConditionsRequired
	}
	return nil
}
