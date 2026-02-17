package cloud

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFreeTierLimiter_CanCreateJob(t *testing.T) {
	l := NewFreeTierLimiter(FreeTierConfig{MaxJobs: 2, MaxExecutionsPerDay: 100, MaxConcurrent: 5})

	if !l.CanCreateJob() {
		t.Error("should allow first job")
	}
	l.RecordJobCreated()
	l.RecordJobCreated()

	if l.CanCreateJob() {
		t.Error("should deny 3rd job at limit 2")
	}
}

func TestFreeTierLimiter_CanExecute(t *testing.T) {
	l := NewFreeTierLimiter(FreeTierConfig{MaxJobs: 10, MaxExecutionsPerDay: 3, MaxConcurrent: 10})

	for i := 0; i < 3; i++ {
		if !l.CanExecute() {
			t.Errorf("should allow execution %d", i+1)
		}
		l.RecordExecutionStart()
		l.RecordExecutionEnd()
	}

	if l.CanExecute() {
		t.Error("should deny after daily limit")
	}
}

func TestFreeTierLimiter_ConcurrentLimit(t *testing.T) {
	l := NewFreeTierLimiter(FreeTierConfig{MaxJobs: 10, MaxExecutionsPerDay: 100, MaxConcurrent: 2})

	l.RecordExecutionStart()
	l.RecordExecutionStart()

	if l.CanExecute() {
		t.Error("should deny at concurrent limit")
	}

	l.RecordExecutionEnd()
	if !l.CanExecute() {
		t.Error("should allow after concurrent release")
	}
}

func TestFreeTierLimiter_Middleware(t *testing.T) {
	l := NewFreeTierLimiter(FreeTierConfig{MaxJobs: 1, MaxExecutionsPerDay: 100, MaxConcurrent: 10})
	l.RecordJobCreated() // Already at limit

	handler := l.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	// POST /api/v1/jobs should be blocked
	req := httptest.NewRequest("POST", "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}

	// GET should pass through
	req = httptest.NewRequest("GET", "/api/v1/jobs", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 for GET passthrough, got %d", rec.Code)
	}
}

func TestFreeTierLimiter_Stats(t *testing.T) {
	l := NewFreeTierLimiter(DefaultFreeTierConfig())
	l.RecordJobCreated()
	l.RecordExecutionStart()

	stats := l.Stats()
	if stats["jobs"].(int32) != 1 {
		t.Errorf("expected 1 job, got %v", stats["jobs"])
	}
	if stats["concurrent"].(int32) != 1 {
		t.Errorf("expected 1 concurrent, got %v", stats["concurrent"])
	}
}

func TestDefaultFreeTierConfig(t *testing.T) {
	cfg := DefaultFreeTierConfig()
	if cfg.MaxJobs != 5 {
		t.Errorf("expected max_jobs 5, got %d", cfg.MaxJobs)
	}
	if cfg.MaxExecutionsPerDay != 100 {
		t.Errorf("expected max_executions 100, got %d", cfg.MaxExecutionsPerDay)
	}
}
