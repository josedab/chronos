package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func resetRegistry() {
	// Create a new registry for each test to avoid conflicts
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
}

func TestNew(t *testing.T) {
	resetRegistry()

	m := New()

	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.JobsTotal == nil {
		t.Error("JobsTotal not initialized")
	}
	if m.JobsEnabled == nil {
		t.Error("JobsEnabled not initialized")
	}
	if m.ExecutionsTotal == nil {
		t.Error("ExecutionsTotal not initialized")
	}
	if m.ExecutionDuration == nil {
		t.Error("ExecutionDuration not initialized")
	}
	if m.RaftIsLeader == nil {
		t.Error("RaftIsLeader not initialized")
	}
	if m.RaftPeers == nil {
		t.Error("RaftPeers not initialized")
	}
}

func TestHandler(t *testing.T) {
	handler := Handler()

	if handler == nil {
		t.Fatal("Handler returned nil")
	}

	// Test that handler responds
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMetrics_RecordExecution(t *testing.T) {
	resetRegistry()
	m := New()

	// Should not panic
	m.RecordExecution("job-123", "test-job", "success", 1.5)
	m.RecordExecution("job-123", "test-job", "failed", 0.5)
}

func TestMetrics_RecordHTTPRequest(t *testing.T) {
	resetRegistry()
	m := New()

	// Should not panic
	m.RecordHTTPRequest("GET", "/api/v1/jobs", "200", 0.1)
	m.RecordHTTPRequest("POST", "/api/v1/jobs", "201", 0.2)
	m.RecordHTTPRequest("GET", "/api/v1/jobs/123", "404", 0.05)
}

func TestMetrics_SetLeader(t *testing.T) {
	resetRegistry()
	m := New()

	// Set as leader
	m.SetLeader(true)

	// Set as follower
	m.SetLeader(false)
}

func TestMetrics_SetJobCounts(t *testing.T) {
	resetRegistry()
	m := New()

	m.SetJobCounts(10, 8)
	m.SetJobCounts(15, 12)
	m.SetJobCounts(0, 0)
}

func TestMetrics_SetPeerCount(t *testing.T) {
	resetRegistry()
	m := New()

	m.SetPeerCount(3)
	m.SetPeerCount(5)
	m.SetPeerCount(1)
}

func TestMetrics_Handler_ReturnsValidHTTP(t *testing.T) {
	// Just test the handler returns a valid response
	handler := Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty response body")
	}
}
