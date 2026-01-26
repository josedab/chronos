package cron

import (
	"testing"
	"time"
)

func TestParseStandard(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"every minute", "* * * * *", false},
		{"every hour", "0 * * * *", false},
		{"every day at midnight", "0 0 * * *", false},
		{"every monday at noon", "0 12 * * 1", false},
		{"every 5 minutes", "*/5 * * * *", false},
		{"range", "0-30 * * * *", false},
		{"list", "0,15,30,45 * * * *", false},
		{"complex", "0 9-17 * * 1-5", false},
		{"named month", "0 0 1 JAN *", false},
		{"named day", "0 0 * * MON", false},
		{"invalid field count", "* * *", true},
		{"invalid minute", "60 * * * *", true},
		{"invalid hour", "* 24 * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseStandard(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStandard(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
			}
		})
	}
}

func TestParseExtended(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"every second", "* * * * * *", false},
		{"every minute at second 0", "0 * * * * *", false},
		{"every 10 seconds", "*/10 * * * * *", false},
		{"invalid field count", "* * * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseExtended(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExtended(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
			}
		})
	}
}

func TestParseDescriptors(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"yearly", "@yearly", false},
		{"annually", "@annually", false},
		{"monthly", "@monthly", false},
		{"weekly", "@weekly", false},
		{"daily", "@daily", false},
		{"midnight", "@midnight", false},
		{"hourly", "@hourly", false},
		{"every 5m", "@every 5m", false},
		{"every 1h30m", "@every 1h30m", false},
		{"every 1s", "@every 1s", false},
		{"invalid descriptor", "@invalid", true},
		{"invalid duration", "@every invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
			}
		})
	}
}

func TestScheduleNext(t *testing.T) {
	// Fixed time for testing: 2024-01-15 10:30:00 Monday
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		spec     string
		from     time.Time
		expected time.Time
	}{
		{
			name:     "every minute",
			spec:     "* * * * *",
			from:     baseTime,
			expected: time.Date(2024, 1, 15, 10, 31, 0, 0, time.UTC),
		},
		{
			name:     "every hour",
			spec:     "0 * * * *",
			from:     baseTime,
			expected: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			name:     "daily at midnight",
			spec:     "0 0 * * *",
			from:     baseTime,
			expected: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "weekly on monday",
			spec:     "0 0 * * 1",
			from:     baseTime,
			expected: time.Date(2024, 1, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "every 5 minutes",
			spec:     "*/5 * * * *",
			from:     baseTime,
			expected: time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := Parse(tt.spec)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.spec, err)
			}
			next := sched.Next(tt.from)
			if !next.Equal(tt.expected) {
				t.Errorf("Next(%v) = %v, want %v", tt.from, next, tt.expected)
			}
		})
	}
}

func TestEveryScheduleNext(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		spec     string
		from     time.Time
		expected time.Time
	}{
		{
			name:     "every 5 minutes",
			spec:     "@every 5m",
			from:     baseTime,
			expected: time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC),
		},
		{
			name:     "every 1 hour",
			spec:     "@every 1h",
			from:     baseTime,
			expected: time.Date(2024, 1, 15, 11, 30, 0, 0, time.UTC),
		},
		{
			name:     "every 30 seconds",
			spec:     "@every 30s",
			from:     baseTime,
			expected: time.Date(2024, 1, 15, 10, 30, 30, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := Parse(tt.spec)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.spec, err)
			}
			next := sched.Next(tt.from)
			if !next.Equal(tt.expected) {
				t.Errorf("Next(%v) = %v, want %v", tt.from, next, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		spec    string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 0 * * *", false},
		{"@hourly", false},
		{"invalid", true},
		{"* * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			err := Validate(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	specs := []string{
		"* * * * *",
		"*/5 * * * *",
		"0 9-17 * * 1-5",
		"@hourly",
	}

	for _, spec := range specs {
		b.Run(spec, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = Parse(spec)
			}
		})
	}
}

func BenchmarkNext(b *testing.B) {
	sched, _ := Parse("*/5 * * * *")
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sched.Next(baseTime)
	}
}
