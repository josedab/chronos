// Package models defines the core data structures for Chronos.
package models

import "errors"

// Common errors.
var (
	ErrJobNotFound        = errors.New("job not found")
	ErrJobNameRequired    = errors.New("job name is required")
	ErrScheduleRequired   = errors.New("schedule is required")
	ErrWebhookRequired    = errors.New("webhook configuration is required")
	ErrWebhookURLRequired = errors.New("webhook URL is required")
	ErrExecutionNotFound  = errors.New("execution not found")
	ErrInvalidCronExpr    = errors.New("invalid cron expression")
	ErrInvalidTimezone    = errors.New("invalid timezone")
	ErrJobAlreadyExists   = errors.New("job already exists")
	ErrNotLeader          = errors.New("not the cluster leader")
	ErrLockNotAcquired    = errors.New("failed to acquire lock")
	ErrJobVersionNotFound = errors.New("job version not found")
	ErrNamespaceNotFound  = errors.New("namespace not found")
)
