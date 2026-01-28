package studio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewStudio(t *testing.T) {
	studio := NewStudio()

	if studio == nil {
		t.Fatal("expected studio, got nil")
	}
	if studio.history == nil {
		t.Error("expected history to be initialized")
	}
	if studio.client == nil {
		t.Error("expected client to be initialized")
	}
}

func TestStudio_Test(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	studio := NewStudio()

	req := &TestRequest{
		Method: "GET",
		URL:    server.URL,
	}

	resp, err := studio.Test(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestStudio_Test_POST(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	studio := NewStudio()

	req := &TestRequest{
		Method: "POST",
		URL:    server.URL,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"key": "value"}`,
	}

	resp, err := studio.Test(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
	if receivedBody != `{"key": "value"}` {
		t.Errorf("unexpected body: %s", receivedBody)
	}
}

func TestStudio_Test_Headers(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	studio := NewStudio()

	req := &TestRequest{
		Method: "GET",
		URL:    server.URL,
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
			"Authorization":   "Bearer test-token",
		},
	}

	_, err := studio.Test(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Error("expected custom header to be set")
	}
	if receivedHeaders.Get("Authorization") != "Bearer test-token" {
		t.Error("expected authorization header to be set")
	}
}

func TestStudio_Test_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	studio := NewStudio()

	req := &TestRequest{
		Method:  "GET",
		URL:     server.URL,
		Timeout: 100 * time.Millisecond,
	}

	resp, err := studio.Test(context.Background(), req)
	// Timeout should result in error or failed response
	if err == nil && resp.Success {
		t.Error("expected timeout error or failed response")
	}
}

func TestStudio_Test_InvalidURL(t *testing.T) {
	studio := NewStudio()

	req := &TestRequest{
		Method: "GET",
		URL:    "not-a-valid-url",
	}

	_, err := studio.Test(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestStudio_ValidateURL(t *testing.T) {
	studio := NewStudio()

	tests := []struct {
		name     string
		url      string
		valid    bool
		warnings bool
	}{
		{"valid https", "https://example.com/webhook", true, false},
		{"valid http", "http://localhost:8080/api", true, true}, // warning for http
		{"missing scheme", "example.com/webhook", false, false},
		{"empty", "", false, false},
		{"valid with path", "https://api.example.com/v1/jobs", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := studio.ValidateURL(tt.url)
			if result.Valid != tt.valid {
				t.Errorf("expected valid=%v, got valid=%v", tt.valid, result.Valid)
			}
			if tt.warnings && len(result.Warnings) == 0 {
				t.Error("expected warnings")
			}
		})
	}
}

func TestStudio_ParseCurl(t *testing.T) {
	studio := NewStudio()

	curl := `curl -X POST "https://api.example.com/webhook" -H "Content-Type: application/json" -H "Authorization: Bearer token" -d '{"key": "value"}'`

	req, err := studio.ParseCurl(curl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("expected method POST, got %s", req.Method)
	}
	if req.URL != "https://api.example.com/webhook" {
		t.Errorf("expected URL https://api.example.com/webhook, got %s", req.URL)
	}
	if req.Headers["Content-Type"] != "application/json" {
		t.Error("expected Content-Type header")
	}
}

func TestStudio_ParseCurl_Simple(t *testing.T) {
	studio := NewStudio()

	curl := `curl https://example.com/api`

	req, err := studio.ParseCurl(curl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Method != "GET" {
		t.Errorf("expected method GET, got %s", req.Method)
	}
	if req.URL != "https://example.com/api" {
		t.Errorf("expected URL https://example.com/api, got %s", req.URL)
	}
}

func TestStudio_GenerateJob(t *testing.T) {
	studio := NewStudio()

	req := &TestRequest{
		Method: "POST",
		URL:    "https://example.com/webhook",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"action": "process"}`,
	}

	job := studio.GenerateJobConfig(req, "*/5 * * * *")

	if job.Schedule != "*/5 * * * *" {
		t.Errorf("expected schedule */5 * * * *, got %s", job.Schedule)
	}
	if job.Webhook.URL != "https://example.com/webhook" {
		t.Errorf("expected webhook URL, got %s", job.Webhook.URL)
	}
	if job.Webhook.Method != "POST" {
		t.Errorf("expected method POST, got %s", job.Webhook.Method)
	}
}

func TestStudio_GetHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	studio := NewStudio()

	// Make a few test requests
	for i := 0; i < 3; i++ {
		req := &TestRequest{Method: "GET", URL: server.URL}
		studio.Test(context.Background(), req)
	}

	history := studio.GetHistory(10)
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}
}

func TestStudio_GetHistory_WithLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	studio := NewStudio()

	for i := 0; i < 10; i++ {
		req := &TestRequest{Method: "GET", URL: server.URL}
		studio.Test(context.Background(), req)
	}

	history := studio.GetHistory(5)
	if len(history) != 5 {
		t.Errorf("expected 5 history entries (limited), got %d", len(history))
	}
}

func TestStudio_ClearHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	studio := NewStudio()

	req := &TestRequest{Method: "GET", URL: server.URL}
	studio.Test(context.Background(), req)

	studio.ClearHistory()

	history := studio.GetHistory(10)
	if len(history) != 0 {
		t.Error("expected history to be cleared")
	}
}

func TestTestRequest_Fields(t *testing.T) {
	req := TestRequest{
		ID:              "req-1",
		Name:            "Test Request",
		Method:          "POST",
		URL:             "https://example.com",
		Headers:         map[string]string{"X-Key": "value"},
		Body:            "body content",
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	}

	if req.Method != "POST" {
		t.Errorf("expected method POST, got %s", req.Method)
	}
	if !req.FollowRedirects {
		t.Error("expected FollowRedirects to be true")
	}
}

func TestTestResponse_Fields(t *testing.T) {
	resp := TestResponse{
		ID:          "resp-1",
		RequestID:   "req-1",
		StatusCode:  200,
		Status:      "200 OK",
		ContentType: "application/json",
		Duration:    100 * time.Millisecond,
		Success:     true,
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestValidationResult_Fields(t *testing.T) {
	result := ValidationResult{
		Valid:       true,
		URL:         "https://example.com",
		Warnings:    []string{"warning1"},
		Suggestions: []string{"suggestion1"},
	}

	if !result.Valid {
		t.Error("expected valid to be true")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
}
