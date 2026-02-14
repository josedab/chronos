package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func TestExecuteShadow_BothSucceed(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer primary.Close()

	shadow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer shadow.Close()

	disp := New(&Config{NodeID: "test", Timeout: 10 * time.Second, MaxConcurrent: 10})
	exec := &models.Execution{ID: "e1", JobID: "j1"}

	result, err := disp.ExecuteShadow(ctx(t),
		&models.WebhookConfig{URL: primary.URL, Method: "GET"},
		&models.WebhookConfig{URL: shadow.URL, Method: "GET"},
		exec)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Match {
		t.Errorf("expected match, got discrepancies: %+v", result.Discrepancies)
	}
}

func TestExecuteShadow_Discrepancy(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer primary.Close()

	shadow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer shadow.Close()

	disp := New(&Config{NodeID: "test", Timeout: 10 * time.Second, MaxConcurrent: 10})
	exec := &models.Execution{ID: "e1", JobID: "j1"}

	result, _ := disp.ExecuteShadow(ctx(t),
		&models.WebhookConfig{URL: primary.URL, Method: "GET"},
		&models.WebhookConfig{URL: shadow.URL, Method: "GET"},
		exec)

	if result.Match {
		t.Error("expected mismatch for different status codes")
	}
	if len(result.Discrepancies) == 0 {
		t.Error("expected discrepancies")
	}

	found := false
	for _, d := range result.Discrepancies {
		if d.Field == "status_code" {
			found = true
			if d.Primary != "200" || d.Shadow != "500" {
				t.Errorf("expected 200 vs 500, got %s vs %s", d.Primary, d.Shadow)
			}
		}
	}
	if !found {
		t.Error("expected status_code discrepancy")
	}
}

func TestExecuteShadow_NilWebhook(t *testing.T) {
	disp := New(&Config{NodeID: "test", Timeout: 10 * time.Second, MaxConcurrent: 10})
	exec := &models.Execution{ID: "e1", JobID: "j1"}

	_, err := disp.ExecuteShadow(ctx(t), nil, &models.WebhookConfig{URL: "http://x"}, exec)
	if err == nil {
		t.Error("expected error for nil primary")
	}

	_, err = disp.ExecuteShadow(ctx(t), &models.WebhookConfig{URL: "http://x"}, nil, exec)
	if err == nil {
		t.Error("expected error for nil shadow")
	}
}

func TestShouldUseCanary(t *testing.T) {
	// 0% should never canary
	for i := 0; i < 100; i++ {
		if ShouldUseCanary(0) {
			t.Fatal("0% should never canary")
		}
	}

	// 100% should always canary
	for i := 0; i < 100; i++ {
		if !ShouldUseCanary(100) {
			t.Fatal("100% should always canary")
		}
	}

	// 50% should be roughly half (statistical)
	canaryCount := 0
	trials := 10000
	for i := 0; i < trials; i++ {
		if ShouldUseCanary(50) {
			canaryCount++
		}
	}
	ratio := float64(canaryCount) / float64(trials)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("50%% canary ratio should be ~0.5, got %.2f", ratio)
	}
}

func TestExecuteShadow_LatencyTracked(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer primary.Close()

	shadow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer shadow.Close()

	disp := New(&Config{NodeID: "test", Timeout: 10 * time.Second, MaxConcurrent: 10})
	exec := &models.Execution{ID: "e1", JobID: "j1"}

	result, _ := disp.ExecuteShadow(ctx(t),
		&models.WebhookConfig{URL: primary.URL, Method: "GET"},
		&models.WebhookConfig{URL: shadow.URL, Method: "GET"},
		exec)

	if result.ShadowLatency < 10*time.Millisecond {
		t.Errorf("expected shadow latency >= 10ms, got %s", result.ShadowLatency)
	}
}

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return c
}
