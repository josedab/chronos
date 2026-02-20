package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockChronosServer creates a mock Chronos API for acceptance tests.
func mockChronosServer(t *testing.T) *httptest.Server {
	t.Helper()

	jobs := make(map[string]*Job)
	namespaces := make(map[string]*Namespace)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		// Jobs
		case r.Method == "POST" && r.URL.Path == "/api/v1/jobs":
			var job Job
			json.NewDecoder(r.Body).Decode(&job)
			job.ID = "job-" + job.Name
			jobs[job.ID] = &job
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(job)

		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/jobs/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
			job, ok := jobs[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
				return
			}
			json.NewEncoder(w).Encode(job)

		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/api/v1/jobs/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
			var job Job
			json.NewDecoder(r.Body).Decode(&job)
			job.ID = id
			jobs[id] = &job
			json.NewEncoder(w).Encode(job)

		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/jobs/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
			delete(jobs, id)
			w.WriteHeader(http.StatusNoContent)

		case r.Method == "GET" && r.URL.Path == "/api/v1/jobs":
			jobList := make([]Job, 0, len(jobs))
			for _, j := range jobs {
				jobList = append(jobList, *j)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobList})

		// Namespaces
		case r.Method == "POST" && r.URL.Path == "/api/v1/namespaces":
			var ns Namespace
			json.NewDecoder(r.Body).Decode(&ns)
			ns.ID = "ns-" + ns.Name
			namespaces[ns.ID] = &ns
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(ns)

		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
			ns, ok := namespaces[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(ns)

		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
			delete(namespaces, id)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestClient_CRUD_Jobs(t *testing.T) {
	server := mockChronosServer(t)
	defer server.Close()

	client := NewClient(server.URL, "test-key", 30)
	ctx := context.Background()

	// Create
	created, err := client.CreateJob(ctx, &Job{
		Name:     "acceptance-test",
		Schedule: "*/5 * * * *",
		Enabled:  true,
		Webhook:  &Webhook{URL: "http://example.com/hook", Method: "POST"},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected job ID")
	}
	if created.Name != "acceptance-test" {
		t.Errorf("expected name acceptance-test, got %s", created.Name)
	}

	// Read
	fetched, err := client.GetJob(ctx, created.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if fetched.Name != "acceptance-test" {
		t.Errorf("expected name acceptance-test, got %s", fetched.Name)
	}

	// Update
	updated, err := client.UpdateJob(ctx, created.ID, &Job{
		Name:     "acceptance-test-updated",
		Schedule: "*/10 * * * *",
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != "acceptance-test-updated" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}

	// List
	jobs, err := client.ListJobs(ctx, "")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	// Delete
	if err := client.DeleteJob(ctx, created.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify deletion
	_, err = client.GetJob(ctx, created.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestClient_CRUD_Namespaces(t *testing.T) {
	server := mockChronosServer(t)
	defer server.Close()

	client := NewClient(server.URL, "test-key", 30)
	ctx := context.Background()

	// Create
	created, err := client.CreateNamespace(ctx, &Namespace{
		Name:        "production",
		Description: "Production namespace",
		MaxJobs:     100,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected namespace ID")
	}

	// Read
	fetched, err := client.GetNamespace(ctx, created.ID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if fetched.Name != "production" {
		t.Errorf("expected name production, got %s", fetched.Name)
	}

	// Delete
	if err := client.DeleteNamespace(ctx, created.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}

func TestClient_AuthHeader(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "my-api-key", 30)
	client.GetJob(context.Background(), "test")

	if receivedAuth != "Bearer my-api-key" {
		t.Errorf("expected 'Bearer my-api-key', got %q", receivedAuth)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 30)
	_, err := client.GetJob(context.Background(), "test")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
