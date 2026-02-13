package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware(t *testing.T) {
	handler := Middleware("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	// X-Trace-ID may or may not be set depending on whether tracer is a noop
	// but the middleware should not panic
}

func TestMiddleware_ErrorStatus(t *testing.T) {
	handler := Middleware("test-service")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/bad", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestStatusWriter_DefaultsTo200(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, statusCode: http.StatusOK}
	sw.Write([]byte("hello"))

	if sw.statusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", sw.statusCode)
	}
}

func TestStatusWriter_CapturesCode(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, statusCode: http.StatusOK}
	sw.WriteHeader(http.StatusNotFound)

	if sw.statusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", sw.statusCode)
	}
}

func TestPropagationCarrier(t *testing.T) {
	h := http.Header{}
	c := propagationCarrier(h)
	c.Set("traceparent", "00-abc-def-01")
	if c.Get("traceparent") != "00-abc-def-01" {
		t.Errorf("expected traceparent value, got %q", c.Get("traceparent"))
	}
	keys := c.Keys()
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
}

func TestStartRaftLeaderElectionSpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartRaftLeaderElectionSpan(ctx, "node-1", true)
	defer span.End()
	if span == nil {
		t.Fatal("expected non-nil span")
	}
}

func TestStartRaftApplySpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartRaftApplySpan(ctx, "create_job")
	defer span.End()
	if span == nil {
		t.Fatal("expected non-nil span")
	}
}

func TestStartRaftSnapshotSpan(t *testing.T) {
	ctx := context.Background()
	ctx, span := StartRaftSnapshotSpan(ctx, "node-1")
	defer span.End()
	if span == nil {
		t.Fatal("expected non-nil span")
	}
}
