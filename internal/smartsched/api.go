// Package smartsched provides REST API handlers for schedule optimization.
package smartsched

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/chronos/chronos/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// APIHandler provides HTTP handlers for the schedule advisor.
type APIHandler struct {
	optimizer *Optimizer
}

// NewAPIHandler creates a new advisor API handler.
func NewAPIHandler(optimizer *Optimizer) *APIHandler {
	return &APIHandler{optimizer: optimizer}
}

// Routes returns a chi router with advisor endpoints.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/jobs/{id}/recommendation", h.GetRecommendation)
	r.Get("/jobs/{id}/history", h.GetJobHistory)
	r.Post("/jobs/{id}/record", h.RecordExecution)
	return r
}

func (h *APIHandler) GetRecommendation(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	rec, err := h.optimizer.GetRecommendation(jobID)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": err.Error()},
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    rec,
	})
}

func (h *APIHandler) GetJobHistory(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	history, ok := h.optimizer.GetJobHistory(jobID)
	if !ok {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": "no history for job"},
		})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"job_id":       history.JobID,
			"executions":   len(history.Executions),
			"avg_duration": history.AvgDuration.String(),
			"p95_duration": history.P95Duration.String(),
			"success_rate": history.SuccessRate,
			"peak_hours":   history.PeakHours,
			"best_slots":   history.BestTimeSlots,
		},
	})
}

// RecordRequest represents a request to record an execution.
type RecordRequest struct {
	Duration string `json:"duration"`
	Status   string `json:"status"`
}

func (h *APIHandler) RecordExecution(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	var req RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "invalid_body", "message": err.Error()},
		})
		return
	}

	dur, _ := time.ParseDuration(req.Duration)
	h.optimizer.RecordExecution(jobID, time.Now(), dur, req.Status)

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
