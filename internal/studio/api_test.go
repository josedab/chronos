package studio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func setupStudioAPI(t *testing.T) *chi.Mux {
	t.Helper()
	studio := NewStudio()
	handler := NewAPIHandler(studio)
	r := chi.NewRouter()
	r.Mount("/studio", handler.Routes())
	return r
}

func TestStudioAPI_TestWebhook(t *testing.T) {
	// Target webhook
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer target.Close()

	router := setupStudioAPI(t)

	body, _ := json.Marshal(TestWebhookRequest{
		Method: "POST",
		URL:    target.URL,
		Headers: map[string]string{
			"X-Test": "studio",
		},
		Body:           `{"test":true}`,
		TimeoutSeconds: 5,
	})

	req := httptest.NewRequest("POST", "/studio/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("expected success=true")
	}

	data := resp["data"].(map[string]interface{})
	if data["success"] != true {
		t.Error("expected webhook test success")
	}
	if int(data["status_code"].(float64)) != 200 {
		t.Errorf("expected status 200, got %v", data["status_code"])
	}
}

func TestStudioAPI_TestWebhook_MissingURL(t *testing.T) {
	router := setupStudioAPI(t)

	body, _ := json.Marshal(TestWebhookRequest{Method: "GET"})
	req := httptest.NewRequest("POST", "/studio/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestStudioAPI_GetHistory(t *testing.T) {
	router := setupStudioAPI(t)

	req := httptest.NewRequest("GET", "/studio/history", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestStudioAPI_InvalidBody(t *testing.T) {
	router := setupStudioAPI(t)

	req := httptest.NewRequest("POST", "/studio/test", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
