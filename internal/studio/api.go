// Package studio provides REST API handlers for webhook testing.
package studio

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chronos/chronos/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// APIHandler provides HTTP handlers for the webhook testing studio.
type APIHandler struct {
	studio *Studio
}

// NewAPIHandler creates a new studio API handler.
func NewAPIHandler(studio *Studio) *APIHandler {
	return &APIHandler{studio: studio}
}

// Routes returns a chi router with studio endpoints.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/test", h.TestWebhook)
	r.Get("/history", h.GetHistory)
	return r
}

// TestWebhookRequest is the API request for testing a webhook.
type TestWebhookRequest struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers,omitempty"`
	Body            string            `json:"body,omitempty"`
	TimeoutSeconds  int               `json:"timeout_seconds,omitempty"`
	FollowRedirects bool              `json:"follow_redirects,omitempty"`
}

func (h *APIHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	var req TestWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "invalid_body", "message": err.Error()},
		})
		return
	}

	if req.URL == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "url_required", "message": "URL is required"},
		})
		return
	}

	timeout := 30 * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	testReq := &TestRequest{
		Method:          req.Method,
		URL:             req.URL,
		Headers:         req.Headers,
		Body:            req.Body,
		Timeout:         timeout,
		FollowRedirects: req.FollowRedirects,
	}

	resp, err := h.studio.Test(r.Context(), testReq)
	if err != nil && resp == nil {
		httputil.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "execution_error", "message": err.Error()},
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    resp,
	})
}

func (h *APIHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	history := h.studio.GetHistory(50)
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"responses": history, "total": len(history)},
	})
}
