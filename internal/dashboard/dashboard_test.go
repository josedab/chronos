package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestRender_EmptyData(t *testing.T) {
	data := &DashboardData{FetchedAt: time.Now()}
	output := Render(data)

	if !strings.Contains(output, "CHRONOS DASHBOARD") {
		t.Error("expected dashboard title")
	}
	if !strings.Contains(output, "Ctrl+C") {
		t.Error("expected exit hint")
	}
}

func TestRender_WithJobs(t *testing.T) {
	data := &DashboardData{
		FetchedAt: time.Now(),
		Jobs: []JobSummary{
			{ID: "1", Name: "daily-backup", Schedule: "0 2 * * *", Enabled: true},
			{ID: "2", Name: "health-check", Schedule: "*/5 * * * *", Enabled: false},
		},
		Cluster: ClusterInfo{IsLeader: true, JobsTotal: 2, Running: 1},
	}

	output := Render(data)

	if !strings.Contains(output, "daily-backup") {
		t.Error("expected job name in output")
	}
	if !strings.Contains(output, "✓ Leader") {
		t.Error("expected leader indicator")
	}
	if !strings.Contains(output, "disabled") {
		t.Error("expected disabled status for health-check")
	}
}

func TestRender_WithExecutions(t *testing.T) {
	data := &DashboardData{
		FetchedAt: time.Now(),
		Executions: []ExecutionSummary{
			{ID: "e1", JobName: "backup", Status: "success", Duration: 1500, TraceID: "abc123def456"},
			{ID: "e2", JobName: "cleanup", Status: "failed", Duration: 500, Error: "timeout"},
		},
	}

	output := Render(data)

	if !strings.Contains(output, "RECENT EXECUTIONS") {
		t.Error("expected executions section")
	}
	if !strings.Contains(output, "success") {
		t.Error("expected success status")
	}
	if !strings.Contains(output, "abc123de") {
		t.Error("expected truncated trace ID")
	}
}

func TestRender_Follower(t *testing.T) {
	data := &DashboardData{
		FetchedAt: time.Now(),
		Cluster:   ClusterInfo{IsLeader: false},
	}

	output := Render(data)
	if !strings.Contains(output, "○ Follower") {
		t.Error("expected follower indicator")
	}
}

func TestBox(t *testing.T) {
	result := box("TEST", 30)
	if !strings.Contains(result, "TEST") {
		t.Error("expected title in box")
	}
	if !strings.Contains(result, "┌") {
		t.Error("expected box border")
	}
}

func TestTruncStr(t *testing.T) {
	if truncStr("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}
	if truncStr("very long string", 10) != "very lo..." {
		t.Errorf("expected truncation, got %q", truncStr("very long string", 10))
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8080/")
	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("expected trailing slash removed, got %s", c.BaseURL)
	}
}
