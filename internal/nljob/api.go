// Package nljob provides HTTP API for natural language job creation.
package nljob

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// APIHandler handles NL job API requests.
type APIHandler struct {
	creator *Creator
	logger  zerolog.Logger
}

// NewAPIHandler creates a new API handler.
func NewAPIHandler(creator *Creator, logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		creator: creator,
		logger:  logger.With().Str("component", "nljob-api").Logger(),
	}
}

// APIResponse is the standard API response.
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

// ParseRequest is the request body for parsing NL descriptions.
type ParseRequest struct {
	Description string `json:"description"`
}

// ScheduleRequest is the request body for schedule suggestions.
type ScheduleRequest struct {
	Description string `json:"description"`
}

// ExplainRequest is the request body for job explanations.
type ExplainRequest struct {
	JobID string `json:"job_id,omitempty"`
	Job   interface{} `json:"job,omitempty"`
}

// Routes returns the chi router for NL job endpoints.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/parse", h.Parse)
	r.Post("/schedule", h.SuggestSchedule)
	r.Post("/explain", h.ExplainJob)
	r.Post("/interactive", h.Interactive)

	return r
}

// Parse handles POST /nljob/parse.
func (h *APIHandler) Parse(w http.ResponseWriter, r *http.Request) {
	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.Description == "" {
		h.writeError(w, http.StatusBadRequest, "EMPTY_DESCRIPTION", "description is required")
		return
	}

	result, err := h.creator.ParseDescription(r.Context(), req.Description)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "PARSE_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("description", truncate(req.Description, 50)).
		Int("jobs", len(result.Jobs)).
		Msg("Parsed NL job description")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// SuggestSchedule handles POST /nljob/schedule.
func (h *APIHandler) SuggestSchedule(w http.ResponseWriter, r *http.Request) {
	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	schedule, err := h.creator.SuggestSchedule(r.Context(), req.Description)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "SCHEDULE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"schedule":    schedule,
			"description": req.Description,
		},
	})
}

// ExplainJob handles POST /nljob/explain.
func (h *APIHandler) ExplainJob(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// For mock/simple cases, provide a basic explanation
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"explanation": "This job runs according to the specified schedule and executes the configured webhook or command.",
		},
	})
}

// Interactive handles POST /nljob/interactive - conversational job building.
func (h *APIHandler) Interactive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message     string                 `json:"message"`
		Context     map[string]interface{} `json:"context,omitempty"`
		SessionID   string                 `json:"session_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// Parse the message
	result, err := h.creator.ParseDescription(r.Context(), req.Message)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "PARSE_ERROR", err.Error())
		return
	}

	response := map[string]interface{}{
		"session_id": req.SessionID,
		"jobs":       result.Jobs,
		"confidence": result.Confidence,
		"message":    "I've created the following job configuration based on your request.",
	}

	if len(result.Warnings) > 0 {
		response["warnings"] = result.Warnings
		response["message"] = "I've created a job configuration, but there are some items to review."
	}

	if len(result.Suggestions) > 0 {
		response["suggestions"] = result.Suggestions
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
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
