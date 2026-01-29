package assistant

import (
	"testing"
)

func TestParse(t *testing.T) {
	a := New("UTC")

	tests := []struct {
		name       string
		input      string
		wantCron   string
		minConf    float64
	}{
		{
			name:     "every 5 minutes",
			input:    "every 5 minutes",
			wantCron: "*/5 * * * *",
			minConf:  0.8,
		},
		{
			name:     "every hour",
			input:    "every hour",
			wantCron: "0 * * * *",
			minConf:  0.8,
		},
		{
			name:     "every day",
			input:    "every day",
			wantCron: "0 0 * * *",
			minConf:  0.8,
		},
		{
			name:     "daily at 9am",
			input:    "daily at 9am",
			wantCron: "0 9 * * *",
			minConf:  0.8,
		},
		{
			name:     "daily at 3pm",
			input:    "daily at 3pm",
			wantCron: "0 15 * * *",
			minConf:  0.8,
		},
		{
			name:     "every Monday at 9am",
			input:    "every Monday at 9am",
			wantCron: "0 9 * * 1",
			minConf:  0.8,
		},
		{
			name:     "every 30 minutes",
			input:    "every 30 minutes",
			wantCron: "*/30 * * * *",
			minConf:  0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := a.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if result.Schedule != tt.wantCron {
				t.Errorf("expected schedule %q, got %q", tt.wantCron, result.Schedule)
			}

			if result.Confidence < tt.minConf {
				t.Errorf("expected confidence >= %f, got %f", tt.minConf, result.Confidence)
			}
		})
	}
}

func TestExtractTimezone(t *testing.T) {
	a := New("UTC")

	tests := []struct {
		input    string
		expected string
	}{
		{"run at 9am in eastern time", "America/New_York"},
		{"daily at 5pm pacific time", "America/Los_Angeles"},
		{"every hour utc", "UTC"},
		{"run at midnight", "UTC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tz := a.extractTimezone(tt.input)
			if tz != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tz)
			}
		})
	}
}

func TestSuggest(t *testing.T) {
	a := New("UTC")

	suggestions := a.Suggest("")
	if len(suggestions) < 5 {
		t.Errorf("expected at least 5 suggestions, got %d", len(suggestions))
	}

	for _, s := range suggestions {
		if s.Schedule == "" {
			t.Error("suggestion should have a schedule")
		}
		if s.Description == "" {
			t.Error("suggestion should have a description")
		}
	}
}

func TestExplainCron(t *testing.T) {
	tests := []struct {
		cron     string
		contains string
	}{
		{"* * * * *", "every minute"},
		{"0 * * * *", "every hour"},
		{"0 9 * * *", "9AM"},
		{"0 0 * * 0", "Sunday"},
	}

	for _, tt := range tests {
		t.Run(tt.cron, func(t *testing.T) {
			explanation := ExplainCron(tt.cron)
			if explanation == "" {
				t.Error("explanation should not be empty")
			}
		})
	}
}

func TestValidateCron(t *testing.T) {
	tests := []struct {
		cron    string
		valid   bool
	}{
		{"* * * * *", true},
		{"0 0 * * *", true},
		{"0 9 * * 1-5", true},
		{"invalid", false},
		{"* * *", false},
	}

	for _, tt := range tests {
		t.Run(tt.cron, func(t *testing.T) {
			err := ValidateCron(tt.cron)
			if (err == nil) != tt.valid {
				t.Errorf("expected valid=%v, got error=%v", tt.valid, err)
			}
		})
	}
}
