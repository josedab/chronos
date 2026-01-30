// Package marketplace provides built-in job templates.
package marketplace

import (
	"time"
)

// BuiltInTemplates returns all built-in marketplace templates.
func BuiltInTemplates() []*Template {
	return []*Template{
		slackNotificationTemplate(),
		databaseBackupTemplate(),
		healthCheckTemplate(),
		reportGeneratorTemplate(),
		cacheCleanupTemplate(),
		certificateExpiryCheckTemplate(),
		logRotationTemplate(),
		dataRetentionTemplate(),
		slackDailyDigestTemplate(),
		webhookPingTemplate(),
		elasticsearchCleanupTemplate(),
		redisFlushTemplate(),
		gitHubActionsTemplate(),
		datadogMetricsTemplate(),
		pagerDutyHeartbeatTemplate(),
	}
}

// slackNotificationTemplate provides a Slack notification job template.
func slackNotificationTemplate() *Template {
	return &Template{
		ID:          "slack-notification",
		Name:        "Slack Notification",
		Description: "Send scheduled messages to a Slack channel via webhook",
		Category:    CategoryNotification,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"slack", "notification", "messaging", "alerts"},
		Icon:        "💬",
		Schedule:    "0 9 * * 1-5",
		Timezone:    "America/New_York",
		Webhook: WebhookTemplate{
			URL:    "{{SLACK_WEBHOOK_URL}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"text": "{{MESSAGE}}", "channel": "{{CHANNEL}}", "username": "Chronos Bot", "icon_emoji": ":clock1:"}`,
		},
		Timeout:     "30s",
		Concurrency: "forbid",
		RetryPolicy: &RetryPolicyTemplate{
			MaxAttempts:     3,
			InitialInterval: "5s",
			MaxInterval:     "30s",
			Multiplier:      2.0,
		},
		Variables: []TemplateVariable{
			{
				Name:        "SLACK_WEBHOOK_URL",
				Description: "Slack incoming webhook URL",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "MESSAGE",
				Description: "Message to send",
				Type:        "string",
				Required:    true,
				Default:     "Hello from Chronos!",
			},
			{
				Name:        "CHANNEL",
				Description: "Slack channel (e.g., #general)",
				Type:        "string",
				Required:    false,
				Default:     "#general",
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Readme: `# Slack Notification Template

Send scheduled messages to Slack channels using incoming webhooks.

## Setup

1. Create a Slack incoming webhook at https://api.slack.com/messaging/webhooks
2. Copy the webhook URL
3. Configure the template variables

## Variables

- **SLACK_WEBHOOK_URL**: Your Slack webhook URL (keep this secret!)
- **MESSAGE**: The message content to send
- **CHANNEL**: Override the default channel (optional)

## Examples

- Daily standup reminder
- Weekly report notifications
- System status updates
`,
		Examples: []Example{
			{
				Name:        "Daily Standup Reminder",
				Description: "Remind team about daily standup at 9:45 AM",
				Variables: map[string]string{
					"MESSAGE": "🕐 Daily standup in 15 minutes!",
					"CHANNEL": "#engineering",
				},
			},
		},
	}
}

// databaseBackupTemplate provides a database backup trigger template.
func databaseBackupTemplate() *Template {
	return &Template{
		ID:          "database-backup",
		Name:        "Database Backup Trigger",
		Description: "Trigger database backup via webhook endpoint",
		Category:    CategoryBackup,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"database", "backup", "postgres", "mysql", "disaster-recovery"},
		Icon:        "💾",
		Schedule:    "0 2 * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{BACKUP_ENDPOINT}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"database": "{{DATABASE_NAME}}", "type": "{{BACKUP_TYPE}}", "retention_days": {{RETENTION_DAYS}}}`,
		},
		Timeout:     "30m",
		Concurrency: "forbid",
		RetryPolicy: &RetryPolicyTemplate{
			MaxAttempts:     2,
			InitialInterval: "1m",
			MaxInterval:     "5m",
			Multiplier:      2.0,
		},
		Variables: []TemplateVariable{
			{
				Name:        "BACKUP_ENDPOINT",
				Description: "Backup service API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "DATABASE_NAME",
				Description: "Name of the database to backup",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "BACKUP_TYPE",
				Description: "Type of backup (full, incremental)",
				Type:        "string",
				Required:    false,
				Default:     "full",
			},
			{
				Name:        "RETENTION_DAYS",
				Description: "Number of days to retain backup",
				Type:        "number",
				Required:    false,
				Default:     30,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Readme: `# Database Backup Trigger

Automatically trigger database backups on a schedule.

## Supported Databases

- PostgreSQL
- MySQL/MariaDB
- MongoDB
- Any database with HTTP backup API

## Best Practices

- Schedule backups during low-traffic periods
- Test restore procedures regularly
- Monitor backup completion
`,
	}
}

// healthCheckTemplate provides a health check monitoring template.
func healthCheckTemplate() *Template {
	return &Template{
		ID:          "health-check",
		Name:        "Endpoint Health Check",
		Description: "Monitor endpoint availability with periodic health checks",
		Category:    CategoryMonitoring,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"health", "monitoring", "uptime", "availability"},
		Icon:        "🏥",
		Schedule:    "*/5 * * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{HEALTH_ENDPOINT}}",
			Method: "GET",
			Headers: map[string]string{
				"User-Agent": "Chronos-HealthCheck/1.0",
			},
		},
		Timeout:     "30s",
		Concurrency: "allow",
		RetryPolicy: &RetryPolicyTemplate{
			MaxAttempts:     3,
			InitialInterval: "5s",
			MaxInterval:     "15s",
			Multiplier:      2.0,
		},
		Variables: []TemplateVariable{
			{
				Name:        "HEALTH_ENDPOINT",
				Description: "URL of the health check endpoint",
				Type:        "string",
				Required:    true,
				Validation:  "^https?://.*",
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Readme: `# Endpoint Health Check

Monitor your services with periodic health checks.

## Features

- Configurable check interval
- Automatic retries on failure
- Tracks response times

## Usage

Set the HEALTH_ENDPOINT to your service's health check URL (e.g., https://api.example.com/health)
`,
	}
}

// reportGeneratorTemplate provides a report generation template.
func reportGeneratorTemplate() *Template {
	return &Template{
		ID:          "report-generator",
		Name:        "Report Generator",
		Description: "Trigger periodic report generation",
		Category:    CategoryReporting,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"reports", "analytics", "data", "business-intelligence"},
		Icon:        "📊",
		Schedule:    "0 6 * * 1",
		Timezone:    "America/New_York",
		Webhook: WebhookTemplate{
			URL:    "{{REPORT_API}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"report_type": "{{REPORT_TYPE}}", "format": "{{FORMAT}}", "recipients": [{{RECIPIENTS}}], "date_range": "{{DATE_RANGE}}"}`,
		},
		Timeout:     "10m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "REPORT_API",
				Description: "Report generation API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "REPORT_TYPE",
				Description: "Type of report to generate",
				Type:        "string",
				Required:    true,
				Default:     "weekly-summary",
			},
			{
				Name:        "FORMAT",
				Description: "Output format (pdf, csv, xlsx)",
				Type:        "string",
				Required:    false,
				Default:     "pdf",
			},
			{
				Name:        "RECIPIENTS",
				Description: "Email recipients (comma-separated, quoted)",
				Type:        "string",
				Required:    false,
				Default:     `"team@example.com"`,
			},
			{
				Name:        "DATE_RANGE",
				Description: "Date range for the report",
				Type:        "string",
				Required:    false,
				Default:     "last_7_days",
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// cacheCleanupTemplate provides a cache cleanup template.
func cacheCleanupTemplate() *Template {
	return &Template{
		ID:          "cache-cleanup",
		Name:        "Cache Cleanup",
		Description: "Periodically clean up expired cache entries",
		Category:    CategoryMaintenance,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"cache", "cleanup", "maintenance", "redis", "memcached"},
		Icon:        "🧹",
		Schedule:    "0 3 * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{CLEANUP_ENDPOINT}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"action": "cleanup", "max_age_hours": {{MAX_AGE_HOURS}}, "patterns": [{{PATTERNS}}]}`,
		},
		Timeout:     "15m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "CLEANUP_ENDPOINT",
				Description: "Cache cleanup API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "MAX_AGE_HOURS",
				Description: "Maximum age of cache entries in hours",
				Type:        "number",
				Required:    false,
				Default:     24,
			},
			{
				Name:        "PATTERNS",
				Description: "Cache key patterns to clean (quoted, comma-separated)",
				Type:        "string",
				Required:    false,
				Default:     `"*"`,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// certificateExpiryCheckTemplate provides a certificate expiry monitoring template.
func certificateExpiryCheckTemplate() *Template {
	return &Template{
		ID:          "certificate-expiry-check",
		Name:        "SSL Certificate Expiry Check",
		Description: "Monitor SSL certificate expiration dates",
		Category:    CategorySecurity,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"ssl", "tls", "certificate", "security", "expiry"},
		Icon:        "🔐",
		Schedule:    "0 8 * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{CERT_CHECK_ENDPOINT}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"domains": [{{DOMAINS}}], "warning_days": {{WARNING_DAYS}}, "notify_channel": "{{NOTIFY_CHANNEL}}"}`,
		},
		Timeout:     "5m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "CERT_CHECK_ENDPOINT",
				Description: "Certificate check API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "DOMAINS",
				Description: "Domains to check (quoted, comma-separated)",
				Type:        "string",
				Required:    true,
				Default:     `"example.com"`,
			},
			{
				Name:        "WARNING_DAYS",
				Description: "Days before expiry to warn",
				Type:        "number",
				Required:    false,
				Default:     30,
			},
			{
				Name:        "NOTIFY_CHANNEL",
				Description: "Notification channel (slack, email)",
				Type:        "string",
				Required:    false,
				Default:     "slack",
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// logRotationTemplate provides a log rotation template.
func logRotationTemplate() *Template {
	return &Template{
		ID:          "log-rotation",
		Name:        "Log Rotation Trigger",
		Description: "Trigger log rotation and archival",
		Category:    CategoryMaintenance,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"logs", "rotation", "archival", "storage"},
		Icon:        "📜",
		Schedule:    "0 0 * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{LOG_ROTATION_ENDPOINT}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"action": "rotate", "compress": {{COMPRESS}}, "retention_days": {{RETENTION_DAYS}}}`,
		},
		Timeout:     "30m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "LOG_ROTATION_ENDPOINT",
				Description: "Log rotation API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "COMPRESS",
				Description: "Whether to compress rotated logs",
				Type:        "boolean",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "RETENTION_DAYS",
				Description: "Number of days to retain logs",
				Type:        "number",
				Required:    false,
				Default:     90,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// dataRetentionTemplate provides a data retention/cleanup template.
func dataRetentionTemplate() *Template {
	return &Template{
		ID:          "data-retention",
		Name:        "Data Retention Cleanup",
		Description: "Delete old data based on retention policy",
		Category:    CategoryDataPipeline,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"data", "retention", "cleanup", "gdpr", "compliance"},
		Icon:        "🗑️",
		Schedule:    "0 1 * * 0",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{RETENTION_ENDPOINT}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"tables": [{{TABLES}}], "retention_days": {{RETENTION_DAYS}}, "dry_run": {{DRY_RUN}}}`,
		},
		Timeout:     "1h",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "RETENTION_ENDPOINT",
				Description: "Data retention API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "TABLES",
				Description: "Tables/collections to clean (quoted, comma-separated)",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "RETENTION_DAYS",
				Description: "Days of data to retain",
				Type:        "number",
				Required:    true,
				Default:     365,
			},
			{
				Name:        "DRY_RUN",
				Description: "Dry run mode (don't actually delete)",
				Type:        "boolean",
				Required:    false,
				Default:     false,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// slackDailyDigestTemplate provides a daily digest template.
func slackDailyDigestTemplate() *Template {
	return &Template{
		ID:          "slack-daily-digest",
		Name:        "Slack Daily Digest",
		Description: "Send a daily digest/summary to Slack",
		Category:    CategoryNotification,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"slack", "digest", "summary", "daily"},
		Icon:        "📰",
		Schedule:    "0 17 * * 1-5",
		Timezone:    "America/New_York",
		Webhook: WebhookTemplate{
			URL:    "{{DIGEST_API}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"channel": "{{CHANNEL}}", "digest_type": "{{DIGEST_TYPE}}", "include_metrics": {{INCLUDE_METRICS}}}`,
		},
		Timeout:     "5m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "DIGEST_API",
				Description: "Digest generation API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "CHANNEL",
				Description: "Slack channel to post to",
				Type:        "string",
				Required:    true,
				Default:     "#general",
			},
			{
				Name:        "DIGEST_TYPE",
				Description: "Type of digest (daily, weekly)",
				Type:        "string",
				Required:    false,
				Default:     "daily",
			},
			{
				Name:        "INCLUDE_METRICS",
				Description: "Include metrics in digest",
				Type:        "boolean",
				Required:    false,
				Default:     true,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// webhookPingTemplate provides a simple webhook ping template.
func webhookPingTemplate() *Template {
	return &Template{
		ID:          "webhook-ping",
		Name:        "Webhook Ping",
		Description: "Simple ping to keep webhooks/services alive",
		Category:    CategoryMonitoring,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"ping", "keepalive", "heartbeat"},
		Icon:        "🏓",
		Schedule:    "*/15 * * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{PING_URL}}",
			Method: "GET",
			Headers: map[string]string{
				"User-Agent": "Chronos-Ping/1.0",
			},
		},
		Timeout:     "30s",
		Concurrency: "allow",
		Variables: []TemplateVariable{
			{
				Name:        "PING_URL",
				Description: "URL to ping",
				Type:        "string",
				Required:    true,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// elasticsearchCleanupTemplate provides an Elasticsearch index cleanup template.
func elasticsearchCleanupTemplate() *Template {
	return &Template{
		ID:          "elasticsearch-cleanup",
		Name:        "Elasticsearch Index Cleanup",
		Description: "Delete old Elasticsearch indices based on age",
		Category:    CategoryMaintenance,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"elasticsearch", "cleanup", "indices", "logs"},
		Icon:        "🔍",
		Schedule:    "0 4 * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{ES_CLEANUP_ENDPOINT}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"index_pattern": "{{INDEX_PATTERN}}", "retention_days": {{RETENTION_DAYS}}, "dry_run": {{DRY_RUN}}}`,
		},
		Timeout:     "30m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "ES_CLEANUP_ENDPOINT",
				Description: "Elasticsearch cleanup API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "INDEX_PATTERN",
				Description: "Index pattern to clean (e.g., logs-*)",
				Type:        "string",
				Required:    true,
				Default:     "logs-*",
			},
			{
				Name:        "RETENTION_DAYS",
				Description: "Days of indices to retain",
				Type:        "number",
				Required:    false,
				Default:     30,
			},
			{
				Name:        "DRY_RUN",
				Description: "Dry run mode",
				Type:        "boolean",
				Required:    false,
				Default:     false,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// redisFlushTemplate provides a Redis cache flush template.
func redisFlushTemplate() *Template {
	return &Template{
		ID:          "redis-flush",
		Name:        "Redis Cache Flush",
		Description: "Flush Redis cache by pattern",
		Category:    CategoryMaintenance,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"redis", "cache", "flush", "cleanup"},
		Icon:        "🔴",
		Schedule:    "0 3 * * 0",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "{{REDIS_API}}",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer {{API_TOKEN}}",
			},
			Body: `{"action": "flush", "pattern": "{{PATTERN}}", "database": {{DATABASE}}}`,
		},
		Timeout:     "10m",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "REDIS_API",
				Description: "Redis management API endpoint",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "API_TOKEN",
				Description: "API authentication token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "PATTERN",
				Description: "Key pattern to flush (e.g., cache:*)",
				Type:        "string",
				Required:    true,
				Default:     "cache:*",
			},
			{
				Name:        "DATABASE",
				Description: "Redis database number",
				Type:        "number",
				Required:    false,
				Default:     0,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// gitHubActionsTemplate provides a GitHub Actions workflow trigger template.
func gitHubActionsTemplate() *Template {
	return &Template{
		ID:          "github-actions-trigger",
		Name:        "GitHub Actions Workflow Trigger",
		Description: "Trigger a GitHub Actions workflow on a schedule",
		Category:    CategoryIntegration,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"github", "actions", "ci", "cd", "automation"},
		Icon:        "🐙",
		Schedule:    "0 0 * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "https://api.github.com/repos/{{OWNER}}/{{REPO}}/actions/workflows/{{WORKFLOW_ID}}/dispatches",
			Method: "POST",
			Headers: map[string]string{
				"Accept":        "application/vnd.github+json",
				"Authorization": "Bearer {{GITHUB_TOKEN}}",
				"Content-Type":  "application/json",
			},
			Body: `{"ref": "{{REF}}", "inputs": {{INPUTS}}}`,
		},
		Timeout:     "30s",
		Concurrency: "forbid",
		Variables: []TemplateVariable{
			{
				Name:        "OWNER",
				Description: "GitHub repository owner",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "REPO",
				Description: "GitHub repository name",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "WORKFLOW_ID",
				Description: "Workflow file name or ID",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "GITHUB_TOKEN",
				Description: "GitHub personal access token",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "REF",
				Description: "Git ref (branch/tag) to run workflow on",
				Type:        "string",
				Required:    false,
				Default:     "main",
			},
			{
				Name:        "INPUTS",
				Description: "Workflow inputs as JSON object",
				Type:        "string",
				Required:    false,
				Default:     "{}",
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// datadogMetricsTemplate provides a Datadog custom metrics submission template.
func datadogMetricsTemplate() *Template {
	return &Template{
		ID:          "datadog-metrics",
		Name:        "Datadog Custom Metrics",
		Description: "Submit custom metrics to Datadog",
		Category:    CategoryMonitoring,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"datadog", "metrics", "monitoring", "observability"},
		Icon:        "🐕",
		Schedule:    "* * * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "https://api.datadoghq.com/api/v2/series",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type": "application/json",
				"DD-API-KEY":   "{{DD_API_KEY}}",
			},
			Body: `{"series": [{"metric": "{{METRIC_NAME}}", "type": {{METRIC_TYPE}}, "points": [[{{TIMESTAMP}}, {{VALUE}}]], "tags": [{{TAGS}}]}]}`,
		},
		Timeout:     "30s",
		Concurrency: "allow",
		Variables: []TemplateVariable{
			{
				Name:        "DD_API_KEY",
				Description: "Datadog API key",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "METRIC_NAME",
				Description: "Name of the metric",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "METRIC_TYPE",
				Description: "Type of metric (0=unspecified, 1=count, 2=rate, 3=gauge)",
				Type:        "number",
				Required:    false,
				Default:     3,
			},
			{
				Name:        "TIMESTAMP",
				Description: "Unix timestamp (use $NOW for current time)",
				Type:        "string",
				Required:    true,
			},
			{
				Name:        "VALUE",
				Description: "Metric value",
				Type:        "number",
				Required:    true,
			},
			{
				Name:        "TAGS",
				Description: "Metric tags (quoted, comma-separated)",
				Type:        "string",
				Required:    false,
				Default:     `"env:production"`,
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// pagerDutyHeartbeatTemplate provides a PagerDuty heartbeat template.
func pagerDutyHeartbeatTemplate() *Template {
	return &Template{
		ID:          "pagerduty-heartbeat",
		Name:        "PagerDuty Heartbeat",
		Description: "Send heartbeat to PagerDuty to indicate system health",
		Category:    CategoryMonitoring,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"pagerduty", "heartbeat", "monitoring", "alerting"},
		Icon:        "💓",
		Schedule:    "*/5 * * * *",
		Timezone:    "UTC",
		Webhook: WebhookTemplate{
			URL:    "https://events.pagerduty.com/v2/enqueue",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"routing_key": "{{ROUTING_KEY}}", "event_action": "trigger", "dedup_key": "{{DEDUP_KEY}}", "payload": {"summary": "{{SUMMARY}}", "source": "chronos", "severity": "info"}}`,
		},
		Timeout:     "30s",
		Concurrency: "allow",
		Variables: []TemplateVariable{
			{
				Name:        "ROUTING_KEY",
				Description: "PagerDuty integration routing key",
				Type:        "secret",
				Required:    true,
			},
			{
				Name:        "DEDUP_KEY",
				Description: "Deduplication key for the heartbeat",
				Type:        "string",
				Required:    true,
				Default:     "chronos-heartbeat",
			},
			{
				Name:        "SUMMARY",
				Description: "Summary message for the heartbeat",
				Type:        "string",
				Required:    false,
				Default:     "Chronos heartbeat - system healthy",
			},
		},
		Downloads: 0,
		Rating:    0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
