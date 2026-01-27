package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDuration_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		expected string
	}{
		{"zero", Duration(0), `"0s"`},
		{"one second", Duration(time.Second), `"1s"`},
		{"one minute", Duration(time.Minute), `"1m0s"`},
		{"complex", Duration(90 * time.Second), `"1m30s"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.duration)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Duration
	}{
		{"string format", `"30s"`, Duration(30 * time.Second)},
		{"minute", `"5m"`, Duration(5 * time.Minute)},
		{"complex", `"1h30m"`, Duration(90 * time.Minute)},
		{"numeric", `1000000000`, Duration(time.Second)}, // nanoseconds
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.input), &d)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if d != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, d)
			}
		})
	}
}

func TestDuration_Duration(t *testing.T) {
	d := Duration(time.Hour)
	if d.Duration() != time.Hour {
		t.Errorf("expected 1h, got %v", d.Duration())
	}
}

func TestJob_Validate(t *testing.T) {
	tests := []struct {
		name    string
		job     *Job
		wantErr error
	}{
		{
			name:    "missing name",
			job:     &Job{},
			wantErr: ErrJobNameRequired,
		},
		{
			name:    "missing schedule",
			job:     &Job{Name: "test"},
			wantErr: ErrScheduleRequired,
		},
		{
			name:    "missing webhook",
			job:     &Job{Name: "test", Schedule: "* * * * *"},
			wantErr: ErrWebhookRequired,
		},
		{
			name:    "missing webhook URL",
			job:     &Job{Name: "test", Schedule: "* * * * *", Webhook: &WebhookConfig{}},
			wantErr: ErrWebhookURLRequired,
		},
		{
			name: "valid job",
			job: &Job{
				Name:     "test",
				Schedule: "* * * * *",
				Webhook:  &WebhookConfig{URL: "http://example.com"},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if err != tt.wantErr {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestJob_Validate_SetsDefaultMethod(t *testing.T) {
	job := &Job{
		Name:     "test",
		Schedule: "* * * * *",
		Webhook:  &WebhookConfig{URL: "http://example.com"},
	}

	err := job.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Webhook.Method != "GET" {
		t.Errorf("expected method GET, got %s", job.Webhook.Method)
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", policy.MaxAttempts)
	}
	if policy.InitialInterval != Duration(time.Second) {
		t.Errorf("expected InitialInterval 1s, got %v", policy.InitialInterval)
	}
	if policy.MaxInterval != Duration(time.Minute) {
		t.Errorf("expected MaxInterval 1m, got %v", policy.MaxInterval)
	}
	if policy.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %f", policy.Multiplier)
	}
}

func TestAuthType_Values(t *testing.T) {
	tests := []struct {
		authType AuthType
		expected string
	}{
		{AuthTypeNone, ""},
		{AuthTypeBasic, "basic"},
		{AuthTypeBearer, "bearer"},
		{AuthTypeAPIKey, "api_key"},
	}

	for _, tt := range tests {
		if string(tt.authType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.authType)
		}
	}
}

func TestConcurrencyPolicy_Values(t *testing.T) {
	tests := []struct {
		policy   ConcurrencyPolicy
		expected string
	}{
		{ConcurrencyAllow, "allow"},
		{ConcurrencyForbid, "forbid"},
		{ConcurrencyReplace, "replace"},
	}

	for _, tt := range tests {
		if string(tt.policy) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.policy)
		}
	}
}

func TestJob_JSON_RoundTrip(t *testing.T) {
	original := &Job{
		ID:          "job-123",
		Name:        "test-job",
		Description: "A test job",
		Schedule:    "*/5 * * * *",
		Timezone:    "America/New_York",
		Webhook: &WebhookConfig{
			URL:    "https://api.example.com/webhook",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"action":"run"}`,
		},
		RetryPolicy: &RetryPolicy{
			MaxAttempts:     5,
			InitialInterval: Duration(10 * time.Second),
			MaxInterval:     Duration(5 * time.Minute),
			Multiplier:      1.5,
		},
		Timeout:     Duration(30 * time.Second),
		Concurrency: ConcurrencyForbid,
		Enabled:     true,
		CreatedAt:   time.Now().Truncate(time.Second),
		UpdatedAt:   time.Now().Truncate(time.Second),
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal
	var decoded Job
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Compare key fields
	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: %s vs %s", decoded.ID, original.ID)
	}
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch")
	}
	if decoded.Webhook.URL != original.Webhook.URL {
		t.Errorf("Webhook URL mismatch")
	}
	if decoded.RetryPolicy.MaxAttempts != original.RetryPolicy.MaxAttempts {
		t.Errorf("RetryPolicy MaxAttempts mismatch")
	}
}
