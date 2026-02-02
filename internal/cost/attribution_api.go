// Package cost provides cost attribution API handlers.
package cost

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// APIHandler handles cost attribution API requests.
type APIHandler struct {
	attribution *Attribution
}

// NewAPIHandler creates a new cost attribution API handler.
func NewAPIHandler(attribution *Attribution) *APIHandler {
	return &APIHandler{attribution: attribution}
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Routes returns the chi router with all cost attribution routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Execution costs
	r.Post("/executions", h.RecordExecution)
	r.Get("/executions/{executionID}", h.GetExecutionCost)
	r.Get("/jobs/{jobID}/costs", h.GetJobCosts)
	r.Get("/jobs/{jobID}/report", h.GetJobCostReport)

	// Cost summaries
	r.Get("/summary", h.GetCostSummary)
	r.Get("/chargeback", h.GenerateChargebackReport)
	r.Get("/export", h.ExportCosts)

	// Cost prediction
	r.Post("/predict", h.PredictCost)

	// Budget management
	r.Post("/budgets", h.CreateBudget)
	r.Get("/budgets", h.ListBudgets)
	r.Get("/budgets/{budgetID}", h.GetBudget)
	r.Put("/budgets/{budgetID}", h.UpdateBudget)
	r.Delete("/budgets/{budgetID}", h.DeleteBudget)

	// Alerts
	r.Get("/alerts", h.GetAlerts)
	r.Post("/alerts/{alertID}/acknowledge", h.AcknowledgeAlert)

	return r
}

// RecordExecution records execution cost.
func (h *APIHandler) RecordExecution(w http.ResponseWriter, r *http.Request) {
	var cost ExecutionCost
	if err := json.NewDecoder(r.Body).Decode(&cost); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if err := h.attribution.RecordExecution(r.Context(), &cost); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: cost})
}

// GetExecutionCost retrieves execution cost.
func (h *APIHandler) GetExecutionCost(w http.ResponseWriter, r *http.Request) {
	executionID := chi.URLParam(r, "executionID")
	cost, err := h.attribution.GetExecutionCost(executionID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: cost})
}

// GetJobCosts retrieves costs for a job.
func (h *APIHandler) GetJobCosts(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	limit := 100 // default limit

	costs, err := h.attribution.GetJobCosts(jobID, limit)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: costs})
}

// GetJobCostReport retrieves a detailed cost report for a job.
func (h *APIHandler) GetJobCostReport(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	from, to := h.parseTimeRange(r)

	report, err := h.attribution.GetJobCostReport(r.Context(), jobID, from, to)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: report})
}

// GetCostSummary retrieves a cost summary.
func (h *APIHandler) GetCostSummary(w http.ResponseWriter, r *http.Request) {
	from, to := h.parseTimeRange(r)

	summary, err := h.attribution.GetCostSummary(r.Context(), from, to)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: summary})
}

// GenerateChargebackReport generates a chargeback report.
func (h *APIHandler) GenerateChargebackReport(w http.ResponseWriter, r *http.Request) {
	from, to := h.parseTimeRange(r)
	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "namespace"
	}

	report, err := h.attribution.GenerateChargebackReport(r.Context(), from, to, groupBy)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: report})
}

// ExportCosts exports cost data.
func (h *APIHandler) ExportCosts(w http.ResponseWriter, r *http.Request) {
	from, to := h.parseTimeRange(r)

	data, err := h.attribution.Export(r.Context(), from, to)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=costs.json")
	w.Write(data)
}

// PredictCost predicts execution cost.
func (h *APIHandler) PredictCost(w http.ResponseWriter, r *http.Request) {
	var req CostPredictionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	prediction, err := h.attribution.PredictCost(r.Context(), &req)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: prediction})
}

// CreateBudget creates a new attribution budget.
func (h *APIHandler) CreateBudget(w http.ResponseWriter, r *http.Request) {
	var budget AttributionBudget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if err := h.attribution.CreateAttrBudget(&budget); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: budget})
}

// ListBudgets lists all attribution budgets.
func (h *APIHandler) ListBudgets(w http.ResponseWriter, r *http.Request) {
	budgets := h.attribution.ListAttrBudgets()
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: budgets})
}

// GetBudget retrieves an attribution budget by ID.
func (h *APIHandler) GetBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")
	budget, err := h.attribution.GetAttrBudget(budgetID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: budget})
}

// UpdateBudget updates an attribution budget.
func (h *APIHandler) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")

	var budget AttributionBudget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	budget.ID = budgetID
	if err := h.attribution.UpdateAttrBudget(&budget); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: budget})
}

// DeleteBudget deletes an attribution budget.
func (h *APIHandler) DeleteBudget(w http.ResponseWriter, r *http.Request) {
	budgetID := chi.URLParam(r, "budgetID")

	if err := h.attribution.DeleteAttrBudget(budgetID); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// GetAlerts retrieves attribution budget alerts.
func (h *APIHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	unacknowledgedOnly := r.URL.Query().Get("unacknowledged") == "true"
	alerts := h.attribution.GetAttrAlerts(unacknowledgedOnly)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: alerts})
}

// AcknowledgeAlert acknowledges an attribution alert.
func (h *APIHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertID")

	if err := h.attribution.AcknowledgeAttrAlert(alertID); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

func (h *APIHandler) parseTimeRange(r *http.Request) (from, to time.Time) {
	// Default to last 30 days
	to = time.Now().UTC()
	from = to.AddDate(0, 0, -30)

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}

	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	return from, to
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
