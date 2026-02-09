// Package nljob provides natural language job creation using LLM integration.
package nljob

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ProviderType represents the type of LLM provider.
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderLocal     ProviderType = "local"
	ProviderMock      ProviderType = "mock" // For testing
)

// Config configures the NL job creator.
type Config struct {
	Provider      ProviderType `json:"provider"`
	APIKey        string       `json:"api_key,omitempty"`
	APIEndpoint   string       `json:"api_endpoint,omitempty"`
	Model         string       `json:"model"`
	MaxTokens     int          `json:"max_tokens"`
	Temperature   float64      `json:"temperature"`
	SystemPrompt  string       `json:"system_prompt,omitempty"`
	ValidateJobs  bool         `json:"validate_jobs"`
	DefaultTags   map[string]string `json:"default_tags,omitempty"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Provider:     ProviderMock,
		Model:        "gpt-4",
		MaxTokens:    2000,
		Temperature:  0.3,
		ValidateJobs: true,
		SystemPrompt: defaultSystemPrompt,
		DefaultTags:  map[string]string{"created_by": "nl-assistant"},
	}
}

const defaultSystemPrompt = `You are Chronos, an intelligent job scheduler assistant. 
Your task is to convert natural language descriptions into valid Chronos job configurations.

When creating jobs, consider:
1. Schedule format: Use standard cron syntax (minute hour day month weekday) or special keywords like @hourly, @daily
2. Retry policy: Consider appropriate retry counts and delays based on the job type
3. Timeouts: Set reasonable timeouts based on expected job duration
4. Error handling: Configure alerts and notification preferences

Always output valid JSON that matches the Chronos job schema.`

// ParseResult contains the result of parsing natural language.
type ParseResult struct {
	Jobs       []*models.Job         `json:"jobs,omitempty"`
	Workflows  []*WorkflowSuggestion `json:"workflows,omitempty"`
	Confidence float64               `json:"confidence"`
	Warnings   []string              `json:"warnings,omitempty"`
	Suggestions []string             `json:"suggestions,omitempty"`
}

// WorkflowSuggestion suggests a multi-job workflow.
type WorkflowSuggestion struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Dependencies map[string][]string `json:"dependencies"`
}

// LLMProvider is the interface for LLM backends.
type LLMProvider interface {
	Complete(ctx context.Context, prompt string) (string, error)
	Name() string
}

// Creator handles natural language job creation.
type Creator struct {
	config   Config
	provider LLMProvider
	logger   zerolog.Logger
}

// NewCreator creates a new NL job creator.
func NewCreator(config Config, logger zerolog.Logger) *Creator {
	var provider LLMProvider

	switch config.Provider {
	case ProviderMock:
		provider = &MockProvider{}
	case ProviderOpenAI:
		provider = &OpenAIProvider{
			APIKey:   config.APIKey,
			Endpoint: config.APIEndpoint,
			Model:    config.Model,
		}
	case ProviderAnthropic:
		provider = &AnthropicProvider{
			APIKey:   config.APIKey,
			Endpoint: config.APIEndpoint,
			Model:    config.Model,
		}
	default:
		provider = &MockProvider{}
	}

	return &Creator{
		config:   config,
		provider: provider,
		logger:   logger.With().Str("component", "nljob").Logger(),
	}
}

// ParseDescription converts natural language to job configuration.
func (c *Creator) ParseDescription(ctx context.Context, description string) (*ParseResult, error) {
	prompt := c.buildPrompt(description)

	response, err := c.provider.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	result, err := c.parseResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Apply defaults and validate
	for _, job := range result.Jobs {
		c.applyDefaults(job)
		if c.config.ValidateJobs {
			if warnings := c.validateJob(job); len(warnings) > 0 {
				result.Warnings = append(result.Warnings, warnings...)
			}
		}
	}

	c.logger.Info().
		Str("description", truncate(description, 100)).
		Int("jobs_created", len(result.Jobs)).
		Float64("confidence", result.Confidence).
		Msg("Parsed natural language job description")

	return result, nil
}

func (c *Creator) buildPrompt(description string) string {
	return fmt.Sprintf(`%s

User Request:
%s

Generate a JSON response with the following structure:
{
  "jobs": [
    {
      "name": "string",
      "description": "string",
      "schedule": "cron expression or special keyword",
      "enabled": boolean,
      "webhook": {
        "url": "string",
        "method": "GET/POST/PUT",
        "headers": {},
        "body": "string"
      },
      "retry_policy": {
        "max_retries": number,
        "initial_delay": "duration string like 1m",
        "max_delay": "duration string",
        "backoff_multiplier": number
      },
      "timeout": "duration string",
      "tags": {}
    }
  ],
  "confidence": 0.0-1.0,
  "suggestions": ["optional improvement suggestions"]
}

Respond only with valid JSON:`, c.config.SystemPrompt, description)
}

func (c *Creator) parseResponse(response string) (*ParseResult, error) {
	// Extract JSON from response (LLMs sometimes include explanation text)
	jsonStr := extractJSON(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var result struct {
		Jobs        []json.RawMessage `json:"jobs"`
		Confidence  float64           `json:"confidence"`
		Suggestions []string          `json:"suggestions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	parseResult := &ParseResult{
		Jobs:        make([]*models.Job, 0, len(result.Jobs)),
		Confidence:  result.Confidence,
		Suggestions: result.Suggestions,
	}

	for _, jobJSON := range result.Jobs {
		job := &models.Job{}
		if err := json.Unmarshal(jobJSON, job); err != nil {
			parseResult.Warnings = append(parseResult.Warnings,
				fmt.Sprintf("Failed to parse job: %v", err))
			continue
		}

		// Generate ID if not set
		if job.ID == "" {
			job.ID = uuid.New().String()
		}

		parseResult.Jobs = append(parseResult.Jobs, job)
	}

	return parseResult, nil
}

func (c *Creator) applyDefaults(job *models.Job) {
	now := time.Now().UTC()

	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	job.Version = 1

	// Apply default tags
	if job.Tags == nil {
		job.Tags = make(map[string]string)
	}
	for k, v := range c.config.DefaultTags {
		if _, exists := job.Tags[k]; !exists {
			job.Tags[k] = v
		}
	}

	// Ensure retry policy
	if job.RetryPolicy == nil {
		job.RetryPolicy = models.DefaultRetryPolicy()
	}
}

func (c *Creator) validateJob(job *models.Job) []string {
	var warnings []string

	if job.Name == "" {
		warnings = append(warnings, "Job name is empty")
	}

	if job.Schedule == "" {
		warnings = append(warnings, "Job schedule is empty")
	} else if !isValidSchedule(job.Schedule) {
		warnings = append(warnings, fmt.Sprintf("Schedule '%s' may be invalid", job.Schedule))
	}

	if job.Webhook == nil {
		warnings = append(warnings, "Job has no webhook configured")
	}

	return warnings
}

// SuggestSchedule suggests a cron schedule from natural language.
func (c *Creator) SuggestSchedule(ctx context.Context, description string) (string, error) {
	// Try to parse common patterns first
	if schedule := parseCommonSchedulePatterns(description); schedule != "" {
		return schedule, nil
	}

	// Fall back to LLM
	prompt := fmt.Sprintf(`Convert this schedule description to a cron expression:
"%s"

Respond with ONLY the cron expression (5 or 6 fields). Examples:
- "every hour" -> "0 * * * *"
- "daily at 9am" -> "0 9 * * *"
- "every monday at 8:30" -> "30 8 * * 1"

Cron expression:`, description)

	response, err := c.provider.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}

	// Clean up response
	schedule := strings.TrimSpace(response)
	schedule = strings.Trim(schedule, "\"'`")

	return schedule, nil
}

// ExplainJob provides a natural language explanation of a job configuration.
func (c *Creator) ExplainJob(ctx context.Context, job *models.Job) (string, error) {
	jobJSON, _ := json.MarshalIndent(job, "", "  ")

	prompt := fmt.Sprintf(`Explain this Chronos job configuration in simple terms:

%s

Provide a brief, user-friendly explanation including:
1. What the job does
2. When it runs (schedule explained)
3. Any retry or error handling behavior
4. Important settings to note

Keep the explanation concise (2-3 paragraphs).`, string(jobJSON))

	return c.provider.Complete(ctx, prompt)
}

// Helper functions

func extractJSON(s string) string {
	// Try to find JSON object
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}

func isValidSchedule(schedule string) bool {
	// Check special keywords
	specials := []string{"@yearly", "@annually", "@monthly", "@weekly", "@daily", "@midnight", "@hourly"}
	for _, s := range specials {
		if schedule == s {
			return true
		}
	}

	// Check cron format (5 or 6 fields)
	fields := strings.Fields(schedule)
	return len(fields) >= 5 && len(fields) <= 6
}

func parseCommonSchedulePatterns(description string) string {
	desc := strings.ToLower(description)

	patterns := map[string]string{
		"every hour":              "0 * * * *",
		"hourly":                  "0 * * * *",
		"every day":               "0 0 * * *",
		"daily":                   "0 0 * * *",
		"every week":              "0 0 * * 0",
		"weekly":                  "0 0 * * 0",
		"every month":             "0 0 1 * *",
		"monthly":                 "0 0 1 * *",
		"every minute":            "* * * * *",
		"every 5 minutes":         "*/5 * * * *",
		"every 10 minutes":        "*/10 * * * *",
		"every 15 minutes":        "*/15 * * * *",
		"every 30 minutes":        "*/30 * * * *",
		"midnight":                "0 0 * * *",
		"noon":                    "0 12 * * *",
		"monday":                  "0 0 * * 1",
		"every monday":            "0 0 * * 1",
		"tuesday":                 "0 0 * * 2",
		"wednesday":               "0 0 * * 3",
		"thursday":                "0 0 * * 4",
		"friday":                  "0 0 * * 5",
		"weekdays":                "0 0 * * 1-5",
		"weekends":                "0 0 * * 0,6",
	}

	for pattern, cron := range patterns {
		if strings.Contains(desc, pattern) {
			return cron
		}
	}

	// Try to parse "at X:XX" patterns
	timePattern := regexp.MustCompile(`at (\d{1,2}):?(\d{2})?\s*(am|pm)?`)
	if matches := timePattern.FindStringSubmatch(desc); len(matches) > 0 {
		hour := parseInt(matches[1])
		minute := 0
		if matches[2] != "" {
			minute = parseInt(matches[2])
		}
		if matches[3] == "pm" && hour < 12 {
			hour += 12
		} else if matches[3] == "am" && hour == 12 {
			hour = 0
		}
		return fmt.Sprintf("%d %d * * *", minute, hour)
	}

	return ""
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// MockProvider is a mock LLM provider for testing.
type MockProvider struct{}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Complete(ctx context.Context, prompt string) (string, error) {
	// Return a mock response based on common keywords
	if strings.Contains(strings.ToLower(prompt), "backup") {
		return `{
  "jobs": [{
    "name": "daily-backup",
    "description": "Daily database backup job",
    "schedule": "0 2 * * *",
    "enabled": true,
    "webhook": {
      "url": "http://localhost:8080/backup",
      "method": "POST"
    },
    "retry_policy": {
      "max_retries": 3,
      "initial_delay": "1m",
      "max_delay": "10m",
      "backoff_multiplier": 2
    },
    "timeout": "30m"
  }],
  "confidence": 0.9,
  "suggestions": ["Consider adding alerting for backup failures"]
}`, nil
	}

	return `{
  "jobs": [{
    "name": "scheduled-job",
    "description": "Scheduled task",
    "schedule": "0 * * * *",
    "enabled": true,
    "webhook": {
      "url": "http://localhost:8080/task",
      "method": "POST"
    }
  }],
  "confidence": 0.7
}`, nil
}

// OpenAIProvider implements LLM completion using OpenAI API.
type OpenAIProvider struct {
	APIKey   string
	Endpoint string
	Model    string
}

func (p *OpenAIProvider) Name() string { return "openai" }

func (p *OpenAIProvider) Complete(ctx context.Context, prompt string) (string, error) {
	// Placeholder - would use OpenAI API client
	return "", fmt.Errorf("OpenAI provider not yet implemented - set OPENAI_API_KEY and implement HTTP client")
}

// AnthropicProvider implements LLM completion using Anthropic API.
type AnthropicProvider struct {
	APIKey   string
	Endpoint string
	Model    string
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) Complete(ctx context.Context, prompt string) (string, error) {
	// Placeholder - would use Anthropic API client
	return "", fmt.Errorf("Anthropic provider not yet implemented - set ANTHROPIC_API_KEY and implement HTTP client")
}
