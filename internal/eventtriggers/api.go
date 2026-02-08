// Package eventtriggers provides HTTP API handlers for event triggers.
package eventtriggers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// APIHandler handles event trigger API requests.
type APIHandler struct {
	manager *Manager
	logger  zerolog.Logger
}

// NewAPIHandler creates a new API handler.
func NewAPIHandler(manager *Manager, logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		manager: manager,
		logger:  logger.With().Str("component", "eventtriggers-api").Logger(),
	}
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError contains error details.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Routes returns the chi router for event trigger endpoints.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Trigger CRUD
	r.Get("/", h.ListTriggers)
	r.Post("/", h.CreateTrigger)
	r.Get("/{id}", h.GetTrigger)
	r.Put("/{id}", h.UpdateTrigger)
	r.Delete("/{id}", h.DeleteTrigger)

	// Trigger actions
	r.Post("/{id}/enable", h.EnableTrigger)
	r.Post("/{id}/disable", h.DisableTrigger)

	// Event processing
	r.Post("/webhook/{id}", h.HandleWebhook)
	r.Post("/events", h.ProcessEvent)

	// Stats
	r.Get("/stats", h.GetStats)

	// Job-specific triggers
	r.Get("/job/{jobId}", h.GetJobTriggers)

	return r
}

// ListTriggers handles GET /triggers.
func (h *APIHandler) ListTriggers(w http.ResponseWriter, r *http.Request) {
	triggers := h.manager.ListTriggers()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    triggers,
	})
}

// CreateTrigger handles POST /triggers.
func (h *APIHandler) CreateTrigger(w http.ResponseWriter, r *http.Request) {
	var trigger EventTrigger
	if err := json.NewDecoder(r.Body).Decode(&trigger); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if err := h.manager.RegisterTrigger(&trigger); err != nil {
		h.writeError(w, http.StatusBadRequest, "REGISTRATION_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("trigger_id", trigger.ID).
		Str("job_id", trigger.JobID).
		Msg("Created event trigger")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    trigger,
	})
}

// GetTrigger handles GET /triggers/{id}.
func (h *APIHandler) GetTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	trigger, ok := h.manager.GetTrigger(id)
	if !ok {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "trigger not found")
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    trigger,
	})
}

// UpdateTrigger handles PUT /triggers/{id}.
func (h *APIHandler) UpdateTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var update EventTrigger
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if err := h.manager.UpdateTrigger(id, &update); err != nil {
		h.writeError(w, http.StatusBadRequest, "UPDATE_ERROR", err.Error())
		return
	}

	trigger, _ := h.manager.GetTrigger(id)
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    trigger,
	})
}

// DeleteTrigger handles DELETE /triggers/{id}.
func (h *APIHandler) DeleteTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.UnregisterTrigger(id); err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.logger.Info().Str("trigger_id", id).Msg("Deleted event trigger")
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
	})
}

// EnableTrigger handles POST /triggers/{id}/enable.
func (h *APIHandler) EnableTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.EnableTrigger(id); err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
	})
}

// DisableTrigger handles POST /triggers/{id}/disable.
func (h *APIHandler) DisableTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.DisableTrigger(id); err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
	})
}

// HandleWebhook handles POST /triggers/webhook/{id}.
func (h *APIHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	trigger, ok := h.manager.GetTrigger(id)
	if !ok {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "trigger not found")
		return
	}

	if trigger.Type != TriggerTypeWebhook {
		h.writeError(w, http.StatusBadRequest, "INVALID_TYPE", "not a webhook trigger")
		return
	}

	// Parse webhook payload
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// Extract headers
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	event := &TriggerEvent{
		Type:    TriggerTypeWebhook,
		Source:  r.RemoteAddr,
		Data:    payload,
		Headers: headers,
	}

	results, err := h.manager.ProcessEvent(r.Context(), event)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "PROCESS_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    results,
	})
}

// ProcessEvent handles POST /triggers/events.
func (h *APIHandler) ProcessEvent(w http.ResponseWriter, r *http.Request) {
	var event TriggerEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	results, err := h.manager.ProcessEvent(r.Context(), &event)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "PROCESS_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    results,
	})
}

// GetStats handles GET /triggers/stats.
func (h *APIHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetJobTriggers handles GET /triggers/job/{jobId}.
func (h *APIHandler) GetJobTriggers(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	triggers := h.manager.GetTriggersForJob(jobID)
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    triggers,
	})
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}
