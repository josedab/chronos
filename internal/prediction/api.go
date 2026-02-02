// Package prediction provides prediction API handlers.
package prediction

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// APIHandler handles prediction API requests.
type APIHandler struct {
	predictor *Predictor
}

// NewAPIHandler creates a new prediction API handler.
func NewAPIHandler(predictor *Predictor) *APIHandler {
	return &APIHandler{predictor: predictor}
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Routes returns the chi router with all prediction routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Record executions
	r.Post("/executions", h.RecordExecution)

	// Predictions
	r.Get("/jobs/{jobID}/predict", h.Predict)
	r.Get("/jobs/{jobID}/stats", h.GetStats)
	r.Get("/jobs/{jobID}/anomalies", h.DetectAnomalies)

	// Alerts
	r.Get("/alerts", h.GetAlerts)
	r.Post("/alerts/{alertID}/acknowledge", h.AcknowledgeAlert)

	// Recommendations
	r.Get("/jobs/{jobID}/recommendations", h.GetRecommendations)

	// Batch predictions
	r.Post("/batch-predict", h.BatchPredict)

	// Dashboard data
	r.Get("/dashboard", h.GetDashboard)

	return r
}

// RecordExecution records an execution for analysis.
func (h *APIHandler) RecordExecution(w http.ResponseWriter, r *http.Request) {
	var record ExecutionRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if err := h.predictor.RecordExecution(r.Context(), record); err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true})
}

// Predict returns failure prediction for a job.
func (h *APIHandler) Predict(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	prediction, err := h.predictor.Predict(r.Context(), jobID)
	if err != nil {
		if err == ErrInsufficientData {
			h.writeJSON(w, http.StatusOK, APIResponse{
				Success: true,
				Data: map[string]interface{}{
					"job_id":  jobID,
					"message": "insufficient data for prediction",
				},
			})
			return
		}
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: prediction})
}

// GetStats returns statistics for a job.
func (h *APIHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	stats, err := h.predictor.GetStats(r.Context(), jobID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: stats})
}

// DetectAnomalies detects anomalies for a job.
func (h *APIHandler) DetectAnomalies(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	anomalies, err := h.predictor.DetectAnomalies(r.Context(), jobID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"job_id":    jobID,
			"anomalies": anomalies,
		},
	})
}

// GetAlerts returns prediction alerts.
func (h *APIHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	acknowledged := r.URL.Query().Get("acknowledged") == "true"
	alerts, err := h.predictor.GetAlerts(r.Context(), acknowledged)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: alerts})
}

// AcknowledgeAlert acknowledges an alert.
func (h *APIHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	alertID := chi.URLParam(r, "alertID")
	if err := h.predictor.AcknowledgeAlert(r.Context(), alertID); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// Recommendation represents an actionable recommendation.
type Recommendation struct {
	Type        string `json:"type"`
	Priority    string `json:"priority"` // critical, high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
}

// GetRecommendations returns recommendations for a job.
func (h *APIHandler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	
	// Get prediction and stats
	prediction, predErr := h.predictor.Predict(r.Context(), jobID)
	stats, statsErr := h.predictor.GetStats(r.Context(), jobID)
	anomalies, _ := h.predictor.DetectAnomalies(r.Context(), jobID)

	recommendations := make([]Recommendation, 0)

	// Based on prediction
	if predErr == nil && prediction != nil {
		if prediction.RiskLevel == RiskCritical || prediction.RiskLevel == RiskHigh {
			recommendations = append(recommendations, Recommendation{
				Type:        "failure_prevention",
				Priority:    "high",
				Title:       "High Failure Risk Detected",
				Description: "This job has a high probability of failure",
				Action:      "Review recent changes and consider increasing timeout/retries",
				Impact:      "Reduce failure rate by 50%+",
			})
		}
	}

	// Based on stats
	if statsErr == nil && stats != nil {
		// High retry rate
		if stats.AvgRetries > 1.5 {
			recommendations = append(recommendations, Recommendation{
				Type:        "optimization",
				Priority:    "medium",
				Title:       "High Retry Rate",
				Description: "Average retry count is above normal threshold",
				Action:      "Investigate webhook endpoint reliability",
				Impact:      "Improve execution efficiency",
			})
		}

		// Degrading trend
		if stats.RecentTrend == "degrading" {
			recommendations = append(recommendations, Recommendation{
				Type:        "monitoring",
				Priority:    "high",
				Title:       "Performance Degradation",
				Description: "Job performance has been declining recently",
				Action:      "Set up additional monitoring and alerts",
				Impact:      "Early detection of issues",
			})
		}

		// Duration outliers
		if stats.P99Duration > stats.AvgDuration*3 {
			recommendations = append(recommendations, Recommendation{
				Type:        "optimization",
				Priority:    "low",
				Title:       "Duration Variability",
				Description: "Some executions take significantly longer than average",
				Action:      "Consider increasing timeout to handle edge cases",
				Impact:      "Prevent timeout failures",
			})
		}
	}

	// Based on anomalies
	for _, anomaly := range anomalies {
		recommendations = append(recommendations, Recommendation{
			Type:        "anomaly",
			Priority:    "medium",
			Title:       "Anomaly Detected",
			Description: anomaly,
			Action:      "Investigate recent executions",
			Impact:      "Identify root cause",
		})
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"job_id":          jobID,
			"recommendations": recommendations,
		},
	})
}

// BatchPredictRequest is a request for batch predictions.
type BatchPredictRequest struct {
	JobIDs []string `json:"job_ids"`
}

// BatchPredict returns predictions for multiple jobs.
func (h *APIHandler) BatchPredict(w http.ResponseWriter, r *http.Request) {
	var req BatchPredictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	predictions := make(map[string]*Prediction)
	for _, jobID := range req.JobIDs {
		prediction, err := h.predictor.Predict(r.Context(), jobID)
		if err == nil {
			predictions[jobID] = prediction
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: predictions})
}

// DashboardData represents the prediction dashboard data.
type DashboardData struct {
	TotalJobs            int                 `json:"total_jobs"`
	JobsAtRisk           int                 `json:"jobs_at_risk"`
	CriticalJobs         int                 `json:"critical_jobs"`
	HighRiskJobs         int                 `json:"high_risk_jobs"`
	MediumRiskJobs       int                 `json:"medium_risk_jobs"`
	HealthyJobs          int                 `json:"healthy_jobs"`
	UnacknowledgedAlerts int                 `json:"unacknowledged_alerts"`
	TopRiskyJobs         []JobRiskSummary    `json:"top_risky_jobs"`
	RecentAlerts         []Alert             `json:"recent_alerts"`
}

// JobRiskSummary is a summary of a job's risk.
type JobRiskSummary struct {
	JobID              string    `json:"job_id"`
	RiskLevel          RiskLevel `json:"risk_level"`
	FailureProbability float64   `json:"failure_probability"`
	SuccessRate        float64   `json:"success_rate"`
	RecentTrend        string    `json:"recent_trend"`
}

// GetDashboard returns dashboard data.
func (h *APIHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	dashboard := &DashboardData{
		TopRiskyJobs: make([]JobRiskSummary, 0),
	}

	// Get all job IDs from predictor's internal state
	h.predictor.mu.RLock()
	jobIDs := make([]string, 0, len(h.predictor.stats))
	for jobID := range h.predictor.stats {
		jobIDs = append(jobIDs, jobID)
	}
	h.predictor.mu.RUnlock()

	dashboard.TotalJobs = len(jobIDs)

	// Collect predictions and categorize
	type jobRisk struct {
		summary JobRiskSummary
		prob    float64
	}
	riskyJobs := make([]jobRisk, 0)

	for _, jobID := range jobIDs {
		prediction, err := h.predictor.Predict(r.Context(), jobID)
		stats, _ := h.predictor.GetStats(r.Context(), jobID)
		
		if err != nil || prediction == nil {
			dashboard.HealthyJobs++
			continue
		}

		switch prediction.RiskLevel {
		case RiskCritical:
			dashboard.CriticalJobs++
			dashboard.JobsAtRisk++
		case RiskHigh:
			dashboard.HighRiskJobs++
			dashboard.JobsAtRisk++
		case RiskMedium:
			dashboard.MediumRiskJobs++
		default:
			dashboard.HealthyJobs++
		}

		if prediction.FailureProbability > 0.3 {
			summary := JobRiskSummary{
				JobID:              jobID,
				RiskLevel:          prediction.RiskLevel,
				FailureProbability: prediction.FailureProbability,
			}
			if stats != nil {
				summary.SuccessRate = stats.SuccessRate
				summary.RecentTrend = stats.RecentTrend
			}
			riskyJobs = append(riskyJobs, jobRisk{summary: summary, prob: prediction.FailureProbability})
		}
	}

	// Sort by failure probability and take top N
	for i := 0; i < len(riskyJobs)-1; i++ {
		for j := i + 1; j < len(riskyJobs); j++ {
			if riskyJobs[j].prob > riskyJobs[i].prob {
				riskyJobs[i], riskyJobs[j] = riskyJobs[j], riskyJobs[i]
			}
		}
	}
	
	for i := 0; i < limit && i < len(riskyJobs); i++ {
		dashboard.TopRiskyJobs = append(dashboard.TopRiskyJobs, riskyJobs[i].summary)
	}

	// Get recent alerts
	alerts, _ := h.predictor.GetAlerts(r.Context(), false)
	dashboard.RecentAlerts = alerts
	
	for _, a := range alerts {
		if !a.Acknowledged {
			dashboard.UnacknowledgedAlerts++
		}
	}

	// Limit alerts to 10 most recent
	if len(dashboard.RecentAlerts) > 10 {
		dashboard.RecentAlerts = dashboard.RecentAlerts[len(dashboard.RecentAlerts)-10:]
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: dashboard})
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
