package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func mockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/jobs":
			var def JobDefinition
			json.NewDecoder(r.Body).Decode(&def)
			resp := map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":         "job-" + def.Name,
					"name":       def.Name,
					"schedule":   def.Schedule,
					"status":     "active",
					"webhook_url": def.WebhookURL,
					"created_at": time.Now(),
					"updated_at": time.Now(),
				},
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp)

		case r.Method == "GET" && r.URL.Path == "/api/v1/jobs/job-test":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id": "job-test", "name": "test", "schedule": "* * * * *",
				},
			})

		case r.Method == "DELETE" && r.URL.Path == "/api/v1/jobs/job-test":
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.Method == "POST" && r.URL.Path == "/api/v1/jobs/job-test/trigger":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id": "exec-1", "job_id": "job-test", "status": "running",
				},
			})

		case r.Method == "POST" && r.URL.Path == "/api/v1/jobs/job-test/enable":
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.Method == "POST" && r.URL.Path == "/api/v1/jobs/job-test/disable":
			json.NewEncoder(w).Encode(map[string]interface{}{"success": true})

		case r.Method == "GET" && r.URL.Path == "/api/v1/jobs":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": []interface{}{
					map[string]interface{}{"id": "j1", "name": "job-1"},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
		}
	}))
}

func TestClient_CreateJob(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	client := NewClient(server.URL, WithAPIKey("test-key"))
	job, err := client.CreateJob(context.Background(), &JobDefinition{
		Name:       "daily-report",
		Schedule:   "0 9 * * *",
		WebhookURL: "https://api.example.com/report",
		Method:     "POST",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected non-nil job response")
	}
}

func TestClient_GetJob(t *testing.T) {
	server := mockServer(t)
	defer server.Close()

	client := NewClient(server.URL)
	job, err := client.GetJob(context.Background(), "job-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("expected non-nil job")
	}
}

func TestClient_Options(t *testing.T) {
	client := NewClient("http://localhost:8080",
		WithAPIKey("key-123"),
		WithNamespace("production"),
		WithTimeout(5*time.Second),
	)

	if client.apiKey != "key-123" {
		t.Errorf("expected api key key-123, got %s", client.apiKey)
	}
	if client.namespace != "production" {
		t.Errorf("expected namespace production, got %s", client.namespace)
	}
}

func TestEverySchedule(t *testing.T) {
	tests := []struct {
		name   string
		fn     func() string
		expect string
	}{
		{"Minute(5)", func() string { return EverySchedule.Minute(5) }, "0 */5 * * * *"},
		{"Hour(1)", func() string { return EverySchedule.Hour(1) }, "0 0 */1 * * *"},
		{"Day(9,0)", func() string { return EverySchedule.Day(9, 0) }, "0 0 9 * * *"},
		{"Midnight", func() string { return EverySchedule.Midnight() }, "0 0 0 * * *"},
		{"Noon", func() string { return EverySchedule.Noon() }, "0 0 12 * * *"},
		{"Monday(10,30)", func() string { return EverySchedule.Monday(10, 30) }, "0 30 10 * * 1"},
		{"Weekday(9,0)", func() string { return EverySchedule.Weekday(9, 0) }, "0 0 9 * * 1-5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn()
			if got != tc.expect {
				t.Errorf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}

func TestCronBuilder(t *testing.T) {
	cron := NewCronBuilder().
		AtSecond(0).
		EveryNMinutes(5).
		AtHour(9).
		Build()

	if cron != "0 */5 9 * * *" {
		t.Errorf("expected '0 */5 9 * * *', got %q", cron)
	}
}

func TestJobBuilder(t *testing.T) {
	sched := New()

	job, err := sched.NewJob("test-job").
		WithSchedule("0 */5 * * * *").
		Handler(func(ctx context.Context, j *Job) error {
			return nil
		}).
		Timeout(30 * time.Second).
		Tag("team", "platform").
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Name != "test-job" {
		t.Errorf("expected name test-job, got %s", job.Name)
	}
	if job.Tags["team"] != "platform" {
		t.Error("expected tag team=platform")
	}
}

func TestJobBuilder_MissingSchedule(t *testing.T) {
	sched := New()
	_, err := sched.NewJob("test").
		Handler(func(ctx context.Context, j *Job) error { return nil }).
		Build()
	if err == nil {
		t.Error("expected error for missing schedule")
	}
}

func TestJobBuilder_MissingHandler(t *testing.T) {
	sched := New()
	_, err := sched.NewJob("test").
		WithSchedule("* * * * *").
		Build()
	if err == nil {
		t.Error("expected error for missing handler")
	}
}

func TestJobBuilder_Every(t *testing.T) {
	sched := New()
	job, err := sched.NewJob("test").
		Every("minute").
		Handler(func(ctx context.Context, j *Job) error { return nil }).
		Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.Schedule != "0 * * * * *" {
		t.Errorf("expected minute schedule, got %s", job.Schedule)
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{StatusCode: 404, Code: "not_found", Message: "job not found"}
	msg := err.Error()
	if msg != "not_found: job not found (status 404)" {
		t.Errorf("unexpected error message: %s", msg)
	}
}
