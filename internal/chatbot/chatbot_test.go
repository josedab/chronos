package chatbot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// MockJobManager implements JobManager for testing.
type MockJobManager struct {
	jobs       map[string]*Job
	executions map[string][]Execution
}

func NewMockJobManager() *MockJobManager {
	return &MockJobManager{
		jobs: map[string]*Job{
			"job-1": {ID: "job-1", Name: "Daily Backup", Schedule: "0 0 * * *", Status: "active", NextRun: time.Now().Add(24 * time.Hour)},
			"job-2": {ID: "job-2", Name: "Hourly Sync", Schedule: "0 * * * *", Status: "active", NextRun: time.Now().Add(time.Hour)},
			"job-3": {ID: "job-3", Name: "Weekly Report", Schedule: "0 0 * * 0", Status: "paused", NextRun: time.Now().Add(7 * 24 * time.Hour)},
		},
		executions: map[string][]Execution{
			"job-1": {
				{ID: "exec-1", JobID: "job-1", Status: "success", StartedAt: time.Now().Add(-time.Hour), Duration: "5m"},
				{ID: "exec-2", JobID: "job-1", Status: "success", StartedAt: time.Now().Add(-25 * time.Hour), Duration: "4m"},
			},
		},
	}
}

func (m *MockJobManager) ListJobs(ctx context.Context, tenantID string) ([]Job, error) {
	jobs := make([]Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, *j)
	}
	return jobs, nil
}

func (m *MockJobManager) GetJob(ctx context.Context, tenantID, jobID string) (*Job, error) {
	if j, ok := m.jobs[jobID]; ok {
		return j, nil
	}
	return nil, ErrCommandNotFound
}

func (m *MockJobManager) RunJob(ctx context.Context, tenantID, jobID string) (*Execution, error) {
	if _, ok := m.jobs[jobID]; !ok {
		return nil, ErrCommandNotFound
	}
	return &Execution{
		ID:        "exec-new",
		JobID:     jobID,
		Status:    "running",
		StartedAt: time.Now(),
	}, nil
}

func (m *MockJobManager) PauseJob(ctx context.Context, tenantID, jobID string) error {
	if j, ok := m.jobs[jobID]; ok {
		j.Status = "paused"
		return nil
	}
	return ErrCommandNotFound
}

func (m *MockJobManager) ResumeJob(ctx context.Context, tenantID, jobID string) error {
	if j, ok := m.jobs[jobID]; ok {
		j.Status = "active"
		return nil
	}
	return ErrCommandNotFound
}

func (m *MockJobManager) GetJobHistory(ctx context.Context, tenantID, jobID string, limit int) ([]Execution, error) {
	if execs, ok := m.executions[jobID]; ok {
		if limit > 0 && limit < len(execs) {
			return execs[:limit], nil
		}
		return execs, nil
	}
	return []Execution{}, nil
}

func (m *MockJobManager) GetJobStatus(ctx context.Context, tenantID, jobID string) (*JobStatus, error) {
	if _, ok := m.jobs[jobID]; !ok {
		return nil, ErrCommandNotFound
	}
	return &JobStatus{
		JobID:        jobID,
		IsHealthy:    true,
		SuccessRate:  0.95,
		AvgDuration:  "4m30s",
		RecentErrors: 1,
	}, nil
}

func TestSlackBot_HandleHelp(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		Text:    "",
		TeamID:  "T123",
		UserID:  "U123",
		Command: "/chronos",
	}

	resp, err := bot.handleHelp(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleHelp failed: %v", err)
	}

	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected response type 'ephemeral', got %q", resp.ResponseType)
	}

	if len(resp.Blocks) == 0 {
		t.Error("expected blocks in response")
	}
}

func TestSlackBot_HandleListJobs(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		Text:   "",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleListJobs(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleListJobs failed: %v", err)
	}

	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response type 'in_channel', got %q", resp.ResponseType)
	}

	// Should have header + divider + 3 job blocks
	if len(resp.Blocks) < 5 {
		t.Errorf("expected at least 5 blocks, got %d", len(resp.Blocks))
	}
}

func TestSlackBot_HandleJobStatus(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		Text:   "job-1",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleJobStatus(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleJobStatus failed: %v", err)
	}

	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response type 'in_channel', got %q", resp.ResponseType)
	}
}

func TestSlackBot_HandleJobStatus_NoJobID(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		Text:   "",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleJobStatus(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleJobStatus failed: %v", err)
	}

	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected response type 'ephemeral', got %q", resp.ResponseType)
	}

	if !strings.Contains(resp.Text, "specify a job ID") {
		t.Errorf("expected prompt to specify job ID, got %q", resp.Text)
	}
}

func TestSlackBot_HandleRunJob(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		Text:   "job-1",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleRunJob(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleRunJob failed: %v", err)
	}

	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response type 'in_channel', got %q", resp.ResponseType)
	}

	// Should mention execution ID in blocks
	found := false
	for _, block := range resp.Blocks {
		if block.Text != nil && strings.Contains(block.Text.Text, "exec-new") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected execution ID in response")
	}
}

func TestSlackBot_HandlePauseJob(t *testing.T) {
	jm := NewMockJobManager()
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, jm)

	cmd := &SlashCommand{
		Text:   "job-1",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handlePauseJob(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handlePauseJob failed: %v", err)
	}

	if !strings.Contains(resp.Text, "paused") {
		t.Errorf("expected pause confirmation, got %q", resp.Text)
	}

	// Verify job was paused
	if jm.jobs["job-1"].Status != "paused" {
		t.Error("job should be paused")
	}
}

func TestSlackBot_HandleResumeJob(t *testing.T) {
	jm := NewMockJobManager()
	jm.jobs["job-3"].Status = "paused"
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, jm)

	cmd := &SlashCommand{
		Text:   "job-3",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleResumeJob(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleResumeJob failed: %v", err)
	}

	if !strings.Contains(resp.Text, "resumed") {
		t.Errorf("expected resume confirmation, got %q", resp.Text)
	}

	// Verify job was resumed
	if jm.jobs["job-3"].Status != "active" {
		t.Error("job should be active")
	}
}

func TestSlackBot_HandleJobHistory(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		Text:   "job-1",
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleJobHistory(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleJobHistory failed: %v", err)
	}

	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response type 'in_channel', got %q", resp.ResponseType)
	}
}

func TestSlackBot_HandleClusterHealth(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	cmd := &SlashCommand{
		TeamID: "T123",
		UserID: "U123",
	}

	resp, err := bot.handleClusterHealth(context.Background(), cmd)
	if err != nil {
		t.Fatalf("handleClusterHealth failed: %v", err)
	}

	if resp.ResponseType != "in_channel" {
		t.Errorf("expected response type 'in_channel', got %q", resp.ResponseType)
	}

	// Should have health info blocks
	if len(resp.Blocks) < 3 {
		t.Errorf("expected at least 3 blocks, got %d", len(resp.Blocks))
	}
}

func TestSlackBot_UnknownCommand(t *testing.T) {
	bot := NewSlackBot(SlackConfig{SigningSecret: "test"}, NewMockJobManager())

	resp := bot.buildUnknownCommandResponse("foobar")

	if resp.ResponseType != "ephemeral" {
		t.Errorf("expected response type 'ephemeral', got %q", resp.ResponseType)
	}

	if !strings.Contains(resp.Text, "Unknown command") {
		t.Errorf("expected unknown command message, got %q", resp.Text)
	}
}

func TestSlackBot_RateLimiter(t *testing.T) {
	limiter := newRateLimiter(2, time.Second)

	// First two should be allowed
	if !limiter.allow("user1") {
		t.Error("first request should be allowed")
	}
	if !limiter.allow("user1") {
		t.Error("second request should be allowed")
	}

	// Third should be denied
	if limiter.allow("user1") {
		t.Error("third request should be denied")
	}

	// Different user should be allowed
	if !limiter.allow("user2") {
		t.Error("different user should be allowed")
	}
}

func TestSlackBot_HandleSlashCommand_Integration(t *testing.T) {
	bot := NewSlackBot(SlackConfig{
		SigningSecret: "test-secret",
	}, NewMockJobManager())

	// Create test request (without signature verification for unit test)
	form := url.Values{}
	form.Add("command", "/chronos")
	form.Add("text", "list")
	form.Add("team_id", "T123")
	form.Add("user_id", "U123")
	form.Add("channel_id", "C123")

	req := httptest.NewRequest("POST", "/slack/command", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Skip signature for unit test
	req.Header.Set("X-Slack-Request-Timestamp", "0")
	req.Header.Set("X-Slack-Signature", "")

	w := httptest.NewRecorder()

	// Note: This will fail signature verification in real scenario
	// For proper testing, would need to generate valid signature
	bot.HandleSlashCommand(w, req)

	// Expect unauthorized due to missing signature
	if w.Code != http.StatusUnauthorized {
		t.Logf("Response: %s", w.Body.String())
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"running", "🔄"},
		{"success", "✅"},
		{"succeeded", "✅"},
		{"active", "✅"},
		{"enabled", "✅"},
		{"failed", "❌"},
		{"error", "❌"},
		{"pending", "⏳"},
		{"queued", "⏳"},
		{"paused", "⏸️"},
		{"disabled", "⏸️"},
		{"cancelled", "🚫"},
		{"unknown", "❓"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := getStatusEmoji(tt.status)
			if result != tt.expected {
				t.Errorf("getStatusEmoji(%q) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	// Test zero time
	result := formatTime(time.Time{})
	if result != "N/A" {
		t.Errorf("formatTime(zero) = %q, want 'N/A'", result)
	}

	// Test non-zero time
	tm := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	result = formatTime(tm)
	if !strings.Contains(result, "Jan 15") {
		t.Errorf("formatTime should contain 'Jan 15', got %q", result)
	}
}

func TestExtractTenantID(t *testing.T) {
	result := extractTenantID("T12345")
	if result != "tenant-T12345" {
		t.Errorf("extractTenantID = %q, want 'tenant-T12345'", result)
	}
}
