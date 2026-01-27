package dispatcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func TestDispatcher_ExecuteWebhook_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("X-Chronos-Job-ID") == "" {
			t.Error("missing X-Chronos-Job-ID header")
		}
		if r.Header.Get("X-Chronos-Execution-ID") == "" {
			t.Error("missing X-Chronos-Execution-ID header")
		}
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("expected X-Custom-Header to be 'custom-value', got %q", r.Header.Get("X-Custom-Header"))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "POST",
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
			},
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     1,
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, err := disp.Execute(ctx, job, time.Now())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected status %s, got %s", models.ExecutionSuccess, exec.Status)
	}
	if exec.StatusCode != 200 {
		t.Errorf("expected status code 200, got %d", exec.StatusCode)
	}
	if exec.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", exec.Attempts)
	}
}

func TestDispatcher_ExecuteWebhook_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "GET",
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     2,
			InitialInterval: models.Duration(100 * time.Millisecond),
			MaxInterval:     models.Duration(time.Second),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, _ := disp.Execute(ctx, job, time.Now())

	if exec.Status != models.ExecutionFailed {
		t.Errorf("expected status %s, got %s", models.ExecutionFailed, exec.Status)
	}
	if exec.StatusCode != 500 {
		t.Errorf("expected status code 500, got %d", exec.StatusCode)
	}
	if exec.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", exec.Attempts)
	}
}

func TestDispatcher_ExecuteWebhook_ClientError_NoRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "GET",
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     3,
			InitialInterval: models.Duration(100 * time.Millisecond),
			MaxInterval:     models.Duration(time.Second),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, _ := disp.Execute(ctx, job, time.Now())

	// 4xx errors should not be retried (except 429)
	if exec.Attempts != 1 {
		t.Errorf("expected 1 attempt (no retry for 400), got %d", exec.Attempts)
	}
	if attempts != 1 {
		t.Errorf("expected server to receive 1 request, got %d", attempts)
	}
}

func TestDispatcher_ExecuteWebhook_CustomSuccessCodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:          server.URL,
			Method:       "POST",
			SuccessCodes: []int{202},
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     1,
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, err := disp.Execute(ctx, job, time.Now())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected status %s, got %s", models.ExecutionSuccess, exec.Status)
	}
}

func TestDispatcher_ExecuteWebhook_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if user != "testuser" || pass != "testpass" {
			t.Errorf("unexpected credentials: %s:%s", user, pass)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "GET",
			Auth: &models.AuthConfig{
				Type:     models.AuthTypeBasic,
				Username: "testuser",
				Password: "testpass",
			},
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     1,
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, err := disp.Execute(ctx, job, time.Now())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected status %s, got %s", models.ExecutionSuccess, exec.Status)
	}
}

func TestDispatcher_ExecuteWebhook_BearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-secret-token" {
			t.Errorf("expected Bearer token, got %q", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "GET",
			Auth: &models.AuthConfig{
				Type:  models.AuthTypeBearer,
				Token: "my-secret-token",
			},
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     1,
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, err := disp.Execute(ctx, job, time.Now())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected status %s, got %s", models.ExecutionSuccess, exec.Status)
	}
}

func TestDispatcher_ExecuteWebhook_RequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body["key"] != "value" {
			t.Errorf("expected key=value, got %v", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	job := &models.Job{
		ID:       "test-job",
		Name:     "Test Job",
		Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL:    server.URL,
			Method: "POST",
			Body:   `{"key": "value"}`,
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts:     1,
			InitialInterval: models.Duration(time.Second),
			MaxInterval:     models.Duration(time.Minute),
			Multiplier:      2.0,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	exec, err := disp.Execute(ctx, job, time.Now())
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected status %s, got %s", models.ExecutionSuccess, exec.Status)
	}
}

func TestDispatcher_Concurrency(t *testing.T) {
	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	if disp.RunningCount() != 0 {
		t.Errorf("expected 0 running, got %d", disp.RunningCount())
	}
}

func TestDispatcher_IsJobRunning(t *testing.T) {
	disp := New(&Config{
		NodeID:        "test-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 10,
	})

	if disp.IsJobRunning("nonexistent") {
		t.Error("expected job to not be running")
	}
}
