package quickstart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEchoServer_Start(t *testing.T) {
	es, err := NewEchoServer(0)
	if err != nil {
		t.Fatalf("creating echo server: %v", err)
	}

	if err := es.Start(); err != nil {
		t.Fatalf("starting echo server: %v", err)
	}
	defer es.Stop(context.Background())

	if es.Port() == 0 {
		t.Error("expected non-zero port")
	}

	// Test health endpoint
	resp, err := http.Get(es.URL() + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestEchoServer_Echo(t *testing.T) {
	es, _ := NewEchoServer(0)
	es.Start()
	defer es.Stop(context.Background())

	resp, err := http.Post(es.URL()+"/webhook", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["echo"] != true {
		t.Error("expected echo=true")
	}
	if body["method"] != "POST" {
		t.Errorf("expected POST, got %s", body["method"])
	}

	if len(es.Requests()) != 1 {
		t.Errorf("expected 1 request recorded, got %d", len(es.Requests()))
	}
}

func TestDemoJobs(t *testing.T) {
	jobs := DemoJobs("http://localhost:9999")
	if len(jobs) != 3 {
		t.Fatalf("expected 3 demo jobs, got %d", len(jobs))
	}

	names := make(map[string]bool)
	for _, j := range jobs {
		name, _ := j["name"].(string)
		names[name] = true
		if j["schedule"] == nil || j["schedule"] == "" {
			t.Errorf("job %s missing schedule", name)
		}
	}

	for _, expected := range []string{"hello-world", "daily-report", "cleanup-job"} {
		if !names[expected] {
			t.Errorf("expected demo job %q", expected)
		}
	}
}

func TestCreateDemoJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]interface{}{"id": "demo-job-1"},
		})
	}))
	defer server.Close()

	job := DemoJobs("http://localhost:9999")[0]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := CreateDemoJob(ctx, server.URL, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "demo-job-1" {
		t.Errorf("expected id demo-job-1, got %s", id)
	}
}

func TestCreateDemoJob_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	_, err := CreateDemoJob(context.Background(), server.URL, map[string]interface{}{"name": "test"})
	if err == nil {
		t.Error("expected error for bad request")
	}
}

func TestRunScript(t *testing.T) {
	output := RunScript("http://localhost:9090", "http://localhost:8080", []string{"job-123"})
	if output == "" {
		t.Error("expected non-empty output")
	}
	if len(output) < 100 {
		t.Error("expected substantial output text")
	}
}
