package smartsched

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func setupAdvisorAPI(t *testing.T) (*APIHandler, *chi.Mux) {
	t.Helper()
	opt := NewOptimizer(Config{
		WindowSize:        100,
		MinSamples:        5,
		PeakThreshold:     0.8,
		FailureThreshold:  0.3,
	})

	// Seed some execution data
	for i := 0; i < 20; i++ {
		ts := time.Now().Add(-time.Duration(i) * time.Hour)
		opt.RecordExecution("test-job", ts, time.Duration(100+i*10)*time.Millisecond, "success")
	}
	for i := 0; i < 5; i++ {
		ts := time.Now().Add(-time.Duration(i) * time.Hour)
		opt.RecordExecution("test-job", ts, 5*time.Second, "failed")
	}

	handler := NewAPIHandler(opt)
	r := chi.NewRouter()
	r.Mount("/advisor", handler.Routes())
	return handler, r
}

func TestAdvisorAPI_GetRecommendation(t *testing.T) {
	_, router := setupAdvisorAPI(t)

	req := httptest.NewRequest("GET", "/advisor/jobs/test-job/recommendation", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success")
	}

	data := resp["data"].(map[string]interface{})
	if data["job_id"] != "test-job" {
		t.Errorf("expected job_id test-job, got %v", data["job_id"])
	}
	if data["confidence"] == nil {
		t.Error("expected confidence score")
	}
	if data["insights"] == nil {
		t.Error("expected insights")
	}
}

func TestAdvisorAPI_GetRecommendation_NoHistory(t *testing.T) {
	_, router := setupAdvisorAPI(t)

	req := httptest.NewRequest("GET", "/advisor/jobs/nonexistent/recommendation", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Optimizer returns a recommendation with low confidence for unknown jobs
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["success"] != true {
		// If it returns not found, that's also acceptable
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected success or 404, got %d", rec.Code)
		}
	}
}

func TestAdvisorAPI_GetJobHistory(t *testing.T) {
	_, router := setupAdvisorAPI(t)

	req := httptest.NewRequest("GET", "/advisor/jobs/test-job/history", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["executions"].(float64) < 20 {
		t.Errorf("expected at least 20 executions, got %v", data["executions"])
	}
}

func TestAdvisorAPI_RecordExecution(t *testing.T) {
	_, router := setupAdvisorAPI(t)

	body, _ := json.Marshal(RecordRequest{
		Duration: "500ms",
		Status:   "success",
	})

	req := httptest.NewRequest("POST", "/advisor/jobs/new-job/record", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAdvisorAPI_RecordExecution_InvalidBody(t *testing.T) {
	_, router := setupAdvisorAPI(t)

	req := httptest.NewRequest("POST", "/advisor/jobs/test/record", bytes.NewReader([]byte("bad")))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
