package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func BenchmarkDispatcher_Execute(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "bench-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 100,
	})

	job := &models.Job{
		ID: "bench-job", Name: "Benchmark", Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{URL: server.URL, Method: "POST", Body: `{"test":true}`},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts: 1, InitialInterval: models.Duration(time.Second),
			MaxInterval: models.Duration(time.Second), Multiplier: 1,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		disp.Execute(ctx, job, time.Now())
	}
}

func BenchmarkDispatcher_Execute_WithSigning(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "bench-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 100,
		Signing:       &SigningConfig{Secret: "bench-secret", IncludeTimestamp: true},
	})

	job := &models.Job{
		ID: "bench-job", Name: "Benchmark", Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{URL: server.URL, Method: "POST", Body: `{"test":true}`},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts: 1, InitialInterval: models.Duration(time.Second),
			MaxInterval: models.Duration(time.Second), Multiplier: 1,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		disp.Execute(ctx, job, time.Now())
	}
}

func BenchmarkDispatcher_Execute_WithAssertions(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","data":{"count":42}}`))
	}))
	defer server.Close()

	disp := New(&Config{
		NodeID:        "bench-node",
		Timeout:       30 * time.Second,
		MaxConcurrent: 100,
	})

	job := &models.Job{
		ID: "bench-job", Name: "Benchmark", Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL: server.URL, Method: "POST",
			Assertions: &models.ResponseAssertions{
				Mode:         models.AssertionModeEnforce,
				BodyContains: []string{"ok"},
				JSONPathMatchers: []models.JSONPathMatcher{
					{Path: "status", Expected: "ok"},
					{Path: "data.count", Expected: "42"},
				},
			},
		},
		RetryPolicy: &models.RetryPolicy{
			MaxAttempts: 1, InitialInterval: models.Duration(time.Second),
			MaxInterval: models.Duration(time.Second), Multiplier: 1,
		},
		Timeout: models.Duration(30 * time.Second),
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		disp.Execute(ctx, job, time.Now())
	}
}

func BenchmarkEvaluateAssertions(b *testing.B) {
	result := &models.ExecutionResult{
		Success:    true,
		StatusCode: 200,
		Response:   `{"status":"ok","data":{"items":[1,2,3],"count":3}}`,
	}

	assertions := &models.ResponseAssertions{
		BodyContains: []string{"ok", "items"},
		JSONPathMatchers: []models.JSONPathMatcher{
			{Path: "status", Expected: "ok"},
			{Path: "data.count", Expected: "3"},
		},
		MaxResponseTime: models.Duration(5 * time.Second),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		EvaluateAssertions(assertions, result, time.Millisecond)
	}
}

func BenchmarkComputeHMAC(b *testing.B) {
	secret := "benchmark-secret-key-256-bits!!"
	payload := `{"job_id":"test","execution_id":"exec-123","data":{"key":"value"}}`

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ComputeHMAC(secret, payload, time.Now().Unix())
	}
}

func BenchmarkEventBus_Publish(b *testing.B) {
	bus := NewEventBus(10000)
	bus.Subscribe("", func(e ExecutionEvent) {})
	bus.Subscribe("job-1", func(e ExecutionEvent) {})

	event := ExecutionEvent{
		Type:        EventExecutionCompleted,
		ExecutionID: "exec-1",
		JobID:       "job-1",
		Timestamp:   time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bus.Publish(event)
	}
}
