// Package api provides the REST API handlers.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/chronos/chronos/internal/scheduler"
	"github.com/chronos/chronos/internal/storage"
	"github.com/chronos/chronos/pkg/cron"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// HandlerStore defines the storage operations needed by API handlers.
type HandlerStore interface {
	storage.JobStore
	storage.ExecutionStore
	storage.VersionStore
}

// Handler handles API requests.
type Handler struct {
	store     HandlerStore
	scheduler *scheduler.Scheduler
	logger    zerolog.Logger
}

// NewHandler creates a new API handler.
func NewHandler(store HandlerStore, sched *scheduler.Scheduler, logger zerolog.Logger) *Handler {
	return &Handler{
		store:     store,
		scheduler: sched,
		logger:    logger.With().Str("component", "api").Logger(),
	}
}

// API Response types

// Response is a generic API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo contains error details.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JobRequest is the request body for creating/updating a job.
type JobRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Schedule    string                   `json:"schedule"`
	Timezone    string                   `json:"timezone,omitempty"`
	Webhook     *models.WebhookConfig    `json:"webhook"`
	RetryPolicy *models.RetryPolicy      `json:"retry_policy,omitempty"`
	Timeout     string                   `json:"timeout,omitempty"`
	Concurrency models.ConcurrencyPolicy `json:"concurrency,omitempty"`
	Tags        map[string]string        `json:"tags,omitempty"`
	Enabled     bool                     `json:"enabled"`
}

// JobResponse is the response for job operations.
type JobResponse struct {
	*models.Job
	NextRun *time.Time `json:"next_run,omitempty"`
}

// ListJobsResponse is the response for listing jobs.
type ListJobsResponse struct {
	Jobs   []*JobResponse `json:"jobs"`
	Total  int            `json:"total"`
	Offset int            `json:"offset,omitempty"`
	Limit  int            `json:"limit,omitempty"`
}

// ListExecutionsResponse is the response for listing executions.
type ListExecutionsResponse struct {
	Executions []*models.Execution `json:"executions"`
	Total      int                 `json:"total"`
}

// Health check

// HealthCheck handles GET /health.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
		},
	})
}

// Job handlers

// CreateJob handles POST /api/v1/jobs.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	// Validate request
	if err := h.validateJobRequest(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Parse timeout
	var timeout models.Duration
	if req.Timeout != "" {
		dur, err := time.ParseDuration(req.Timeout)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "INVALID_TIMEOUT", "Invalid timeout format")
			return
		}
		timeout = models.Duration(dur)
	}

	// Create job
	job := &models.Job{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Schedule:    req.Schedule,
		Timezone:    req.Timezone,
		Webhook:     req.Webhook,
		RetryPolicy: req.RetryPolicy,
		Timeout:     timeout,
		Concurrency: req.Concurrency,
		Tags:        req.Tags,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	// Set defaults
	if job.Concurrency == "" {
		job.Concurrency = models.ConcurrencyForbid
	}
	if job.RetryPolicy == nil {
		job.RetryPolicy = models.DefaultRetryPolicy()
	}

	// Store job
	if err := h.store.CreateJob(job); err != nil {
		h.logger.Error().Err(err).Msg("Failed to create job")
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to create job")
		return
	}

	// Add to scheduler
	if err := h.scheduler.AddJob(job); err != nil {
		h.logger.Error().Err(err).Msg("Failed to add job to scheduler")
	}

	h.logger.Info().Str("job_id", job.ID).Str("job_name", job.Name).Msg("Job created")

	h.writeJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    h.jobToResponse(job),
	})
}

// GetJob handles GET /api/v1/jobs/{id}.
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	job, err := h.store.GetJob(jobID)
	if h.HandleStoreError(w, err, "get job") {
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    h.jobToResponse(job),
	})
}

// ListJobs handles GET /api/v1/jobs.
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	offset := 0
	limit := 0 // 0 means no limit

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	var jobs []*models.Job
	var total int
	var err error

	if offset > 0 || limit > 0 {
		jobs, total, err = h.store.ListJobsPaginated(offset, limit)
	} else {
		jobs, err = h.store.ListJobs()
		total = len(jobs)
	}

	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to list jobs")
		return
	}

	responses := make([]*JobResponse, 0, len(jobs))
	for _, job := range jobs {
		responses = append(responses, h.jobToResponse(job))
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: ListJobsResponse{
			Jobs:   responses,
			Total:  total,
			Offset: offset,
			Limit:  limit,
		},
	})
}

// UpdateJob handles PUT /api/v1/jobs/{id}.
func (h *Handler) UpdateJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	// Get existing job
	existing, err := h.store.GetJob(jobID)
	if err != nil {
		if err == models.ErrJobNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get job")
		return
	}

	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	// Validate request
	if err := h.validateJobRequest(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Parse timeout
	var timeout models.Duration
	if req.Timeout != "" {
		dur, err := time.ParseDuration(req.Timeout)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "INVALID_TIMEOUT", "Invalid timeout format")
			return
		}
		timeout = models.Duration(dur)
	}

	// Update job
	existing.Name = req.Name
	existing.Description = req.Description
	existing.Schedule = req.Schedule
	existing.Timezone = req.Timezone
	existing.Webhook = req.Webhook
	existing.RetryPolicy = req.RetryPolicy
	existing.Timeout = timeout
	existing.Concurrency = req.Concurrency
	existing.Tags = req.Tags
	existing.Enabled = req.Enabled
	existing.Version++
	existing.UpdatedAt = time.Now().UTC()

	// Save current config as a version before updating
	jobVersion := &models.JobVersion{
		JobID:     existing.ID,
		Version:   existing.Version,
		Config:    *existing,
		CreatedAt: existing.UpdatedAt,
		CreatedBy: "", // Could be populated from auth context
	}
	if err := h.store.SaveJobVersion(jobVersion); err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to save job version")
	}

	// Store job
	if err := h.store.UpdateJob(existing); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to update job")
		return
	}

	// Update scheduler
	if err := h.scheduler.UpdateJob(existing); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update job in scheduler")
	}

	h.logger.Info().Str("job_id", jobID).Msg("Job updated")

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    h.jobToResponse(existing),
	})
}

// DeleteJob handles DELETE /api/v1/jobs/{id}.
func (h *Handler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	if h.HandleStoreError(w, h.store.DeleteJob(jobID), "delete job") {
		return
	}

	// Remove from scheduler
	h.scheduler.RemoveJob(jobID)

	// Delete executions
	if err := h.store.DeleteExecutions(jobID); err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to delete executions")
	}

	h.logger.Info().Str("job_id", jobID).Msg("Job deleted")

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"id": jobID},
	})
}

// TriggerJob handles POST /api/v1/jobs/{id}/trigger.
func (h *Handler) TriggerJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	execution, err := h.scheduler.TriggerJob(jobID)
	if err != nil {
		apiErr := MapDomainError(err)
		if apiErr.Code == ErrCodeInternalError {
			apiErr = NewExecutionError(err.Error())
		}
		h.WriteAPIError(w, apiErr)
		return
	}

	h.logger.Info().Str("job_id", jobID).Str("execution_id", execution.ID).Msg("Job triggered")

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    execution,
	})
}

// EnableJob handles POST /api/v1/jobs/{id}/enable.
func (h *Handler) EnableJob(w http.ResponseWriter, r *http.Request) {
	h.setJobEnabled(w, r, true)
}

// DisableJob handles POST /api/v1/jobs/{id}/disable.
func (h *Handler) DisableJob(w http.ResponseWriter, r *http.Request) {
	h.setJobEnabled(w, r, false)
}

func (h *Handler) setJobEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	jobID := chi.URLParam(r, "id")

	job, err := h.store.GetJob(jobID)
	if err != nil {
		if err == models.ErrJobNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get job")
		return
	}

	job.Enabled = enabled
	job.UpdatedAt = time.Now().UTC()

	if err := h.store.UpdateJob(job); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to update job")
		return
	}

	if err := h.scheduler.UpdateJob(job); err != nil {
		h.logger.Error().Err(err).Msg("Failed to update job in scheduler")
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	h.logger.Info().Str("job_id", jobID).Str("action", action).Msg("Job status changed")

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    h.jobToResponse(job),
	})
}

// Execution handlers

// ListExecutions handles GET /api/v1/jobs/{id}/executions.
func (h *Handler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	// Check if job exists
	if _, err := h.store.GetJob(jobID); err != nil {
		if err == models.ErrJobNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get job")
		return
	}

	// Get limit from query params
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	executions, err := h.store.ListExecutions(jobID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to list executions")
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: ListExecutionsResponse{
			Executions: executions,
			Total:      len(executions),
		},
	})
}

// GetExecution handles GET /api/v1/jobs/{id}/executions/{execId}.
func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	execID := chi.URLParam(r, "execId")

	execution, err := h.store.GetExecution(jobID, execID)
	if err != nil {
		if err == models.ErrExecutionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Execution not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get execution")
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    execution,
	})
}

// ReplayExecution handles POST /api/v1/jobs/{id}/executions/{execId}/replay.
func (h *Handler) ReplayExecution(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	execID := chi.URLParam(r, "execId")

	// Get the original execution
	originalExec, err := h.store.GetExecution(jobID, execID)
	if err != nil {
		if err == models.ErrExecutionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Execution not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get execution")
		return
	}

	// Get the job to ensure it still exists
	job, err := h.store.GetJob(jobID)
	if err != nil {
		if err == models.ErrJobNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Job not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get job")
		return
	}

	// Trigger the job and get the actual execution result
	execution, err := h.scheduler.TriggerJob(job.ID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "EXECUTION_ERROR", err.Error())
		return
	}

	// Update the execution with replay metadata
	if execution != nil {
		execution.ReplayOf = originalExec.ID
		execution.ReplayCount = originalExec.ReplayCount + 1
		if updateErr := h.store.RecordExecution(execution); updateErr != nil {
			h.logger.Error().Err(updateErr).Msg("Failed to update replay metadata")
		}
	}

	h.logger.Info().
		Str("job_id", jobID).
		Str("original_exec_id", execID).
		Str("replay_exec_id", execution.ID).
		Msg("Execution replay triggered")

	h.writeJSON(w, http.StatusAccepted, Response{
		Success: true,
		Data: map[string]interface{}{
			"replay_execution_id":   execution.ID,
			"original_execution_id": execID,
			"status":                execution.Status,
			"message":               "Execution replay completed",
		},
	})
}

// Version handlers

// ListJobVersions handles GET /api/v1/jobs/{id}/versions.
func (h *Handler) ListJobVersions(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	versions, err := h.store.ListJobVersions(jobID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to list job versions")
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    versions,
	})
}

// GetJobVersion handles GET /api/v1/jobs/{id}/versions/{version}.
func (h *Handler) GetJobVersion(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	versionStr := chi.URLParam(r, "version")

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "Version must be a number")
		return
	}

	jobVersion, err := h.store.GetJobVersion(jobID, version)
	if err != nil {
		if err == models.ErrJobVersionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Job version not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get job version")
		return
	}

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    jobVersion,
	})
}

// RollbackJob handles POST /api/v1/jobs/{id}/rollback/{version}.
func (h *Handler) RollbackJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	versionStr := chi.URLParam(r, "version")

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "Version must be a number")
		return
	}

	// Get the target version
	jobVersion, err := h.store.GetJobVersion(jobID, version)
	if err != nil {
		if err == models.ErrJobVersionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Job version not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to get job version")
		return
	}

	// Create a new version with the old config
	job := jobVersion.Config
	job.UpdatedAt = time.Now()
	job.Version++

	if err := h.store.UpdateJob(&job); err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to rollback job")
		return
	}

	// Update scheduler
	if err := h.scheduler.UpdateJob(&job); err != nil {
		h.logger.Error().Err(err).Str("job_id", jobID).Msg("Failed to update scheduler after rollback")
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("from_version", jobVersion.Version).
		Int("to_version", job.Version).
		Msg("Job rolled back")

	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"job":              job,
			"rolled_back_from": version,
			"new_version":      job.Version,
		},
	})
}

// Cluster handlers

// ClusterStatus handles GET /api/v1/cluster/status.
func (h *Handler) ClusterStatus(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"is_leader":   h.scheduler.IsLeader(),
			"jobs_total":  len(h.scheduler.ListJobs()),
			"running":     h.scheduler.GetMetrics().ExecutionsTotal,
			"next_runs":   h.scheduler.GetNextRuns(10),
		},
	})
}

// Helper methods

func (h *Handler) validateJobRequest(req *JobRequest) error {
	if req.Name == "" {
		return models.ErrJobNameRequired
	}
	if req.Schedule == "" {
		return models.ErrScheduleRequired
	}
	if err := cron.Validate(req.Schedule); err != nil {
		return models.ErrInvalidCronExpr
	}
	if req.Webhook == nil {
		return models.ErrWebhookRequired
	}
	if req.Webhook.URL == "" {
		return models.ErrWebhookURLRequired
	}
	if req.Timezone != "" {
		if _, err := time.LoadLocation(req.Timezone); err != nil {
			return models.ErrInvalidTimezone
		}
	}
	return nil
}

func (h *Handler) jobToResponse(job *models.Job) *JobResponse {
	resp := &JobResponse{Job: job}

	// Calculate next run
	if job.Enabled {
		schedule, err := cron.Parse(job.Schedule)
		if err == nil {
			loc := time.UTC
			if job.Timezone != "" {
				loc, _ = time.LoadLocation(job.Timezone)
			}
			nextRun := schedule.Next(time.Now().In(loc)).UTC()
			resp.NextRun = &nextRun
		}
	}

	return resp
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}
