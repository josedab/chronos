// Package marketplace provides workflow templates for Chronos.
package marketplace

import "time"

// NotificationWorkflowTemplates returns notification and reporting workflow templates.
func NotificationWorkflowTemplates() []*WorkflowTemplate {
	return []*WorkflowTemplate{
		dailyReportingTemplate(),
		multiChannelNotificationTemplate(),
		userOnboardingTemplate(),
	}
}

func dailyReportingTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "daily-reporting",
		Name:        "Daily Reporting",
		Description: "Aggregate metrics and send daily reports to stakeholders",
		Category:    WorkflowCategoryNotification,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"reporting", "daily", "metrics", "stakeholder", "notification"},
		Icon:        "📊",
		License:     "Apache-2.0",
		Verified:    true,
		Featured:    false,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "fetch-sales-metrics",
				Name:        "Fetch Sales Metrics",
				Description: "Get daily sales data",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{METRICS_API_URL}}/sales",
					"method": "GET",
					"headers": map[string]string{
						"Authorization": "Bearer {{METRICS_API_TOKEN}}",
					},
					"query": map[string]string{
						"date": "{{yesterday}}",
					},
				},
				Timeout: "5m",
			},
			{
				ID:          "fetch-traffic-metrics",
				Name:        "Fetch Traffic Metrics",
				Description: "Get daily website traffic data",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{ANALYTICS_URL}}/traffic",
					"method": "GET",
					"headers": map[string]string{
						"Authorization": "Bearer {{ANALYTICS_TOKEN}}",
					},
				},
				Timeout: "5m",
			},
			{
				ID:          "fetch-support-metrics",
				Name:        "Fetch Support Metrics",
				Description: "Get daily support ticket data",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{SUPPORT_API_URL}}/metrics/daily",
					"method": "GET",
					"headers": map[string]string{
						"Authorization": "Bearer {{SUPPORT_API_TOKEN}}",
					},
				},
				Timeout: "5m",
			},
			{
				ID:           "generate-report",
				Name:         "Generate Report",
				Description:  "Compile all metrics into a formatted report",
				Type:         StepTypeHTTP,
				Dependencies: []string{"fetch-sales-metrics", "fetch-traffic-metrics", "fetch-support-metrics"},
				Config: map[string]interface{}{
					"url":    "{{REPORT_SERVICE_URL}}/generate",
					"method": "POST",
					"body": `{
						"template": "daily_summary",
						"data": {
							"sales": "{{steps.fetch-sales-metrics.output}}",
							"traffic": "{{steps.fetch-traffic-metrics.output}}",
							"support": "{{steps.fetch-support-metrics.output}}",
							"date": "{{yesterday}}"
						}
					}`,
				},
				Timeout: "10m",
			},
			{
				ID:           "send-email",
				Name:         "Send Email Report",
				Description:  "Email the report to stakeholders",
				Type:         StepTypeNotification,
				Dependencies: []string{"generate-report"},
				Config: map[string]interface{}{
					"channel": "email",
					"to":      "{{STAKEHOLDER_EMAILS}}",
					"subject": "Daily Business Report - {{yesterday}}",
					"body":    "{{steps.generate-report.output.html}}",
				},
			},
			{
				ID:           "send-slack",
				Name:         "Post to Slack",
				Description:  "Post summary to Slack channel",
				Type:         StepTypeNotification,
				Dependencies: []string{"generate-report"},
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "📊 *Daily Report - {{yesterday}}*\n{{steps.generate-report.output.summary}}",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type: TriggerTypeCron,
				Name: "Daily at 8 AM",
				Config: map[string]interface{}{
					"schedule": "0 8 * * *",
					"timezone": "{{REPORT_TIMEZONE}}",
				},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "METRICS_API_URL", Description: "Metrics API base URL", Type: "string", Required: true},
			{Name: "METRICS_API_TOKEN", Description: "Metrics API token", Type: "secret", Required: true},
			{Name: "ANALYTICS_URL", Description: "Analytics service URL", Type: "string", Required: true},
			{Name: "ANALYTICS_TOKEN", Description: "Analytics API token", Type: "secret", Required: true},
			{Name: "SUPPORT_API_URL", Description: "Support system API URL", Type: "string", Required: true},
			{Name: "SUPPORT_API_TOKEN", Description: "Support API token", Type: "secret", Required: true},
			{Name: "REPORT_SERVICE_URL", Description: "Report generation service URL", Type: "string", Required: true},
			{Name: "STAKEHOLDER_EMAILS", Description: "Comma-separated list of stakeholder emails", Type: "string", Required: true},
			{Name: "SLACK_WEBHOOK", Description: "Slack webhook URL", Type: "secret", Required: true},
			{Name: "REPORT_TIMEZONE", Description: "Timezone for report scheduling", Type: "string", Default: "America/New_York"},
		},
		Readme: `# Daily Reporting Workflow

Aggregates metrics from multiple sources and distributes daily reports via email and Slack.

## Data Sources
- Sales metrics API
- Website analytics
- Support ticket system

## Outputs
- HTML email report to stakeholders
- Slack summary message

## Schedule
Runs daily at 8 AM in the configured timezone.
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func multiChannelNotificationTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "multi-channel-notification",
		Name:        "Multi-Channel Notification",
		Description: "Send notifications across multiple channels with fallback support",
		Category:    WorkflowCategoryNotification,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"notification", "multi-channel", "alert", "slack", "email", "sms"},
		Icon:        "🔔",
		License:     "Apache-2.0",
		Verified:    true,
		Featured:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "prepare-message",
				Name:        "Prepare Message",
				Description: "Format the notification message for all channels",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{TEMPLATE_SERVICE_URL}}/render",
					"method": "POST",
					"body": `{
						"template": "{{NOTIFICATION_TEMPLATE}}",
						"data": {
							"title": "{{NOTIFICATION_TITLE}}",
							"message": "{{NOTIFICATION_MESSAGE}}",
							"severity": "{{NOTIFICATION_SEVERITY}}",
							"timestamp": "{{now}}"
						}
					}`,
				},
				Timeout: "1m",
			},
			{
				ID:           "send-slack",
				Name:         "Send Slack Notification",
				Description:  "Post notification to Slack",
				Type:         StepTypeNotification,
				Dependencies: []string{"prepare-message"},
				Config: map[string]interface{}{
					"channel":  "slack",
					"webhook":  "{{SLACK_WEBHOOK}}",
					"message":  "{{steps.prepare-message.output.slack}}",
					"priority": "{{NOTIFICATION_SEVERITY}}",
				},
			},
			{
				ID:           "send-email",
				Name:         "Send Email Notification",
				Description:  "Send email notification",
				Type:         StepTypeNotification,
				Dependencies: []string{"prepare-message"},
				Config: map[string]interface{}{
					"channel": "email",
					"to":      "{{EMAIL_RECIPIENTS}}",
					"subject": "[{{NOTIFICATION_SEVERITY}}] {{NOTIFICATION_TITLE}}",
					"body":    "{{steps.prepare-message.output.email}}",
				},
			},
			{
				ID:           "send-pagerduty",
				Name:         "Send PagerDuty Alert",
				Description:  "Create PagerDuty incident for critical notifications",
				Type:         StepTypeHTTP,
				Dependencies: []string{"prepare-message"},
				Condition:    "{{NOTIFICATION_SEVERITY}} == 'critical'",
				Config: map[string]interface{}{
					"url":    "https://events.pagerduty.com/v2/enqueue",
					"method": "POST",
					"headers": map[string]string{
						"Content-Type": "application/json",
					},
					"body": `{
						"routing_key": "{{PAGERDUTY_ROUTING_KEY}}",
						"event_action": "trigger",
						"payload": {
							"summary": "{{NOTIFICATION_TITLE}}",
							"severity": "{{NOTIFICATION_SEVERITY}}",
							"source": "chronos",
							"custom_details": {{steps.prepare-message.output.details}}
						}
					}`,
				},
			},
			{
				ID:           "send-sms",
				Name:         "Send SMS Alert",
				Description:  "Send SMS for urgent notifications",
				Type:         StepTypeHTTP,
				Dependencies: []string{"prepare-message"},
				Condition:    "{{NOTIFICATION_SEVERITY}} in ['critical', 'high']",
				Config: map[string]interface{}{
					"url":    "{{SMS_SERVICE_URL}}/send",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{SMS_API_TOKEN}}",
					},
					"body": `{
						"to": "{{SMS_RECIPIENTS}}",
						"message": "{{steps.prepare-message.output.sms}}"
					}`,
				},
			},
			{
				ID:           "log-delivery",
				Name:         "Log Delivery Status",
				Description:  "Record notification delivery results",
				Type:         StepTypeHTTP,
				Dependencies: []string{"send-slack", "send-email", "send-pagerduty", "send-sms"},
				Config: map[string]interface{}{
					"url":    "{{AUDIT_LOG_URL}}/notifications",
					"method": "POST",
					"body": `{
						"notification_id": "{{workflow.run_id}}",
						"channels": {
							"slack": "{{steps.send-slack.status}}",
							"email": "{{steps.send-email.status}}",
							"pagerduty": "{{steps.send-pagerduty.status}}",
							"sms": "{{steps.send-sms.status}}"
						},
						"timestamp": "{{now}}"
					}`,
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type: TriggerTypeWebhook,
				Name: "Webhook trigger",
				Config: map[string]interface{}{
					"path":   "/notify",
					"method": "POST",
				},
				Enabled: true,
			},
			{
				Type:    TriggerTypeManual,
				Name:    "Manual trigger",
				Config:  map[string]interface{}{},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "NOTIFICATION_TITLE", Description: "Notification title", Type: "string", Required: true},
			{Name: "NOTIFICATION_MESSAGE", Description: "Notification message body", Type: "string", Required: true},
			{Name: "NOTIFICATION_SEVERITY", Description: "Severity level (critical, high, medium, low)", Type: "string", Default: "medium"},
			{Name: "NOTIFICATION_TEMPLATE", Description: "Template name for formatting", Type: "string", Default: "default"},
			{Name: "TEMPLATE_SERVICE_URL", Description: "Template rendering service URL", Type: "string", Required: true},
			{Name: "SLACK_WEBHOOK", Description: "Slack incoming webhook URL", Type: "secret", Required: true},
			{Name: "EMAIL_RECIPIENTS", Description: "Email recipients (comma-separated)", Type: "string", Required: true},
			{Name: "PAGERDUTY_ROUTING_KEY", Description: "PagerDuty integration key", Type: "secret", Required: false},
			{Name: "SMS_SERVICE_URL", Description: "SMS gateway service URL", Type: "string", Required: false},
			{Name: "SMS_API_TOKEN", Description: "SMS service API token", Type: "secret", Required: false},
			{Name: "SMS_RECIPIENTS", Description: "SMS recipients (comma-separated phone numbers)", Type: "string", Required: false},
			{Name: "AUDIT_LOG_URL", Description: "Audit logging service URL", Type: "string", Required: true},
		},
		Readme: `# Multi-Channel Notification Workflow

Delivers notifications across multiple channels with automatic fallback and delivery logging.

## Channels
- **Slack**: Always attempted
- **Email**: Always attempted
- **PagerDuty**: Only for critical severity
- **SMS**: Only for critical/high severity

## Severity Levels
- critical: All channels including PagerDuty and SMS
- high: Slack, email, and SMS
- medium: Slack and email only
- low: Slack and email only

## Failure Handling
Each channel is independent - failures in one channel don't affect others.
All delivery attempts are logged for auditing.
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func userOnboardingTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "user-onboarding",
		Name:        "User Onboarding Sequence",
		Description: "Automated user onboarding with welcome emails and setup reminders",
		Category:    WorkflowCategoryNotification,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"onboarding", "user", "email", "drip", "sequence"},
		Icon:        "👋",
		License:     "Apache-2.0",
		Verified:    true,
		Featured:    false,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "get-user-data",
				Name:        "Fetch User Data",
				Description: "Get new user information",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{USER_API_URL}}/users/{{USER_ID}}",
					"method": "GET",
					"headers": map[string]string{
						"Authorization": "Bearer {{USER_API_TOKEN}}",
					},
				},
				Timeout: "1m",
			},
			{
				ID:           "send-welcome-email",
				Name:         "Send Welcome Email",
				Description:  "Send personalized welcome email",
				Type:         StepTypeNotification,
				Dependencies: []string{"get-user-data"},
				Config: map[string]interface{}{
					"channel":  "email",
					"to":       "{{steps.get-user-data.output.email}}",
					"template": "welcome",
					"data": map[string]string{
						"name":    "{{steps.get-user-data.output.name}}",
						"company": "{{COMPANY_NAME}}",
					},
				},
			},
			{
				ID:           "create-crm-record",
				Name:         "Create CRM Record",
				Description:  "Add user to CRM for tracking",
				Type:         StepTypeHTTP,
				Dependencies: []string{"get-user-data"},
				Config: map[string]interface{}{
					"url":    "{{CRM_API_URL}}/contacts",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{CRM_API_TOKEN}}",
					},
					"body": `{
						"email": "{{steps.get-user-data.output.email}}",
						"name": "{{steps.get-user-data.output.name}}",
						"source": "signup",
						"stage": "onboarding"
					}`,
				},
			},
			{
				ID:           "schedule-followup",
				Name:         "Schedule Day 3 Follow-up",
				Description:  "Schedule a follow-up check-in email",
				Type:         StepTypeHTTP,
				Dependencies: []string{"send-welcome-email"},
				Config: map[string]interface{}{
					"url":    "{{CHRONOS_API_URL}}/jobs",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{CHRONOS_API_TOKEN}}",
					},
					"body": `{
						"name": "followup-{{USER_ID}}",
						"schedule": "at {{plus_days(3)}}",
						"executor": "http",
						"payload": {
							"workflow": "onboarding-followup",
							"user_id": "{{USER_ID}}"
						}
					}`,
				},
			},
			{
				ID:           "notify-sales",
				Name:         "Notify Sales Team",
				Description:  "Alert sales team about new signup",
				Type:         StepTypeNotification,
				Dependencies: []string{"get-user-data"},
				Condition:    "{{steps.get-user-data.output.plan}} == 'enterprise'",
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SALES_SLACK_WEBHOOK}}",
					"message": "🎉 New enterprise signup: {{steps.get-user-data.output.name}} ({{steps.get-user-data.output.company}})",
				},
			},
			{
				ID:           "track-event",
				Name:         "Track Onboarding Start",
				Description:  "Send analytics event",
				Type:         StepTypeHTTP,
				Dependencies: []string{"send-welcome-email"},
				Config: map[string]interface{}{
					"url":    "{{ANALYTICS_URL}}/track",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{ANALYTICS_TOKEN}}",
					},
					"body": `{
						"event": "onboarding_started",
						"user_id": "{{USER_ID}}",
						"properties": {
							"plan": "{{steps.get-user-data.output.plan}}",
							"source": "{{steps.get-user-data.output.signup_source}}"
						}
					}`,
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type: TriggerTypeWebhook,
				Name: "New user signup webhook",
				Config: map[string]interface{}{
					"path":   "/onboard",
					"method": "POST",
				},
				Enabled: true,
			},
			{
				Type: TriggerTypeEvent,
				Name: "User created event",
				Config: map[string]interface{}{
					"event": "user.created",
				},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "USER_ID", Description: "ID of the user to onboard", Type: "string", Required: true},
			{Name: "USER_API_URL", Description: "User service API URL", Type: "string", Required: true},
			{Name: "USER_API_TOKEN", Description: "User service API token", Type: "secret", Required: true},
			{Name: "CRM_API_URL", Description: "CRM API URL", Type: "string", Required: true},
			{Name: "CRM_API_TOKEN", Description: "CRM API token", Type: "secret", Required: true},
			{Name: "CHRONOS_API_URL", Description: "Chronos API URL for scheduling", Type: "string", Required: true},
			{Name: "CHRONOS_API_TOKEN", Description: "Chronos API token", Type: "secret", Required: true},
			{Name: "SALES_SLACK_WEBHOOK", Description: "Sales team Slack webhook", Type: "secret", Required: false},
			{Name: "ANALYTICS_URL", Description: "Analytics service URL", Type: "string", Required: true},
			{Name: "ANALYTICS_TOKEN", Description: "Analytics API token", Type: "secret", Required: true},
			{Name: "COMPANY_NAME", Description: "Your company name for emails", Type: "string", Default: "Chronos"},
		},
		Readme: `# User Onboarding Sequence

Automates the user onboarding process with welcome emails, CRM updates, and follow-up scheduling.

## Workflow
1. Fetch user data from user service
2. Send personalized welcome email
3. Create CRM contact record
4. Schedule day 3 follow-up
5. Notify sales for enterprise signups
6. Track analytics event

## Triggers
- Webhook: POST /onboard with user_id
- Event: user.created event

## Enterprise Users
Enterprise plan signups automatically notify the sales team.
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
