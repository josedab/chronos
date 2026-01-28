// Package policy provides policy-as-code enforcement for Chronos jobs.
package policy

import (
	"time"

	"github.com/google/uuid"
)

// BuiltInPolicies returns a set of commonly used built-in policies.
func BuiltInPolicies() []*Policy {
	return []*Policy{
		webhookHTTPSOnlyPolicy(),
		jobNamingConventionPolicy(),
		timeoutLimitsPolicy(),
		retryLimitsPolicy(),
		webhookAllowlistPolicy(),
		scheduleFrequencyPolicy(),
		tagRequirementPolicy(),
	}
}

// webhookHTTPSOnlyPolicy enforces HTTPS for all webhook URLs.
func webhookHTTPSOnlyPolicy() *Policy {
	return &Policy{
		ID:          "builtin-https-only",
		Name:        "HTTPS Only Webhooks",
		Description: "Ensures all webhook URLs use HTTPS for secure communication",
		Type:        PolicyTypeJob,
		Severity:    SeverityError,
		Action:      ActionDeny,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "https-url",
				Name:        "Webhook URL must use HTTPS",
				Description: "All webhook URLs must start with https://",
				Condition: Condition{
					Field:    "webhook.url",
					Operator: OpStartsWith,
					Value:    "https://",
				},
				Message: "Webhook URL must use HTTPS protocol for security",
			},
		},
		Metadata: map[string]string{
			"category": "security",
			"builtin":  "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// jobNamingConventionPolicy enforces job naming conventions.
func jobNamingConventionPolicy() *Policy {
	return &Policy{
		ID:          "builtin-naming-convention",
		Name:        "Job Naming Convention",
		Description: "Enforces consistent job naming with lowercase alphanumeric and hyphens",
		Type:        PolicyTypeJob,
		Severity:    SeverityWarning,
		Action:      ActionWarn,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "lowercase-name",
				Name:        "Job name must follow naming convention",
				Description: "Job names should be lowercase alphanumeric with hyphens",
				Condition: Condition{
					Field:    "name",
					Operator: OpMatches,
					Value:    "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
				},
				Message: "Job name should be lowercase alphanumeric with hyphens (e.g., 'my-daily-job')",
			},
		},
		Metadata: map[string]string{
			"category": "naming",
			"builtin":  "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// timeoutLimitsPolicy enforces reasonable timeout limits.
func timeoutLimitsPolicy() *Policy {
	return &Policy{
		ID:          "builtin-timeout-limits",
		Name:        "Timeout Limits",
		Description: "Ensures job timeouts are within acceptable limits (1s to 30m)",
		Type:        PolicyTypeJob,
		Severity:    SeverityError,
		Action:      ActionDeny,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "min-timeout",
				Name:        "Minimum timeout",
				Description: "Timeout must be at least 1 second",
				Condition: Condition{
					Field:    "timeout_seconds",
					Operator: OpGreaterOrEqual,
					Value:    1.0,
				},
				Message: "Job timeout must be at least 1 second",
			},
			{
				ID:          "max-timeout",
				Name:        "Maximum timeout",
				Description: "Timeout must not exceed 30 minutes",
				Condition: Condition{
					Field:    "timeout_seconds",
					Operator: OpLessOrEqual,
					Value:    1800.0, // 30 minutes
				},
				Message: "Job timeout must not exceed 30 minutes (1800 seconds)",
			},
		},
		Metadata: map[string]string{
			"category": "limits",
			"builtin":  "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// retryLimitsPolicy enforces reasonable retry limits.
func retryLimitsPolicy() *Policy {
	return &Policy{
		ID:          "builtin-retry-limits",
		Name:        "Retry Limits",
		Description: "Ensures retry attempts are within acceptable limits (1-10)",
		Type:        PolicyTypeJob,
		Severity:    SeverityError,
		Action:      ActionDeny,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "max-retries",
				Name:        "Maximum retry attempts",
				Description: "Retry attempts must not exceed 10",
				Condition: Condition{
					Field:    "retry_policy.max_attempts",
					Operator: OpLessOrEqual,
					Value:    10.0,
				},
				Message: "Retry attempts must not exceed 10 to prevent excessive load",
			},
		},
		Metadata: map[string]string{
			"category": "limits",
			"builtin":  "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// webhookAllowlistPolicy enforces webhook URL allowlisting.
func webhookAllowlistPolicy() *Policy {
	return &Policy{
		ID:          "builtin-webhook-allowlist",
		Name:        "Webhook URL Allowlist",
		Description: "Restricts webhook URLs to approved domains (disabled by default)",
		Type:        PolicyTypeJob,
		Severity:    SeverityError,
		Action:      ActionDeny,
		Enabled:     false, // Disabled by default, requires configuration
		Rules: []Rule{
			{
				ID:          "domain-allowlist",
				Name:        "Webhook domain must be in allowlist",
				Description: "Webhook URLs must be from approved domains",
				Condition: Condition{
					Or: []Condition{
						{Field: "webhook.url", Operator: OpContains, Value: ".internal.company.com"},
						{Field: "webhook.url", Operator: OpContains, Value: "api.company.com"},
					},
				},
				Message: "Webhook URL must be from an approved domain",
			},
		},
		Metadata: map[string]string{
			"category":      "security",
			"builtin":       "true",
			"configurable":  "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// scheduleFrequencyPolicy prevents overly frequent schedules.
func scheduleFrequencyPolicy() *Policy {
	return &Policy{
		ID:          "builtin-schedule-frequency",
		Name:        "Schedule Frequency Limits",
		Description: "Prevents jobs from running more frequently than every minute",
		Type:        PolicyTypeJob,
		Severity:    SeverityWarning,
		Action:      ActionWarn,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "min-interval",
				Name:        "Minimum schedule interval",
				Description: "Jobs should not run more frequently than every minute",
				Condition: Condition{
					Field:    "schedule",
					Operator: OpNotEquals,
					Value:    "* * * * * *", // every second
				},
				Message: "Job schedule is very frequent. Consider if this frequency is necessary.",
			},
		},
		Metadata: map[string]string{
			"category": "limits",
			"builtin":  "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// tagRequirementPolicy requires certain tags on jobs.
func tagRequirementPolicy() *Policy {
	return &Policy{
		ID:          "builtin-tag-requirements",
		Name:        "Required Tags",
		Description: "Ensures jobs have required tags (disabled by default)",
		Type:        PolicyTypeJob,
		Severity:    SeverityWarning,
		Action:      ActionWarn,
		Enabled:     false, // Disabled by default
		Rules: []Rule{
			{
				ID:          "team-tag",
				Name:        "Team tag required",
				Description: "Jobs should have a 'team' tag for ownership",
				Condition: Condition{
					Field:    "tags.team",
					Operator: OpExists,
					Value:    nil,
				},
				Message: "Job should have a 'team' tag for ownership tracking",
			},
			{
				ID:          "environment-tag",
				Name:        "Environment tag required",
				Description: "Jobs should have an 'environment' tag",
				Condition: Condition{
					Field:    "tags.environment",
					Operator: OpExists,
					Value:    nil,
				},
				Message: "Job should have an 'environment' tag (e.g., production, staging)",
			},
		},
		Metadata: map[string]string{
			"category":     "governance",
			"builtin":      "true",
			"configurable": "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// CreateCustomPolicy creates a custom policy with the given parameters.
func CreateCustomPolicy(name, description string, policyType PolicyType, severity Severity, action Action, rules []Rule) *Policy {
	return &Policy{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Type:        policyType,
		Severity:    severity,
		Action:      action,
		Enabled:     true,
		Rules:       rules,
		Metadata:    map[string]string{"custom": "true"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// CreateWebhookDomainPolicy creates a policy restricting webhook URLs to specific domains.
func CreateWebhookDomainPolicy(allowedDomains []string) *Policy {
	conditions := make([]Condition, 0, len(allowedDomains))
	for _, domain := range allowedDomains {
		conditions = append(conditions, Condition{
			Field:    "webhook.url",
			Operator: OpContains,
			Value:    domain,
		})
	}

	return &Policy{
		ID:          uuid.New().String(),
		Name:        "Custom Webhook Domain Allowlist",
		Description: "Restricts webhook URLs to configured domains",
		Type:        PolicyTypeJob,
		Severity:    SeverityError,
		Action:      ActionDeny,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "custom-domain-allowlist",
				Name:        "Webhook domain must be in allowlist",
				Description: "Webhook URLs must be from approved domains",
				Condition: Condition{
					Or: conditions,
				},
				Message: "Webhook URL must be from an approved domain",
			},
		},
		Metadata: map[string]string{
			"category": "security",
			"custom":   "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// CreateTimeWindowPolicy creates a policy restricting job execution to specific time windows.
func CreateTimeWindowPolicy(allowedHours []int, allowedDays []int, timezone string) *Policy {
	return &Policy{
		ID:          uuid.New().String(),
		Name:        "Execution Time Window",
		Description: "Restricts job execution to specific hours and days",
		Type:        PolicyTypeExecution,
		Severity:    SeverityError,
		Action:      ActionDeny,
		Enabled:     true,
		Rules: []Rule{
			{
				ID:          "allowed-hours",
				Name:        "Execution must be within allowed hours",
				Description: "Jobs can only execute during specified hours",
				Condition: Condition{
					Field:    "execution_hour",
					Operator: OpIn,
					Value:    toInterfaceSlice(allowedHours),
				},
				Message: "Job execution is not allowed during this hour",
			},
			{
				ID:          "allowed-days",
				Name:        "Execution must be on allowed days",
				Description: "Jobs can only execute on specified days",
				Condition: Condition{
					Field:    "execution_day",
					Operator: OpIn,
					Value:    toInterfaceSlice(allowedDays),
				},
				Message: "Job execution is not allowed on this day",
			},
		},
		Metadata: map[string]string{
			"category": "scheduling",
			"timezone": timezone,
			"custom":   "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// toInterfaceSlice converts an int slice to interface slice.
func toInterfaceSlice(ints []int) []interface{} {
	result := make([]interface{}, len(ints))
	for i, v := range ints {
		result[i] = v
	}
	return result
}
