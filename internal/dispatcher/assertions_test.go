package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func TestEvaluateAssertions_BodyContains(t *testing.T) {
	result := &models.ExecutionResult{Success: true, Response: `{"status":"ok","count":42}`}

	t.Run("passes when body contains string", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			BodyContains: []string{"ok"},
		}, result, time.Millisecond)
		if !ar.Passed {
			t.Errorf("expected pass, got violations: %+v", ar.Violations)
		}
	})

	t.Run("fails when body missing string", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			BodyContains: []string{"error"},
		}, result, time.Millisecond)
		if ar.Passed {
			t.Error("expected failure")
		}
		if len(ar.Violations) != 1 || ar.Violations[0].Type != "body_contains" {
			t.Errorf("expected body_contains violation, got %+v", ar.Violations)
		}
	})
}

func TestEvaluateAssertions_BodyNotContains(t *testing.T) {
	result := &models.ExecutionResult{Success: true, Response: `{"error":"timeout"}`}

	ar := EvaluateAssertions(&models.ResponseAssertions{
		BodyNotContains: []string{"error"},
	}, result, time.Millisecond)
	if ar.Passed {
		t.Error("expected failure for forbidden string")
	}
}

func TestEvaluateAssertions_ResponseTime(t *testing.T) {
	result := &models.ExecutionResult{Success: true, Response: "ok"}

	ar := EvaluateAssertions(&models.ResponseAssertions{
		MaxResponseTime: models.Duration(100 * time.Millisecond),
	}, result, 200*time.Millisecond)

	if ar.Passed {
		t.Error("expected failure for slow response")
	}
	if ar.Violations[0].Type != "response_time" {
		t.Errorf("expected response_time violation, got %s", ar.Violations[0].Type)
	}
}

func TestEvaluateAssertions_JSONPath(t *testing.T) {
	result := &models.ExecutionResult{Success: true, Response: `{"status":"ok","data":{"count":5}}`}

	t.Run("equals match", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			JSONPathMatchers: []models.JSONPathMatcher{
				{Path: "status", Expected: "ok"},
			},
		}, result, time.Millisecond)
		if !ar.Passed {
			t.Errorf("expected pass, got %+v", ar.Violations)
		}
	})

	t.Run("equals mismatch", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			JSONPathMatchers: []models.JSONPathMatcher{
				{Path: "status", Expected: "error"},
			},
		}, result, time.Millisecond)
		if ar.Passed {
			t.Error("expected failure")
		}
	})

	t.Run("nested path", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			JSONPathMatchers: []models.JSONPathMatcher{
				{Path: "data.count", Expected: "5"},
			},
		}, result, time.Millisecond)
		if !ar.Passed {
			t.Errorf("expected pass for nested path, got %+v", ar.Violations)
		}
	})

	t.Run("not_empty operator", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			JSONPathMatchers: []models.JSONPathMatcher{
				{Path: "status", Operator: "not_empty"},
			},
		}, result, time.Millisecond)
		if !ar.Passed {
			t.Errorf("expected pass for not_empty, got %+v", ar.Violations)
		}
	})

	t.Run("contains operator", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			JSONPathMatchers: []models.JSONPathMatcher{
				{Path: "status", Expected: "o", Operator: "contains"},
			},
		}, result, time.Millisecond)
		if !ar.Passed {
			t.Errorf("expected pass for contains, got %+v", ar.Violations)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		ar := EvaluateAssertions(&models.ResponseAssertions{
			JSONPathMatchers: []models.JSONPathMatcher{
				{Path: "nonexistent", Expected: "x"},
			},
		}, result, time.Millisecond)
		if ar.Passed {
			t.Error("expected failure for missing path")
		}
	})
}

func TestEvaluateAssertions_NilReturnsNil(t *testing.T) {
	result := &models.ExecutionResult{Success: true, Response: "ok"}
	ar := EvaluateAssertions(nil, result, time.Millisecond)
	if ar != nil {
		t.Error("expected nil for nil assertions")
	}
}

func TestEvaluateAssertions_InvalidJSON(t *testing.T) {
	result := &models.ExecutionResult{Success: true, Response: "not json"}
	ar := EvaluateAssertions(&models.ResponseAssertions{
		JSONPathMatchers: []models.JSONPathMatcher{
			{Path: "key", Expected: "val"},
		},
	}, result, time.Millisecond)
	if ar.Passed {
		t.Error("expected failure for invalid JSON")
	}
}

func TestDispatcher_Assertions_EnforceMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"error","code":500}`))
	}))
	defer server.Close()

	disp := New(&Config{NodeID: "test", Timeout: 10 * time.Second, MaxConcurrent: 10})
	job := &models.Job{
		ID: "assert-job", Name: "Assert Job", Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL: server.URL, Method: "GET",
			Assertions: &models.ResponseAssertions{
				Mode:         models.AssertionModeEnforce,
				BodyContains: []string{`"status":"ok"`},
			},
		},
		RetryPolicy: &models.RetryPolicy{MaxAttempts: 1, InitialInterval: models.Duration(time.Second), MaxInterval: models.Duration(time.Second), Multiplier: 1},
		Timeout:     models.Duration(10 * time.Second),
	}

	exec, _ := disp.Execute(context.Background(), job, time.Now())
	if exec.Status != models.ExecutionFailed {
		t.Errorf("expected failed (assertion enforce), got %s", exec.Status)
	}
	if exec.AssertionResult == nil || exec.AssertionResult.Passed {
		t.Error("expected assertion result with violations")
	}
}

func TestDispatcher_Assertions_WarnMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"error"}`))
	}))
	defer server.Close()

	disp := New(&Config{NodeID: "test", Timeout: 10 * time.Second, MaxConcurrent: 10})
	job := &models.Job{
		ID: "warn-job", Name: "Warn Job", Schedule: "* * * * *",
		Webhook: &models.WebhookConfig{
			URL: server.URL, Method: "GET",
			Assertions: &models.ResponseAssertions{
				Mode:         models.AssertionModeWarn,
				BodyContains: []string{`"status":"ok"`},
			},
		},
		RetryPolicy: &models.RetryPolicy{MaxAttempts: 1, InitialInterval: models.Duration(time.Second), MaxInterval: models.Duration(time.Second), Multiplier: 1},
		Timeout:     models.Duration(10 * time.Second),
	}

	exec, _ := disp.Execute(context.Background(), job, time.Now())
	if exec.Status != models.ExecutionSuccess {
		t.Errorf("expected success (warn mode), got %s", exec.Status)
	}
	if exec.AssertionResult == nil || exec.AssertionResult.Passed {
		t.Error("expected assertion result with violations even in warn mode")
	}
}
