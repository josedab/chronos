package nljob

import (
	"context"
	"testing"

	"github.com/chronos/chronos/internal/models"
	"github.com/rs/zerolog"
)

func TestCreator_ParseDescription(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultConfig()
	config.Provider = ProviderMock

	creator := NewCreator(config, logger)

	tests := []struct {
		name        string
		description string
		wantJobs    int
		wantErr     bool
	}{
		{
			name:        "backup job",
			description: "Create a backup job that runs at 2am daily",
			wantJobs:    1,
			wantErr:     false,
		},
		{
			name:        "generic job",
			description: "Run a task every hour",
			wantJobs:    1,
			wantErr:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := creator.ParseDescription(context.Background(), tc.description)

			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != nil && len(result.Jobs) != tc.wantJobs {
				t.Errorf("expected %d jobs, got %d", tc.wantJobs, len(result.Jobs))
			}
		})
	}
}

func TestCreator_ParseDescription_BackupJob(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultConfig()
	creator := NewCreator(config, logger)

	result, err := creator.ParseDescription(context.Background(), "Create a daily backup job")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(result.Jobs) == 0 {
		t.Fatal("expected at least one job")
	}

	job := result.Jobs[0]
	if job.Name == "" {
		t.Error("job name should not be empty")
	}
	if job.Schedule == "" {
		t.Error("job schedule should not be empty")
	}
	if job.ID == "" {
		t.Error("job ID should be generated")
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("confidence should be between 0 and 1, got %f", result.Confidence)
	}
}

func TestCreator_SuggestSchedule(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultConfig()
	creator := NewCreator(config, logger)

	tests := []struct {
		description string
		expected    string
	}{
		{"every hour", "0 * * * *"},
		{"daily", "0 0 * * *"},
		{"every 5 minutes", "*/5 * * * *"},
		{"weekly", "0 0 * * 0"},
		{"midnight", "0 0 * * *"},
		{"at 9:00 am", "0 9 * * *"},
		{"at 2:30 pm", "30 14 * * *"},
		{"weekdays", "0 0 * * 1-5"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			schedule, err := creator.SuggestSchedule(context.Background(), tc.description)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if schedule != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, schedule)
			}
		})
	}
}

func TestIsValidSchedule(t *testing.T) {
	tests := []struct {
		schedule string
		valid    bool
	}{
		{"0 * * * *", true},
		{"@hourly", true},
		{"@daily", true},
		{"@weekly", true},
		{"@monthly", true},
		{"*/5 * * * *", true},
		{"0 9 * * 1-5", true},
		{"invalid", false},
		{"0 * *", false},
	}

	for _, tc := range tests {
		t.Run(tc.schedule, func(t *testing.T) {
			if isValidSchedule(tc.schedule) != tc.valid {
				t.Errorf("expected %v for %s", tc.valid, tc.schedule)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `Here's the JSON: {"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			input:    `{"nested": {"inner": 1}}`,
			expected: `{"nested": {"inner": 1}}`,
		},
		{
			input:    `No JSON here`,
			expected: "",
		},
		{
			input:    `Text before {"a": 1} and after`,
			expected: `{"a": 1}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input[:min(20, len(tc.input))], func(t *testing.T) {
			result := extractJSON(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Provider != ProviderMock {
		t.Errorf("expected mock provider, got %s", config.Provider)
	}
	if config.MaxTokens <= 0 {
		t.Error("max tokens should be positive")
	}
	if config.Temperature < 0 || config.Temperature > 1 {
		t.Error("temperature should be between 0 and 1")
	}
	if config.ValidateJobs != true {
		t.Error("validate jobs should default to true")
	}
}

func TestApplyDefaults(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultConfig()
	config.DefaultTags = map[string]string{"source": "test"}
	creator := NewCreator(config, logger)

	job := &models.Job{Name: "test-job"}
	creator.applyDefaults(job)

	if job.ID == "" {
		t.Error("ID should be generated")
	}
	if job.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if job.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
	if job.Tags == nil || job.Tags["source"] != "test" {
		t.Error("default tags should be applied")
	}
	if job.RetryPolicy == nil {
		t.Error("retry policy should be set")
	}
}

func TestValidateJob(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultConfig()
	creator := NewCreator(config, logger)

	tests := []struct {
		name         string
		job          *models.Job
		wantWarnings int
	}{
		{
			name:         "empty job",
			job:          &models.Job{},
			wantWarnings: 3, // name, schedule, webhook
		},
		{
			name: "missing schedule",
			job: &models.Job{
				Name: "test",
				Webhook: &models.WebhookConfig{URL: "http://test"},
			},
			wantWarnings: 1,
		},
		{
			name: "valid job",
			job: &models.Job{
				Name:     "test",
				Schedule: "0 * * * *",
				Webhook:  &models.WebhookConfig{URL: "http://test"},
			},
			wantWarnings: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			warnings := creator.validateJob(tc.job)
			if len(warnings) != tc.wantWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tc.wantWarnings, len(warnings), warnings)
			}
		})
	}
}

func TestMockProvider(t *testing.T) {
	provider := &MockProvider{}

	if provider.Name() != "mock" {
		t.Errorf("expected name 'mock', got %s", provider.Name())
	}

	response, err := provider.Complete(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("mock provider error: %v", err)
	}
	if response == "" {
		t.Error("expected non-empty response")
	}
}

func TestParseCommonSchedulePatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"run every hour", "0 * * * *"},
		{"execute daily", "0 0 * * *"},
		{"trigger every 5 minutes", "*/5 * * * *"},
		{"no pattern here xyz", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := parseCommonSchedulePatterns(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestProviderTypes(t *testing.T) {
	providers := []ProviderType{
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderLocal,
		ProviderMock,
	}

	for _, p := range providers {
		if p == "" {
			t.Error("provider type should not be empty")
		}
	}
}
