// Package marketplace provides workflow templates for Chronos.
// This file contains operations workflow templates (incident response, maintenance, monitoring).
package marketplace

import "time"

// OperationsWorkflowTemplates returns operations-related workflow templates.
func OperationsWorkflowTemplates() []*WorkflowTemplate {
	return []*WorkflowTemplate{
		incidentResponseTemplate(),
		databaseMaintenanceTemplate(),
		websiteMonitoringTemplate(),
		infrastructureHealthCheckTemplate(),
	}
}

func incidentResponseTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "incident-response",
		Name:        "Incident Response Automation",
		Description: "Automated incident response: alert, triage, escalate, and remediate",
		Category:    WorkflowCategoryIncident,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"incident", "oncall", "pagerduty", "remediation", "sre"},
		Icon:        "🚨",
		License:     "Apache-2.0",
		Verified:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "detect",
				Name:        "Detect Incident",
				Description: "Receive and parse incident alert",
				Type:        StepTypeTransform,
				Config: map[string]interface{}{
					"input":  "{{trigger.payload}}",
					"output": "incident",
				},
			},
			{
				ID:           "create-ticket",
				Name:         "Create Incident Ticket",
				Description:  "Create ticket in incident management system",
				Type:         StepTypeHTTP,
				Dependencies: []string{"detect"},
				Config: map[string]interface{}{
					"url":    "{{INCIDENT_MGMT_URL}}/api/incidents",
					"method": "POST",
					"headers": map[string]string{
						"Authorization": "Bearer {{INCIDENT_MGMT_TOKEN}}",
					},
					"body": `{"title": "{{incident.title}}", "severity": "{{incident.severity}}", "source": "{{incident.source}}"}`,
				},
			},
			{
				ID:           "page-oncall",
				Name:         "Page On-Call",
				Description:  "Alert on-call engineer via PagerDuty",
				Type:         StepTypeHTTP,
				Dependencies: []string{"detect"},
				Config: map[string]interface{}{
					"url":    "https://events.pagerduty.com/v2/enqueue",
					"method": "POST",
					"body": `{
						"routing_key": "{{PAGERDUTY_KEY}}",
						"event_action": "trigger",
						"payload": {
							"summary": "{{incident.title}}",
							"severity": "{{incident.severity}}",
							"source": "chronos"
						}
					}`,
				},
			},
			{
				ID:           "check-runbook",
				Name:         "Check for Runbook",
				Description:  "Look up automated remediation runbook",
				Type:         StepTypeHTTP,
				Dependencies: []string{"detect"},
				Config: map[string]interface{}{
					"url":    "{{RUNBOOK_URL}}/api/runbooks/{{incident.type}}",
					"method": "GET",
				},
			},
			{
				ID:           "auto-remediate",
				Name:         "Attempt Auto-Remediation",
				Description:  "Execute automated fix if runbook exists",
				Type:         StepTypeCondition,
				Dependencies: []string{"check-runbook"},
				Condition:    "{{steps.check-runbook.output.runbook_found}} == true",
				Config: map[string]interface{}{
					"if_true":  "execute-runbook",
					"if_false": "wait-for-human",
				},
			},
			{
				ID:           "execute-runbook",
				Name:         "Execute Runbook",
				Description:  "Run automated remediation steps",
				Type:         StepTypeHTTP,
				Dependencies: []string{"auto-remediate"},
				Config: map[string]interface{}{
					"url":    "{{RUNBOOK_URL}}/api/execute",
					"method": "POST",
					"body":   `{"runbook_id": "{{steps.check-runbook.output.runbook_id}}", "incident_id": "{{steps.create-ticket.output.id}}"}`,
				},
				Timeout: "15m",
			},
			{
				ID:           "verify-fix",
				Name:         "Verify Fix",
				Description:  "Check if auto-remediation resolved the issue",
				Type:         StepTypeHTTP,
				Dependencies: []string{"execute-runbook"},
				Config: map[string]interface{}{
					"url":    "{{MONITORING_URL}}/api/check/{{incident.check_id}}",
					"method": "GET",
				},
			},
			{
				ID:           "close-incident",
				Name:         "Close Incident",
				Description:  "Mark incident as resolved",
				Type:         StepTypeHTTP,
				Dependencies: []string{"verify-fix"},
				Condition:    "{{steps.verify-fix.output.status}} == 'healthy'",
				Config: map[string]interface{}{
					"url":    "{{INCIDENT_MGMT_URL}}/api/incidents/{{steps.create-ticket.output.id}}/resolve",
					"method": "POST",
				},
			},
			{
				ID:           "wait-for-human",
				Name:         "Wait for Human Response",
				Description:  "No runbook found, wait for on-call to respond",
				Type:         StepTypeDelay,
				Dependencies: []string{"auto-remediate"},
				Config: map[string]interface{}{
					"duration": "30m",
				},
			},
			{
				ID:           "escalate",
				Name:         "Escalate if Unresolved",
				Description:  "Escalate to secondary on-call if not acknowledged",
				Type:         StepTypeHTTP,
				Dependencies: []string{"wait-for-human"},
				Config: map[string]interface{}{
					"url":    "https://events.pagerduty.com/v2/enqueue",
					"method": "POST",
					"body": `{
						"routing_key": "{{PAGERDUTY_ESCALATION_KEY}}",
						"event_action": "trigger",
						"payload": {
							"summary": "[ESCALATION] {{incident.title}}",
							"severity": "critical",
							"source": "chronos"
						}
					}`,
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type:    TriggerTypeWebhook,
				Name:    "Alert Webhook",
				Config:  map[string]interface{}{"path": "/webhooks/incident"},
				Enabled: true,
			},
			{
				Type:    TriggerTypeEvent,
				Name:    "Monitoring Alert",
				Config:  map[string]interface{}{"source": "prometheus", "type": "alert"},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "INCIDENT_MGMT_URL", Description: "Incident management system URL", Type: "string", Required: true},
			{Name: "INCIDENT_MGMT_TOKEN", Description: "Incident management auth token", Type: "secret", Required: true},
			{Name: "PAGERDUTY_KEY", Description: "PagerDuty routing key", Type: "secret", Required: true},
			{Name: "PAGERDUTY_ESCALATION_KEY", Description: "PagerDuty escalation routing key", Type: "secret", Required: true},
			{Name: "RUNBOOK_URL", Description: "Runbook service URL", Type: "string", Required: true},
			{Name: "MONITORING_URL", Description: "Monitoring system URL", Type: "string", Required: true},
		},
		Readme: `# Incident Response Automation

Automate incident response with detection, notification, auto-remediation, and escalation.

## Flow

1. Receive alert from monitoring
2. Create incident ticket
3. Page on-call engineer
4. Check for automated runbook
5. If runbook exists: execute and verify
6. If no runbook: wait then escalate
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func databaseMaintenanceTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "database-maintenance",
		Name:        "Database Maintenance",
		Description: "Scheduled database maintenance: backup, vacuum, reindex, and health check",
		Category:    WorkflowCategoryMaintenance,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"database", "postgres", "maintenance", "backup"},
		Icon:        "🗄️",
		License:     "Apache-2.0",
		Verified:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "pre-check",
				Name:        "Pre-Maintenance Check",
				Description: "Verify database is healthy before maintenance",
				Type:        StepTypeHTTP,
				Config: map[string]interface{}{
					"url":    "{{DB_ADMIN_URL}}/health",
					"method": "GET",
				},
			},
			{
				ID:           "create-backup",
				Name:         "Create Backup",
				Description:  "Take full database backup",
				Type:         StepTypeHTTP,
				Dependencies: []string{"pre-check"},
				Config: map[string]interface{}{
					"url":    "{{DB_ADMIN_URL}}/backup",
					"method": "POST",
					"body":   `{"type": "full", "destination": "{{BACKUP_BUCKET}}"}`,
				},
				Timeout: "60m",
			},
			{
				ID:           "vacuum-analyze",
				Name:         "Vacuum & Analyze",
				Description:  "Run VACUUM ANALYZE on all tables",
				Type:         StepTypeHTTP,
				Dependencies: []string{"create-backup"},
				Config: map[string]interface{}{
					"url":    "{{DB_ADMIN_URL}}/vacuum",
					"method": "POST",
					"body":   `{"analyze": true}`,
				},
				Timeout: "30m",
			},
			{
				ID:           "reindex",
				Name:         "Reindex Tables",
				Description:  "Rebuild indexes for better performance",
				Type:         StepTypeHTTP,
				Dependencies: []string{"vacuum-analyze"},
				Config: map[string]interface{}{
					"url":    "{{DB_ADMIN_URL}}/reindex",
					"method": "POST",
				},
				Timeout: "45m",
			},
			{
				ID:           "update-stats",
				Name:         "Update Statistics",
				Description:  "Update query planner statistics",
				Type:         StepTypeHTTP,
				Dependencies: []string{"reindex"},
				Config: map[string]interface{}{
					"url":    "{{DB_ADMIN_URL}}/statistics",
					"method": "POST",
				},
			},
			{
				ID:           "post-check",
				Name:         "Post-Maintenance Validation",
				Description:  "Verify database health after maintenance",
				Type:         StepTypeHTTP,
				Dependencies: []string{"update-stats"},
				Config: map[string]interface{}{
					"url":    "{{DB_ADMIN_URL}}/health",
					"method": "GET",
				},
			},
			{
				ID:           "notify",
				Name:         "Send Report",
				Description:  "Notify about maintenance completion",
				Type:         StepTypeNotification,
				Dependencies: []string{"post-check"},
				Config: map[string]interface{}{
					"channel": "slack",
					"webhook": "{{SLACK_WEBHOOK}}",
					"message": "✅ Database maintenance completed\n• Backup: {{steps.create-backup.output.backup_id}}\n• Duration: {{workflow.duration}}",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type: TriggerTypeCron,
				Name: "Weekly Sunday 3 AM",
				Config: map[string]interface{}{
					"schedule": "0 3 * * 0",
					"timezone": "UTC",
				},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "DB_ADMIN_URL", Description: "Database admin API URL", Type: "string", Required: true},
			{Name: "BACKUP_BUCKET", Description: "S3 bucket for backups", Type: "string", Required: true},
			{Name: "SLACK_WEBHOOK", Description: "Slack webhook URL", Type: "secret", Required: true},
		},
		Readme: `# Database Maintenance Workflow

Comprehensive database maintenance including backup, vacuum, reindex, and health checks.
`,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func websiteMonitoringTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "website-monitoring",
		Name:        "Website Monitoring",
		Description: "Monitor website availability and performance from multiple locations",
		Category:    WorkflowCategoryMonitoring,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"monitoring", "uptime", "performance", "synthetic"},
		Icon:        "🌐",
		License:     "Apache-2.0",
		Verified:    true,
		Featured:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "check-us-east",
				Name:        "Check US-East",
				Type:        StepTypeHTTP,
				Description: "Check from US-East region",
				Config: map[string]interface{}{
					"url":     "{{TARGET_URL}}",
					"method":  "GET",
					"timeout": "10s",
				},
			},
			{
				ID:          "check-eu-west",
				Name:        "Check EU-West",
				Type:        StepTypeHTTP,
				Description: "Check from EU-West region",
				Config: map[string]interface{}{
					"url":     "{{TARGET_URL}}",
					"method":  "GET",
					"timeout": "10s",
				},
			},
			{
				ID:          "check-ap-south",
				Name:        "Check AP-South",
				Type:        StepTypeHTTP,
				Description: "Check from AP-South region",
				Config: map[string]interface{}{
					"url":     "{{TARGET_URL}}",
					"method":  "GET",
					"timeout": "10s",
				},
			},
			{
				ID:           "aggregate-results",
				Name:         "Aggregate Results",
				Type:         StepTypeTransform,
				Dependencies: []string{"check-us-east", "check-eu-west", "check-ap-south"},
				Config: map[string]interface{}{
					"expression": `{
						"available": all(steps.*.status == 200),
						"avg_latency": avg(steps.*.duration),
						"regions": [steps.check-us-east, steps.check-eu-west, steps.check-ap-south]
					}`,
				},
			},
			{
				ID:           "record-metrics",
				Name:         "Record Metrics",
				Type:         StepTypeHTTP,
				Dependencies: []string{"aggregate-results"},
				Config: map[string]interface{}{
					"url":    "{{METRICS_URL}}/api/v1/write",
					"method": "POST",
					"body":   `{"metrics": {{steps.aggregate-results.output}}}`,
				},
			},
			{
				ID:           "alert-if-down",
				Name:         "Alert if Down",
				Type:         StepTypeNotification,
				Dependencies: []string{"aggregate-results"},
				Condition:    "{{steps.aggregate-results.output.available}} == false",
				Config: map[string]interface{}{
					"channel":     "pagerduty",
					"routing_key": "{{PAGERDUTY_KEY}}",
					"message":     "Website {{TARGET_URL}} is DOWN",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type:    TriggerTypeCron,
				Name:    "Every 5 minutes",
				Config:  map[string]interface{}{"schedule": "*/5 * * * *"},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "TARGET_URL", Description: "URL to monitor", Type: "string", Required: true},
			{Name: "METRICS_URL", Description: "Metrics backend URL", Type: "string", Required: true},
			{Name: "PAGERDUTY_KEY", Description: "PagerDuty routing key", Type: "secret", Required: true},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func infrastructureHealthCheckTemplate() *WorkflowTemplate {
	return &WorkflowTemplate{
		ID:          "infrastructure-health-check",
		Name:        "Infrastructure Health Check",
		Description: "Comprehensive infrastructure health monitoring",
		Category:    WorkflowCategoryMonitoring,
		Author:      "Chronos",
		Version:     "1.0.0",
		Tags:        []string{"infrastructure", "health", "kubernetes", "database"},
		Icon:        "🏥",
		License:     "Apache-2.0",
		Verified:    true,
		Steps: []WorkflowStepTemplate{
			{
				ID:          "check-k8s",
				Name:        "Check Kubernetes",
				Type:        StepTypeHTTP,
				Description: "Verify K8s cluster health",
				Config: map[string]interface{}{
					"url":     "{{K8S_API}}/healthz",
					"method":  "GET",
					"headers": map[string]string{"Authorization": "Bearer {{K8S_TOKEN}}"},
				},
			},
			{
				ID:          "check-database",
				Name:        "Check Database",
				Type:        StepTypeHTTP,
				Description: "Verify database connectivity",
				Config: map[string]interface{}{
					"url":    "{{DB_HEALTH_URL}}",
					"method": "GET",
				},
			},
			{
				ID:          "check-redis",
				Name:        "Check Redis",
				Type:        StepTypeHTTP,
				Description: "Verify Redis connectivity",
				Config: map[string]interface{}{
					"url":    "{{REDIS_HEALTH_URL}}",
					"method": "GET",
				},
			},
			{
				ID:          "check-storage",
				Name:        "Check Storage",
				Type:        StepTypeHTTP,
				Description: "Verify storage system",
				Config: map[string]interface{}{
					"url":    "{{STORAGE_HEALTH_URL}}",
					"method": "GET",
				},
			},
			{
				ID:           "aggregate",
				Name:         "Aggregate Health Status",
				Type:         StepTypeTransform,
				Dependencies: []string{"check-k8s", "check-database", "check-redis", "check-storage"},
				Config: map[string]interface{}{
					"expression": `{
						"healthy": all(steps.*.status == 200),
						"components": {
							"kubernetes": steps.check-k8s.status == 200,
							"database": steps.check-database.status == 200,
							"redis": steps.check-redis.status == 200,
							"storage": steps.check-storage.status == 200
						}
					}`,
				},
			},
			{
				ID:           "alert-unhealthy",
				Name:         "Alert on Unhealthy",
				Type:         StepTypeNotification,
				Dependencies: []string{"aggregate"},
				Condition:    "{{steps.aggregate.output.healthy}} == false",
				Config: map[string]interface{}{
					"channel":     "pagerduty",
					"routing_key": "{{PAGERDUTY_KEY}}",
					"message":     "Infrastructure health check failed: {{steps.aggregate.output.components}}",
				},
			},
		},
		Triggers: []TriggerTemplate{
			{
				Type:    TriggerTypeCron,
				Name:    "Every minute",
				Config:  map[string]interface{}{"schedule": "* * * * *"},
				Enabled: true,
			},
		},
		Variables: []TemplateVariable{
			{Name: "K8S_API", Description: "Kubernetes API URL", Type: "string", Required: true},
			{Name: "K8S_TOKEN", Description: "Kubernetes token", Type: "secret", Required: true},
			{Name: "DB_HEALTH_URL", Description: "Database health endpoint", Type: "string", Required: true},
			{Name: "REDIS_HEALTH_URL", Description: "Redis health endpoint", Type: "string", Required: true},
			{Name: "STORAGE_HEALTH_URL", Description: "Storage health endpoint", Type: "string", Required: true},
			{Name: "PAGERDUTY_KEY", Description: "PagerDuty routing key", Type: "secret", Required: true},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
