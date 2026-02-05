// Package models defines the core data structures for Chronos.
package models

import "errors"

// Common errors.
var (
	ErrJobNotFound           = errors.New("job not found")
	ErrJobNameRequired       = errors.New("job name is required")
	ErrScheduleRequired      = errors.New("schedule is required")
	ErrWebhookRequired       = errors.New("webhook configuration is required")
	ErrWebhookURLRequired    = errors.New("webhook URL is required")
	ErrExecutionNotFound     = errors.New("execution not found")
	ErrInvalidCronExpr       = errors.New("invalid cron expression")
	ErrInvalidTimezone       = errors.New("invalid timezone")
	ErrJobAlreadyExists      = errors.New("job already exists")
	ErrNotLeader             = errors.New("not the cluster leader")
	ErrLockNotAcquired       = errors.New("failed to acquire lock")
	ErrJobVersionNotFound    = errors.New("job version not found")
	ErrNamespaceNotFound     = errors.New("namespace not found")
	ErrScheduleStateNotFound = errors.New("schedule state not found")
	ErrVersionNotFound       = errors.New("version not found")

	// Alert errors
	ErrAlertChannelNotFound       = errors.New("alert channel not found")
	ErrAlertChannelNameRequired   = errors.New("alert channel name is required")
	ErrAlertChannelTypeRequired   = errors.New("alert channel type is required")
	ErrInvalidAlertChannelType    = errors.New("invalid alert channel type")
	ErrSlackWebhookRequired       = errors.New("slack webhook URL is required")
	ErrEmailRecipientsRequired    = errors.New("email recipients are required")
	ErrPagerDutyKeyRequired       = errors.New("pagerduty routing key is required")
	ErrAlertRuleNotFound          = errors.New("alert rule not found")
	ErrAlertRuleNameRequired      = errors.New("alert rule name is required")
	ErrAlertRuleChannelsRequired  = errors.New("alert rule must have at least one channel")
	ErrAlertRuleConditionsRequired = errors.New("alert rule must have at least one condition enabled")
)
