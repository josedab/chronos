package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/dispatcher"
	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/scheduler"
	"github.com/chronos/chronos/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

func setupTestAPI(t *testing.T) (*Handler, *storage.BadgerStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "chronos-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := storage.NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	logger := zerolog.New(os.Stdout).Level(zerolog.Disabled)

	disp := dispatcher.New(&dispatcher.Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	sched := scheduler.New(store, disp, logger, &scheduler.Config{
		TickInterval: time.Second,
	})
	sched.SetLeader(true)

	handler := NewHandler(store, sched, logger)

	cleanup := func() {
		sched.Stop()
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return handler, store, cleanup
}

func TestHandler_HealthCheck(t *testing.T) {
	handler, _, cleanup := setupTestAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.HealthCheck(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestHandler_CreateJob(t *testing.T) {
	handler, _, cleanup := setupTestAPI(t)
	defer cleanup()

	jobReq := JobRequest{
		Name:     "test-job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    "https://example.com/webhook",
			Method: "POST",
		},
		Enabled: true,
	}

	body, _ := json.Marshal(jobReq)
	req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateJob(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify data
	data, ok := resp.Data.(*JobResponse)
	if !ok {
		// Try type assertion as map
		dataMap, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatal("unexpected data type")
		}
		if dataMap["name"] != "test-job" {
			t.Errorf("expected name 'test-job', got %v", dataMap["name"])
		}
	} else if data.Name != "test-job" {
		t.Errorf("expected name 'test-job', got %s", data.Name)
	}
}

func TestHandler_CreateJob_Validation(t *testing.T) {
	handler, _, cleanup := setupTestAPI(t)
	defer cleanup()

	tests := []struct {
		name    string
		request JobRequest
		wantErr string
	}{
		{
			name: "missing name",
			request: JobRequest{
				Schedule: "* * * * *",
				Webhook: &models.WebhookConfig{
					URL: "https://example.com",
				},
			},
			wantErr: "VALIDATION_ERROR",
		},
		{
			name: "missing schedule",
			request: JobRequest{
				Name: "test-job",
				Webhook: &models.WebhookConfig{
					URL: "https://example.com",
				},
			},
			wantErr: "VALIDATION_ERROR",
		},
		{
			name: "invalid cron expression",
			request: JobRequest{
				Name:     "test-job",
				Schedule: "invalid",
				Webhook: &models.WebhookConfig{
					URL: "https://example.com",
				},
			},
			wantErr: "VALIDATION_ERROR",
		},
		{
			name: "missing webhook",
			request: JobRequest{
				Name:     "test-job",
				Schedule: "* * * * *",
			},
			wantErr: "VALIDATION_ERROR",
		},
		{
			name: "missing webhook URL",
			request: JobRequest{
				Name:     "test-job",
				Schedule: "* * * * *",
				Webhook: &models.WebhookConfig{
					Method: "POST",
				},
			},
			wantErr: "VALIDATION_ERROR",
		},
		{
			name: "invalid timezone",
			request: JobRequest{
				Name:     "test-job",
				Schedule: "* * * * *",
				Timezone: "Invalid/Timezone",
				Webhook: &models.WebhookConfig{
					URL: "https://example.com",
				},
			},
			wantErr: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/v1/jobs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.CreateJob(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}

			var resp Response
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp.Success {
				t.Error("expected success=false")
			}
			if resp.Error == nil || resp.Error.Code != tt.wantErr {
				t.Errorf("expected error code %q, got %v", tt.wantErr, resp.Error)
			}
		})
	}
}

func TestHandler_ListJobs(t *testing.T) {
	handler, store, cleanup := setupTestAPI(t)
	defer cleanup()

	// Create test jobs
	for i := 1; i <= 3; i++ {
		job := &models.Job{
			ID:        fmt.Sprintf("job-%d", i),
			Name:      fmt.Sprintf("Job %d", i),
			Schedule:  "* * * * *",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		store.CreateJob(job)
	}

	req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	w := httptest.NewRecorder()

	handler.ListJobs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("unexpected data type")
	}
	jobs, ok := dataMap["jobs"].([]interface{})
	if !ok {
		t.Fatal("unexpected jobs type")
	}
	if len(jobs) != 3 {
		t.Errorf("expected 3 jobs, got %d", len(jobs))
	}
}

func TestHandler_GetJob(t *testing.T) {
	handler, store, cleanup := setupTestAPI(t)
	defer cleanup()

	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "0 * * * *",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.CreateJob(job)

	// Use chi router context
	req := httptest.NewRequest("GET", "/api/v1/jobs/test-job-1", nil)
	req = addChiURLParam(req, "id", "test-job-1")
	w := httptest.NewRecorder()

	handler.GetJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetJob_NotFound(t *testing.T) {
	handler, _, cleanup := setupTestAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/jobs/nonexistent", nil)
	req = addChiURLParam(req, "id", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetJob(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandler_DeleteJob(t *testing.T) {
	handler, store, cleanup := setupTestAPI(t)
	defer cleanup()

	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.CreateJob(job)

	req := httptest.NewRequest("DELETE", "/api/v1/jobs/test-job-1", nil)
	req = addChiURLParam(req, "id", "test-job-1")
	w := httptest.NewRecorder()

	handler.DeleteJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify job is deleted
	_, err := store.GetJob("test-job-1")
	if err != models.ErrJobNotFound {
		t.Error("expected job to be deleted")
	}
}

func TestHandler_EnableDisableJob(t *testing.T) {
	handler, store, cleanup := setupTestAPI(t)
	defer cleanup()

	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		Enabled:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.CreateJob(job)

	// Enable job
	req := httptest.NewRequest("POST", "/api/v1/jobs/test-job-1/enable", nil)
	req = addChiURLParam(req, "id", "test-job-1")
	w := httptest.NewRecorder()

	handler.EnableJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	updated, _ := store.GetJob("test-job-1")
	if !updated.Enabled {
		t.Error("expected job to be enabled")
	}

	// Disable job
	req = httptest.NewRequest("POST", "/api/v1/jobs/test-job-1/disable", nil)
	req = addChiURLParam(req, "id", "test-job-1")
	w = httptest.NewRecorder()

	handler.DisableJob(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	updated, _ = store.GetJob("test-job-1")
	if updated.Enabled {
		t.Error("expected job to be disabled")
	}
}

func TestHandler_ListExecutions(t *testing.T) {
	handler, store, cleanup := setupTestAPI(t)
	defer cleanup()

	// Create job
	job := &models.Job{
		ID:        "test-job-1",
		Name:      "Test Job",
		Schedule:  "* * * * *",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	store.CreateJob(job)

	// Create executions
	for i := 1; i <= 5; i++ {
		exec := &models.Execution{
			ID:        fmt.Sprintf("exec-%d", i),
			JobID:     "test-job-1",
			Status:    models.ExecutionSuccess,
			StartedAt: time.Now(),
		}
		store.RecordExecution(exec)
	}

	req := httptest.NewRequest("GET", "/api/v1/jobs/test-job-1/executions", nil)
	req = addChiURLParam(req, "id", "test-job-1")
	w := httptest.NewRecorder()

	handler.ListExecutions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_ClusterStatus(t *testing.T) {
	handler, _, cleanup := setupTestAPI(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/cluster/status", nil)
	w := httptest.NewRecorder()

	handler.ClusterStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success=true")
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("unexpected data type")
	}
	if _, exists := dataMap["is_leader"]; !exists {
		t.Error("expected is_leader in response")
	}
}

// Helper to add chi URL params to request
func addChiURLParam(r *http.Request, key, value string) *http.Request {
	ctx := r.Context()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return r.WithContext(ctx)
}
