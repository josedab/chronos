// Package marketplace provides a library of reusable job templates.
package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// Common errors.
var (
	ErrTemplateNotFound   = errors.New("template not found")
	ErrTemplateExists     = errors.New("template already exists")
	ErrInvalidTemplate    = errors.New("invalid template")
	ErrInvalidCategory    = errors.New("invalid category")
)

// Category represents a template category.
type Category string

const (
	CategoryBackup      Category = "backup"
	CategoryMonitoring  Category = "monitoring"
	CategoryNotification Category = "notification"
	CategoryDataPipeline Category = "data-pipeline"
	CategoryMaintenance  Category = "maintenance"
	CategorySecurity     Category = "security"
	CategoryIntegration  Category = "integration"
	CategoryReporting    Category = "reporting"
)

// AllCategories returns all available categories.
func AllCategories() []Category {
	return []Category{
		CategoryBackup,
		CategoryMonitoring,
		CategoryNotification,
		CategoryDataPipeline,
		CategoryMaintenance,
		CategorySecurity,
		CategoryIntegration,
		CategoryReporting,
	}
}

// Template represents a job template in the marketplace.
type Template struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    Category               `json:"category"`
	Author      string                 `json:"author"`
	Version     string                 `json:"version"`
	Tags        []string               `json:"tags"`
	Icon        string                 `json:"icon,omitempty"`
	
	// Job configuration template
	Schedule    string                 `json:"schedule"`
	Timezone    string                 `json:"timezone,omitempty"`
	Webhook     WebhookTemplate        `json:"webhook"`
	Timeout     string                 `json:"timeout,omitempty"`
	Concurrency string                 `json:"concurrency,omitempty"`
	RetryPolicy *RetryPolicyTemplate   `json:"retry_policy,omitempty"`
	
	// Template variables
	Variables   []TemplateVariable     `json:"variables,omitempty"`
	
	// Metadata
	Downloads   int                    `json:"downloads"`
	Rating      float64                `json:"rating"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	
	// Documentation
	Readme      string                 `json:"readme,omitempty"`
	Examples    []Example              `json:"examples,omitempty"`
}

// WebhookTemplate is the webhook configuration template.
type WebhookTemplate struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// RetryPolicyTemplate is the retry policy template.
type RetryPolicyTemplate struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval"`
	MaxInterval     string  `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// TemplateVariable represents a configurable variable in a template.
type TemplateVariable struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        string      `json:"type"` // string, number, boolean, secret
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Validation  string      `json:"validation,omitempty"` // regex pattern
}

// Example shows how to use the template.
type Example struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`
}

// InstantiateRequest contains the variables to instantiate a template.
type InstantiateRequest struct {
	TemplateID string            `json:"template_id"`
	Name       string            `json:"name"`
	Variables  map[string]string `json:"variables"`
	Namespace  string            `json:"namespace,omitempty"`
}

// JobConfig is the instantiated job configuration.
type JobConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schedule    string                 `json:"schedule"`
	Timezone    string                 `json:"timezone,omitempty"`
	Webhook     WebhookConfig          `json:"webhook"`
	Timeout     string                 `json:"timeout,omitempty"`
	Concurrency string                 `json:"concurrency,omitempty"`
	RetryPolicy *RetryPolicyConfig     `json:"retry_policy,omitempty"`
	Tags        map[string]string      `json:"tags,omitempty"`
	Namespace   string                 `json:"namespace,omitempty"`
}

// WebhookConfig is the instantiated webhook configuration.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// RetryPolicyConfig is the instantiated retry policy.
type RetryPolicyConfig struct {
	MaxAttempts     int     `json:"max_attempts"`
	InitialInterval string  `json:"initial_interval"`
	MaxInterval     string  `json:"max_interval"`
	Multiplier      float64 `json:"multiplier"`
}

// Marketplace manages job templates.
type Marketplace struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewMarketplace creates a new marketplace with built-in templates.
func NewMarketplace() *Marketplace {
	m := &Marketplace{
		templates: make(map[string]*Template),
	}
	m.loadBuiltinTemplates()
	return m
}

// ListTemplates returns all templates, optionally filtered.
func (m *Marketplace) ListTemplates(ctx context.Context, category Category, search string) ([]*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Template
	for _, t := range m.templates {
		// Filter by category
		if category != "" && t.Category != category {
			continue
		}
		
		// Filter by search term
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(t.Name), searchLower) &&
				!strings.Contains(strings.ToLower(t.Description), searchLower) {
				continue
			}
		}
		
		result = append(result, t)
	}

	// Sort by downloads (most popular first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Downloads > result[j].Downloads
	})

	return result, nil
}

// GetTemplate returns a template by ID.
func (m *Marketplace) GetTemplate(ctx context.Context, id string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return t, nil
}

// InstantiateTemplate creates a job configuration from a template.
func (m *Marketplace) InstantiateTemplate(ctx context.Context, req *InstantiateRequest) (*JobConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.templates[req.TemplateID]
	if !ok {
		return nil, ErrTemplateNotFound
	}

	// Validate required variables
	for _, v := range t.Variables {
		if v.Required {
			if _, ok := req.Variables[v.Name]; !ok {
				if v.Default == nil {
					return nil, errors.New("missing required variable: " + v.Name)
				}
			}
		}
	}

	// Build variable map with defaults
	vars := make(map[string]string)
	for _, v := range t.Variables {
		if v.Default != nil {
			vars[v.Name] = toString(v.Default)
		}
	}
	for k, v := range req.Variables {
		vars[k] = v
	}

	// Instantiate the template
	config := &JobConfig{
		Name:        req.Name,
		Description: substituteVars(t.Description, vars),
		Schedule:    substituteVars(t.Schedule, vars),
		Timezone:    t.Timezone,
		Webhook: WebhookConfig{
			URL:     substituteVars(t.Webhook.URL, vars),
			Method:  t.Webhook.Method,
			Headers: substituteMapVars(t.Webhook.Headers, vars),
			Body:    substituteVars(t.Webhook.Body, vars),
		},
		Timeout:     t.Timeout,
		Concurrency: t.Concurrency,
		Namespace:   req.Namespace,
		Tags: map[string]string{
			"template":    t.ID,
			"template_version": t.Version,
		},
	}

	if t.RetryPolicy != nil {
		config.RetryPolicy = &RetryPolicyConfig{
			MaxAttempts:     t.RetryPolicy.MaxAttempts,
			InitialInterval: t.RetryPolicy.InitialInterval,
			MaxInterval:     t.RetryPolicy.MaxInterval,
			Multiplier:      t.RetryPolicy.Multiplier,
		}
	}

	// Increment downloads
	t.Downloads++

	return config, nil
}

// AddTemplate adds a custom template.
func (m *Marketplace) AddTemplate(ctx context.Context, t *Template) error {
	if t.ID == "" || t.Name == "" {
		return ErrInvalidTemplate
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[t.ID]; exists {
		return ErrTemplateExists
	}

	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.templates[t.ID] = t
	return nil
}

// loadBuiltinTemplates loads the built-in template library.
func (m *Marketplace) loadBuiltinTemplates() {
	templates := []*Template{
		// Backup templates
		{
			ID:          "database-backup",
			Name:        "Database Backup",
			Description: "Automated database backup to {{storage_provider}}",
			Category:    CategoryBackup,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"database", "backup", "postgresql", "mysql"},
			Schedule:    "0 2 * * *",
			Timezone:    "UTC",
			Webhook: WebhookTemplate{
				URL:    "{{backup_endpoint}}/backup",
				Method: "POST",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"database": "{{database_name}}", "storage": "{{storage_provider}}", "compress": true}`,
			},
			Timeout:     "1h",
			Concurrency: "forbid",
			RetryPolicy: &RetryPolicyTemplate{MaxAttempts: 3, InitialInterval: "1m", MaxInterval: "10m", Multiplier: 2},
			Variables: []TemplateVariable{
				{Name: "backup_endpoint", Description: "Backup service endpoint URL", Type: "string", Required: true},
				{Name: "database_name", Description: "Database name to backup", Type: "string", Required: true},
				{Name: "storage_provider", Description: "Storage provider (s3, gcs, azure)", Type: "string", Required: true, Default: "s3"},
			},
			Downloads: 1250,
			Rating:    4.8,
		},
		{
			ID:          "s3-sync",
			Name:        "S3 Bucket Sync",
			Description: "Sync files between S3 buckets",
			Category:    CategoryBackup,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"s3", "aws", "sync", "backup"},
			Schedule:    "0 */6 * * *",
			Webhook: WebhookTemplate{
				URL:    "{{sync_endpoint}}/sync",
				Method: "POST",
				Body:   `{"source": "{{source_bucket}}", "destination": "{{dest_bucket}}"}`,
			},
			Timeout:     "2h",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "sync_endpoint", Description: "Sync service endpoint", Type: "string", Required: true},
				{Name: "source_bucket", Description: "Source S3 bucket", Type: "string", Required: true},
				{Name: "dest_bucket", Description: "Destination S3 bucket", Type: "string", Required: true},
			},
			Downloads: 890,
			Rating:    4.6,
		},
		// Monitoring templates
		{
			ID:          "health-check",
			Name:        "HTTP Health Check",
			Description: "Monitor endpoint availability and response time",
			Category:    CategoryMonitoring,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"monitoring", "health", "uptime", "http"},
			Schedule:    "*/5 * * * *",
			Webhook: WebhookTemplate{
				URL:    "{{target_url}}",
				Method: "GET",
				Headers: map[string]string{
					"User-Agent": "Chronos-HealthCheck/1.0",
				},
			},
			Timeout:     "30s",
			Concurrency: "allow",
			Variables: []TemplateVariable{
				{Name: "target_url", Description: "URL to health check", Type: "string", Required: true},
			},
			Downloads: 2100,
			Rating:    4.9,
		},
		{
			ID:          "ssl-expiry-check",
			Name:        "SSL Certificate Expiry Check",
			Description: "Check SSL certificate expiration and alert before expiry",
			Category:    CategorySecurity,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"ssl", "tls", "certificate", "security"},
			Schedule:    "0 9 * * *",
			Webhook: WebhookTemplate{
				URL:    "{{checker_endpoint}}/check-ssl",
				Method: "POST",
				Body:   `{"domains": {{domains}}, "alert_days": {{alert_days}}}`,
			},
			Timeout:     "5m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "checker_endpoint", Description: "SSL checker service endpoint", Type: "string", Required: true},
				{Name: "domains", Description: "JSON array of domains to check", Type: "string", Required: true},
				{Name: "alert_days", Description: "Days before expiry to alert", Type: "number", Required: false, Default: 30},
			},
			Downloads: 780,
			Rating:    4.7,
		},
		// Notification templates
		{
			ID:          "slack-daily-report",
			Name:        "Slack Daily Report",
			Description: "Send daily summary report to Slack channel",
			Category:    CategoryNotification,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"slack", "notification", "report", "daily"},
			Schedule:    "0 9 * * 1-5",
			Timezone:    "America/New_York",
			Webhook: WebhookTemplate{
				URL:    "{{slack_webhook_url}}",
				Method: "POST",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"channel": "{{channel}}", "text": "Daily report for {{report_date}}"}`,
			},
			Timeout:     "1m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "slack_webhook_url", Description: "Slack incoming webhook URL", Type: "secret", Required: true},
				{Name: "channel", Description: "Slack channel name", Type: "string", Required: true, Default: "#general"},
				{Name: "report_date", Description: "Report date placeholder", Type: "string", Required: false, Default: "${DATE}"},
			},
			Downloads: 1560,
			Rating:    4.5,
		},
		// Data pipeline templates
		{
			ID:          "etl-trigger",
			Name:        "ETL Pipeline Trigger",
			Description: "Trigger ETL pipeline execution",
			Category:    CategoryDataPipeline,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"etl", "data", "pipeline", "airflow"},
			Schedule:    "0 1 * * *",
			Timezone:    "UTC",
			Webhook: WebhookTemplate{
				URL:    "{{airflow_url}}/api/v1/dags/{{dag_id}}/dagRuns",
				Method: "POST",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"conf": {"triggered_by": "chronos"}}`,
			},
			Timeout:     "5m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "airflow_url", Description: "Airflow base URL", Type: "string", Required: true},
				{Name: "dag_id", Description: "DAG ID to trigger", Type: "string", Required: true},
			},
			Downloads: 650,
			Rating:    4.4,
		},
		// Maintenance templates
		{
			ID:          "cache-clear",
			Name:        "Cache Clear",
			Description: "Clear application or CDN cache",
			Category:    CategoryMaintenance,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"cache", "cdn", "maintenance", "redis"},
			Schedule:    "0 4 * * 0",
			Webhook: WebhookTemplate{
				URL:    "{{cache_endpoint}}/clear",
				Method: "POST",
				Body:   `{"pattern": "{{pattern}}", "force": true}`,
			},
			Timeout:     "10m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "cache_endpoint", Description: "Cache management endpoint", Type: "string", Required: true},
				{Name: "pattern", Description: "Cache key pattern to clear", Type: "string", Required: false, Default: "*"},
			},
			Downloads: 420,
			Rating:    4.3,
		},
		{
			ID:          "log-rotation",
			Name:        "Log Rotation",
			Description: "Rotate and archive application logs",
			Category:    CategoryMaintenance,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"logs", "rotation", "archive", "maintenance"},
			Schedule:    "0 0 * * *",
			Webhook: WebhookTemplate{
				URL:    "{{log_service}}/rotate",
				Method: "POST",
				Body:   `{"retention_days": {{retention_days}}, "compress": true}`,
			},
			Timeout:     "30m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "log_service", Description: "Log management service endpoint", Type: "string", Required: true},
				{Name: "retention_days", Description: "Days to retain logs", Type: "number", Required: false, Default: 30},
			},
			Downloads: 380,
			Rating:    4.2,
		},
		// Reporting templates
		{
			ID:          "weekly-analytics",
			Name:        "Weekly Analytics Report",
			Description: "Generate and send weekly analytics report",
			Category:    CategoryReporting,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"analytics", "report", "weekly", "email"},
			Schedule:    "0 8 * * 1",
			Timezone:    "America/New_York",
			Webhook: WebhookTemplate{
				URL:    "{{analytics_endpoint}}/report/weekly",
				Method: "POST",
				Body:   `{"recipients": {{recipients}}, "metrics": ["pageviews", "users", "conversions"]}`,
			},
			Timeout:     "15m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "analytics_endpoint", Description: "Analytics service endpoint", Type: "string", Required: true},
				{Name: "recipients", Description: "JSON array of email recipients", Type: "string", Required: true},
			},
			Downloads: 520,
			Rating:    4.5,
		},
		// Integration templates
		{
			ID:          "github-action-trigger",
			Name:        "GitHub Actions Trigger",
			Description: "Trigger GitHub Actions workflow on schedule",
			Category:    CategoryIntegration,
			Author:      "Chronos Team",
			Version:     "1.0.0",
			Tags:        []string{"github", "ci", "cd", "actions"},
			Schedule:    "0 6 * * *",
			Webhook: WebhookTemplate{
				URL:    "https://api.github.com/repos/{{owner}}/{{repo}}/actions/workflows/{{workflow}}/dispatches",
				Method: "POST",
				Headers: map[string]string{
					"Accept":        "application/vnd.github+json",
					"Authorization": "Bearer {{github_token}}",
				},
				Body: `{"ref": "{{branch}}"}`,
			},
			Timeout:     "1m",
			Concurrency: "forbid",
			Variables: []TemplateVariable{
				{Name: "owner", Description: "GitHub repository owner", Type: "string", Required: true},
				{Name: "repo", Description: "GitHub repository name", Type: "string", Required: true},
				{Name: "workflow", Description: "Workflow filename (e.g., ci.yml)", Type: "string", Required: true},
				{Name: "branch", Description: "Branch to run workflow on", Type: "string", Required: false, Default: "main"},
				{Name: "github_token", Description: "GitHub personal access token", Type: "secret", Required: true},
			},
			Downloads: 890,
			Rating:    4.7,
		},
	}

	for _, t := range templates {
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()
		m.templates[t.ID] = t
	}
}

// substituteVars replaces {{var}} placeholders with values.
func substituteVars(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

// substituteMapVars substitutes variables in a map.
func substituteMapVars(m map[string]string, vars map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		result[k] = substituteVars(v, vars)
	}
	return result
}

// toString converts a value to string.
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return json.Number(string(rune(val))).String()
	case float64:
		b, _ := json.Marshal(val)
		return string(b)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
