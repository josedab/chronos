package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func setupAlertAPI(t *testing.T) (*AlertHandler, *chi.Mux) {
	t.Helper()
	hub := NewHub()
	hub.AddChannel(context.Background(), &Channel{
		ID: "ch-test", Name: "Test", Type: ChannelWebhook, Enabled: true,
		Config: ChannelConfig{WebhookURL: "http://localhost:9999"},
	})
	engine := NewAlertEngine(hub)
	handler := NewAlertHandler(engine)

	r := chi.NewRouter()
	r.Mount("/alerts", handler.Routes())
	return handler, r
}

func TestAlertAPI_CreateAndListRules(t *testing.T) {
	_, router := setupAlertAPI(t)

	// Create rule
	body, _ := json.Marshal(map[string]interface{}{
		"name":        "test-rule",
		"enabled":     true,
		"events":      []string{"job_failed"},
		"channel_ids": []string{"ch-test"},
		"severity":    "error",
	})

	req := httptest.NewRequest("POST", "/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List rules
	req = httptest.NewRequest("GET", "/alerts/rules", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["total"].(float64) != 1 {
		t.Errorf("expected 1 rule, got %v", data["total"])
	}
}

func TestAlertAPI_GetRule(t *testing.T) {
	handler, router := setupAlertAPI(t)

	handler.engine.AddRule(&AlertRule{
		ID: "r1", Name: "test", Enabled: true,
		Events: []EventType{EventJobFailed}, ChannelIDs: []string{"ch-test"},
	})

	req := httptest.NewRequest("GET", "/alerts/rules/r1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAlertAPI_GetRule_NotFound(t *testing.T) {
	_, router := setupAlertAPI(t)

	req := httptest.NewRequest("GET", "/alerts/rules/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAlertAPI_DeleteRule(t *testing.T) {
	handler, router := setupAlertAPI(t)

	handler.engine.AddRule(&AlertRule{
		ID: "del-1", Name: "delete-me", Enabled: true,
		Events: []EventType{EventJobFailed}, ChannelIDs: []string{"ch-test"},
	})

	req := httptest.NewRequest("DELETE", "/alerts/rules/del-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAlertAPI_TestAlert(t *testing.T) {
	handler, router := setupAlertAPI(t)

	handler.engine.AddRule(&AlertRule{
		Name: "all-failures", Enabled: true,
		Events: []EventType{EventJobFailed}, ChannelIDs: []string{"ch-test"},
		Severity: SeverityError,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"event":    "job_failed",
		"job_id":   "test-job",
		"job_name": "Test Job",
		"error":    "connection refused",
	})

	req := httptest.NewRequest("POST", "/alerts/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["matched"].(float64) < 1 {
		t.Error("expected at least 1 matched delivery")
	}
}

func TestAlertAPI_DeliveryHistory(t *testing.T) {
	_, router := setupAlertAPI(t)

	req := httptest.NewRequest("GET", "/alerts/history", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAlertAPI_CreateRule_Invalid(t *testing.T) {
	_, router := setupAlertAPI(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "", // missing name
	})

	req := httptest.NewRequest("POST", "/alerts/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
