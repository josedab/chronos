// Package e2e provides end-to-end tests for Chronos.
// These tests spin up a full API server with in-memory storage and validate
// the complete request lifecycle.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/api"
	"github.com/chronos/chronos/internal/dispatcher"
	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/scheduler"
	"github.com/chronos/chronos/internal/storage"
	"github.com/rs/zerolog"
)

// testServer creates a full Chronos API server for testing.
type testServer struct {
	server     *httptest.Server
	store      *storage.MemoryStore
	scheduler  *scheduler.Scheduler
	dispatcher *dispatcher.Dispatcher
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	logger := zerolog.Nop()
	store := storage.NewMemoryStore()
	disp := dispatcher.New(&dispatcher.Config{
		NodeID:        "test-node-1",
		Timeout:       10 * time.Second,
		MaxConcurrent: 50,
	})
	sched := scheduler.New(store, disp, logger, scheduler.DefaultConfig())
	sched.SetLeader(true)

	handler := api.NewHandler(store, sched, logger)
	router := api.NewRouter(handler, logger)
	server := httptest.NewServer(router)

	return &testServer{
		server:     server,
		store:      store,
		scheduler:  sched,
		dispatcher: disp,
	}
}

func (ts *testServer) close() {
	ts.server.Close()
}

func (ts *testServer) url(path string) string {
	return ts.server.URL + path
}

func (ts *testServer) doJSON(t *testing.T, method, path string, body interface{}) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.url(path), reqBody)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func decodeResponse(t *testing.T, resp *http.Response) api.Response {
	t.Helper()
	defer resp.Body.Close()
	var r api.Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return r
}

// --- E2E Tests ---

func TestHealthCheck(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	resp, err := http.Get(ts.url("/health"))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestJobCRUD(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	// Create a webhook target
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer webhookServer.Close()

	// 1. Create job
	jobReq := map[string]interface{}{
		"name":     "e2e-test-job",
		"schedule": "*/5 * * * *",
		"webhook": map[string]interface{}{
			"url":    webhookServer.URL,
			"method": "POST",
		},
		"retry_policy": map[string]interface{}{
			"max_attempts":     3,
			"initial_interval": "1s",
			"max_interval":     "30s",
			"multiplier":       2.0,
		},
		"timeout": "5m",
		"enabled": true,
	}

	resp := ts.doJSON(t, "POST", "/api/v1/jobs", jobReq)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(body))
	}

	result := decodeResponse(t, resp)
	if !result.Success {
		t.Fatalf("expected success, got error: %+v", result.Error)
	}

	// Extract job ID
	jobData, _ := json.Marshal(result.Data)
	var createdJob models.Job
	json.Unmarshal(jobData, &createdJob)
	jobID := createdJob.ID

	if jobID == "" {
		t.Fatal("expected non-empty job ID")
	}

	// 2. Get job
	resp = ts.doJSON(t, "GET", "/api/v1/jobs/"+jobID, nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	result = decodeResponse(t, resp)
	if !result.Success {
		t.Fatalf("get job failed: %+v", result.Error)
	}

	// 3. List jobs
	resp = ts.doJSON(t, "GET", "/api/v1/jobs", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Update job
	updateReq := map[string]interface{}{
		"name":     "e2e-test-job-updated",
		"schedule": "*/10 * * * *",
		"webhook": map[string]interface{}{
			"url":    webhookServer.URL,
			"method": "POST",
		},
		"enabled": true,
	}
	resp = ts.doJSON(t, "PUT", "/api/v1/jobs/"+jobID, updateReq)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200 on update, got %d: %s", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	// 5. Trigger job manually
	resp = ts.doJSON(t, "POST", "/api/v1/jobs/"+jobID+"/trigger", nil)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 200 on trigger, got %d: %s", resp.StatusCode, string(body))
	}
	resp.Body.Close()

	// 6. Disable job
	resp = ts.doJSON(t, "POST", "/api/v1/jobs/"+jobID+"/disable", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200 on disable, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. Enable job
	resp = ts.doJSON(t, "POST", "/api/v1/jobs/"+jobID+"/enable", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200 on enable, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 8. Delete job
	resp = ts.doJSON(t, "DELETE", "/api/v1/jobs/"+jobID, nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("expected 200 on delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 9. Verify deletion
	resp = ts.doJSON(t, "GET", "/api/v1/jobs/"+jobID, nil)
	if resp.StatusCode != http.StatusNotFound {
		resp.Body.Close()
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestJobTriggerAndExecution(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	// Webhook that records calls
	var callCount int
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Verify Chronos metadata headers
		if r.Header.Get("X-Chronos-Job-ID") == "" {
			t.Error("missing X-Chronos-Job-ID header")
		}
		if r.Header.Get("X-Chronos-Execution-ID") == "" {
			t.Error("missing X-Chronos-Execution-ID header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	// Create job
	jobReq := map[string]interface{}{
		"name":     "trigger-test",
		"schedule": "0 0 * * *",
		"webhook": map[string]interface{}{
			"url":    webhookServer.URL,
			"method": "POST",
			"body":   `{"triggered": true}`,
		},
		"retry_policy": map[string]interface{}{
			"max_attempts":     1,
			"initial_interval": "1s",
			"multiplier":       1.0,
		},
		"enabled": true,
	}

	resp := ts.doJSON(t, "POST", "/api/v1/jobs", jobReq)
	result := decodeResponse(t, resp)
	jobData, _ := json.Marshal(result.Data)
	var job models.Job
	json.Unmarshal(jobData, &job)

	// Trigger execution
	resp = ts.doJSON(t, "POST", "/api/v1/jobs/"+job.ID+"/trigger", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("trigger failed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	if callCount != 1 {
		t.Errorf("expected 1 webhook call, got %d", callCount)
	}

	// Check executions
	resp = ts.doJSON(t, "GET", "/api/v1/jobs/"+job.ID+"/executions", nil)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("list executions failed: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestJobRetryOnFailure(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	callCount := 0
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Fail first 2 attempts, succeed on 3rd
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	jobReq := map[string]interface{}{
		"name":     "retry-test",
		"schedule": "0 0 * * *",
		"webhook": map[string]interface{}{
			"url":    webhookServer.URL,
			"method": "POST",
		},
		"retry_policy": map[string]interface{}{
			"max_attempts":     3,
			"initial_interval": "10ms",
			"max_interval":     "50ms",
			"multiplier":       1.5,
		},
		"enabled": true,
	}

	resp := ts.doJSON(t, "POST", "/api/v1/jobs", jobReq)
	result := decodeResponse(t, resp)
	jobData, _ := json.Marshal(result.Data)
	var job models.Job
	json.Unmarshal(jobData, &job)

	resp = ts.doJSON(t, "POST", "/api/v1/jobs/"+job.ID+"/trigger", nil)
	resp.Body.Close()

	if callCount != 3 {
		t.Errorf("expected 3 attempts (2 fails + 1 success), got %d", callCount)
	}
}

func TestMultipleJobsIsolation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	// Create multiple jobs
	jobIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		jobReq := map[string]interface{}{
			"name":     fmt.Sprintf("isolation-test-%d", i),
			"schedule": "*/5 * * * *",
			"webhook": map[string]interface{}{
				"url":    webhookServer.URL,
				"method": "GET",
			},
			"enabled": true,
		}
		resp := ts.doJSON(t, "POST", "/api/v1/jobs", jobReq)
		result := decodeResponse(t, resp)
		jobData, _ := json.Marshal(result.Data)
		var job models.Job
		json.Unmarshal(jobData, &job)
		jobIDs[i] = job.ID
	}

	// Delete one job shouldn't affect others
	ts.doJSON(t, "DELETE", "/api/v1/jobs/"+jobIDs[2], nil).Body.Close()

	// Verify remaining jobs
	for i, id := range jobIDs {
		if i == 2 {
			continue
		}
		resp := ts.doJSON(t, "GET", "/api/v1/jobs/"+id, nil)
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.Errorf("job %d (%s) should still exist, got %d", i, id, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestInvalidJobValidation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	tests := []struct {
		name   string
		body   map[string]interface{}
		expect int
	}{
		{
			name:   "missing name",
			body:   map[string]interface{}{"schedule": "* * * * *"},
			expect: http.StatusBadRequest,
		},
		{
			name:   "missing schedule",
			body:   map[string]interface{}{"name": "test"},
			expect: http.StatusBadRequest,
		},
		{
			name: "missing webhook",
			body: map[string]interface{}{
				"name":     "test",
				"schedule": "* * * * *",
			},
			expect: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.doJSON(t, "POST", "/api/v1/jobs", tc.body)
			if resp.StatusCode != tc.expect {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				t.Errorf("expected %d, got %d: %s", tc.expect, resp.StatusCode, string(body))
			} else {
				resp.Body.Close()
			}
		})
	}
}

func TestNonexistentJobReturns404(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/jobs/nonexistent"},
		{"PUT", "/api/v1/jobs/nonexistent"},
		{"DELETE", "/api/v1/jobs/nonexistent"},
		{"POST", "/api/v1/jobs/nonexistent/trigger"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var body interface{}
			if ep.method == "PUT" {
				body = map[string]interface{}{
					"name":     "test",
					"schedule": "* * * * *",
					"webhook":  map[string]interface{}{"url": "http://example.com", "method": "GET"},
				}
			}
			resp := ts.doJSON(t, ep.method, ep.path, body)
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("expected 404, got %d", resp.StatusCode)
			}
			resp.Body.Close()
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	resp, err := http.Get(ts.url("/metrics"))
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestConcurrentJobCreation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookServer.Close()

	errCh := make(chan error, 20)

	for i := 0; i < 20; i++ {
		go func(idx int) {
			jobReq := map[string]interface{}{
				"name":     fmt.Sprintf("concurrent-job-%d", idx),
				"schedule": "*/5 * * * *",
				"webhook": map[string]interface{}{
					"url":    webhookServer.URL,
					"method": "GET",
				},
				"enabled": true,
			}
			resp := ts.doJSON(t, "POST", "/api/v1/jobs", jobReq)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				errCh <- fmt.Errorf("job %d: expected 201, got %d", idx, resp.StatusCode)
			} else {
				errCh <- nil
			}
		}(i)
	}

	for i := 0; i < 20; i++ {
		if err := <-errCh; err != nil {
			t.Error(err)
		}
	}
}
