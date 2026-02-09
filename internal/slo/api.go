// Package slo provides SLA-based alerting with SLO definitions, error budgets, and burn rate calculations.
package slo

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for SLO management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new SLO HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// Routes returns a chi router with SLO routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.ListSLOs)
	r.Post("/", h.CreateSLO)
	r.Get("/status", h.GetAllStatus)
	r.Get("/alerts", h.GetAlerts)

	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.GetSLO)
		r.Put("/", h.UpdateSLO)
		r.Delete("/", h.DeleteSLO)
		r.Get("/status", h.GetStatus)
		r.Get("/history", h.GetHistory)
	})

	return r
}

// CreateSLO creates a new SLO.
func (h *Handler) CreateSLO(w http.ResponseWriter, r *http.Request) {
	var slo SLO
	if err := json.NewDecoder(r.Body).Decode(&slo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.manager.CreateSLO(r.Context(), &slo); err != nil {
		if err == ErrSLOAlreadyExists {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if err == ErrInvalidSLO || err == ErrInvalidTarget || err == ErrInvalidWindow {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(slo)
}

// GetSLO retrieves an SLO by ID.
func (h *Handler) GetSLO(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	slo, err := h.manager.GetSLO(r.Context(), id)
	if err != nil {
		if err == ErrSLONotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slo)
}

// UpdateSLO updates an existing SLO.
func (h *Handler) UpdateSLO(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var slo SLO
	if err := json.NewDecoder(r.Body).Decode(&slo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slo.ID = id

	if err := h.manager.UpdateSLO(r.Context(), &slo); err != nil {
		if err == ErrSLONotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err == ErrInvalidSLO || err == ErrInvalidTarget || err == ErrInvalidWindow {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(slo)
}

// DeleteSLO deletes an SLO.
func (h *Handler) DeleteSLO(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.manager.DeleteSLO(r.Context(), id); err != nil {
		if err == ErrSLONotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSLOs returns all SLOs.
func (h *Handler) ListSLOs(w http.ResponseWriter, r *http.Request) {
	slos := h.manager.ListSLOs(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"slos":  slos,
		"total": len(slos),
	})
}

// GetStatus retrieves the current status of an SLO.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	status, err := h.manager.GetStatus(r.Context(), id)
	if err != nil {
		if err == ErrSLONotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetAllStatus returns status for all SLOs.
func (h *Handler) GetAllStatus(w http.ResponseWriter, r *http.Request) {
	statuses := h.manager.GetAllStatus(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"statuses": statuses,
		"total":    len(statuses),
	})
}

// GetAlerts returns recent alerts.
func (h *Handler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	alerts := h.manager.GetAlerts(r.Context(), limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// GetHistory returns error budget history for an SLO.
func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	points := 24
	if p := r.URL.Query().Get("points"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			points = parsed
		}
	}

	history, err := h.manager.GetErrorBudgetHistory(r.Context(), id, points)
	if err != nil {
		if err == ErrSLONotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"slo_id":  id,
		"points":  points,
		"history": history,
	})
}

// RecordMetricRequest is the request body for recording a metric.
type RecordMetricRequest struct {
	JobID       string            `json:"job_id"`
	ExecutionID string            `json:"execution_id"`
	Success     bool              `json:"success"`
	DurationMs  int64             `json:"duration_ms"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// RecordMetric records an execution metric.
func (h *Handler) RecordMetric(w http.ResponseWriter, r *http.Request) {
	var req RecordMetricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	metric := ExecutionMetric{
		JobID:       req.JobID,
		ExecutionID: req.ExecutionID,
		Success:     req.Success,
		Duration:    time.Duration(req.DurationMs) * time.Millisecond,
		Timestamp:   time.Now().UTC(),
		Tags:        req.Tags,
	}

	if err := h.manager.RecordMetric(r.Context(), metric); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// MountSLORoutes mounts SLO routes on an existing router.
func MountSLORoutes(r chi.Router, manager *Manager) {
	handler := NewHandler(manager)
	r.Mount("/slos", handler.Routes())
}

// IntegrateSLOWithScheduler creates a function that records metrics from executions.
func IntegrateSLOWithScheduler(manager *Manager) func(ctx context.Context, jobID, executionID string, success bool, duration time.Duration, tags map[string]string) {
	return func(ctx context.Context, jobID, executionID string, success bool, duration time.Duration, tags map[string]string) {
		metric := ExecutionMetric{
			JobID:       jobID,
			ExecutionID: executionID,
			Success:     success,
			Duration:    duration,
			Timestamp:   time.Now().UTC(),
			Tags:        tags,
		}
		manager.RecordMetric(ctx, metric)
	}
}
