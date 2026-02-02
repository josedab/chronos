// Package versioning provides API handlers for version control operations.
package versioning

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/chronos/chronos/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// APIHandler handles versioning API requests.
type APIHandler struct {
	manager  *Manager
	jobStore storage.JobStore
	logger   zerolog.Logger
}

// NewAPIHandler creates a new versioning API handler.
func NewAPIHandler(manager *Manager, jobStore storage.JobStore, logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		manager:  manager,
		jobStore: jobStore,
		logger:   logger.With().Str("component", "versioning-api").Logger(),
	}
}

// Response types

// APIResponse is a generic API response.
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

// VersionListResponse is the response for listing versions.
type VersionListResponse struct {
	Versions []*Version `json:"versions"`
	Total    int        `json:"total"`
}

// CreateVersionRequest is the request for creating a version.
type CreateVersionRequest struct {
	Description string `json:"description"`
}

// StartCanaryRequest is the request for starting a canary.
type StartCanaryRequest struct {
	CanaryVersion   int              `json:"canary_version"`
	InitialPercent  int              `json:"initial_percent"`
	RolloutStrategy *RolloutStrategy `json:"rollout_strategy,omitempty"`
}

// UpdateCanaryRequest is the request for updating canary traffic.
type UpdateCanaryRequest struct {
	Percent int `json:"percent"`
}

// Routes registers the versioning API routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Version routes
	r.Get("/jobs/{jobID}/versions", h.ListVersions)
	r.Post("/jobs/{jobID}/versions", h.CreateVersion)
	r.Get("/jobs/{jobID}/versions/{version}", h.GetVersion)
	r.Get("/jobs/{jobID}/versions/{version}/config", h.GetVersionConfig)
	r.Post("/jobs/{jobID}/versions/{version}/rollback", h.Rollback)
	r.Get("/jobs/{jobID}/versions/compare", h.CompareVersions)
	r.Get("/jobs/{jobID}/versions/diff", h.GetVersionDiff)

	// Canary routes
	r.Get("/jobs/{jobID}/canary", h.GetCanary)
	r.Post("/jobs/{jobID}/canary", h.StartCanary)
	r.Put("/jobs/{jobID}/canary", h.UpdateCanary)
	r.Post("/jobs/{jobID}/canary/promote", h.PromoteCanary)
	r.Post("/jobs/{jobID}/canary/rollback", h.RollbackCanary)
	r.Delete("/jobs/{jobID}/canary", h.DeleteCanary)

	// Active canaries list
	r.Get("/canaries", h.ListActiveCanaries)

	return r
}

// ListVersions handles GET /jobs/{jobID}/versions.
func (h *APIHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	versions, err := h.manager.ListVersions(r.Context(), jobID, limit)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: VersionListResponse{
			Versions: versions,
			Total:    len(versions),
		},
	})
}

// CreateVersion handles POST /jobs/{jobID}/versions.
func (h *APIHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	// Get current job
	job, err := h.jobStore.GetJob(jobID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "job not found")
		return
	}

	var req CreateVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Description = "Manual version creation"
	}

	// Get author from context or header
	author := r.Header.Get("X-User-ID")
	if author == "" {
		author = "system"
	}

	version, err := h.manager.CreateVersion(r.Context(), job, author, req.Description)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "VERSION_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("version", version.Number).
		Msg("Version created")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    version,
	})
}

// GetVersion handles GET /jobs/{jobID}/versions/{version}.
func (h *APIHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	versionStr := chi.URLParam(r, "version")

	versionNum, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "version must be a number")
		return
	}

	version, err := h.manager.GetVersion(r.Context(), jobID, versionNum)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    version,
	})
}

// GetVersionConfig handles GET /jobs/{jobID}/versions/{version}/config.
func (h *APIHandler) GetVersionConfig(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	versionStr := chi.URLParam(r, "version")

	versionNum, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "version must be a number")
		return
	}

	job, err := h.manager.GetJobAtVersion(r.Context(), jobID, versionNum)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    job,
	})
}

// Rollback handles POST /jobs/{jobID}/versions/{version}/rollback.
func (h *APIHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	versionStr := chi.URLParam(r, "version")

	targetVersion, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "version must be a number")
		return
	}

	author := r.Header.Get("X-User-ID")
	if author == "" {
		author = "system"
	}

	version, err := h.manager.Rollback(r.Context(), jobID, targetVersion, author)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "ROLLBACK_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("target_version", targetVersion).
		Int("new_version", version.Number).
		Msg("Rollback completed")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    version,
	})
}

// CompareVersions handles GET /jobs/{jobID}/versions/compare?v1=X&v2=Y.
func (h *APIHandler) CompareVersions(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	v1Str := r.URL.Query().Get("v1")
	v2Str := r.URL.Query().Get("v2")

	v1, err := strconv.Atoi(v1Str)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "v1 must be a number")
		return
	}
	v2, err := strconv.Atoi(v2Str)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "v2 must be a number")
		return
	}

	comparison, err := h.manager.CompareVersions(r.Context(), jobID, v1, v2)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    comparison,
	})
}

// GetVersionDiff handles GET /jobs/{jobID}/versions/diff?from=X&to=Y.
func (h *APIHandler) GetVersionDiff(w http.ResponseWriter, r *http.Request) {
	// Same as CompareVersions but with different parameter names
	jobID := chi.URLParam(r, "jobID")

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	// Default "to" to latest
	from, err := strconv.Atoi(fromStr)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "from must be a number")
		return
	}

	var to int
	if toStr == "" || toStr == "latest" {
		latest, err := h.manager.GetLatestVersion(r.Context(), jobID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
			return
		}
		to = latest.Number
	} else {
		to, err = strconv.Atoi(toStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "INVALID_VERSION", "to must be a number")
			return
		}
	}

	comparison, err := h.manager.CompareVersions(r.Context(), jobID, from, to)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "VERSION_NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    comparison.Diff,
	})
}

// GetCanary handles GET /jobs/{jobID}/canary.
func (h *APIHandler) GetCanary(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	canary, err := h.manager.GetCanary(r.Context(), jobID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NO_CANARY", "no active canary deployment")
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    canary,
	})
}

// StartCanary handles POST /jobs/{jobID}/canary.
func (h *APIHandler) StartCanary(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	var req StartCanaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.InitialPercent == 0 {
		req.InitialPercent = 10 // Default to 10%
	}

	canary, err := h.manager.StartCanary(r.Context(), jobID, req.CanaryVersion, req.InitialPercent, req.RolloutStrategy)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "CANARY_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("canary_version", req.CanaryVersion).
		Int("initial_percent", req.InitialPercent).
		Msg("Canary deployment started")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    canary,
	})
}

// UpdateCanary handles PUT /jobs/{jobID}/canary.
func (h *APIHandler) UpdateCanary(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	var req UpdateCanaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	canary, err := h.manager.UpdateCanaryTraffic(r.Context(), jobID, req.Percent)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "CANARY_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("new_percent", req.Percent).
		Msg("Canary traffic updated")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    canary,
	})
}

// PromoteCanary handles POST /jobs/{jobID}/canary/promote.
func (h *APIHandler) PromoteCanary(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	author := r.Header.Get("X-User-ID")
	if author == "" {
		author = "system"
	}

	version, err := h.manager.PromoteCanary(r.Context(), jobID, author)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "PROMOTE_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("job_id", jobID).
		Int("promoted_version", version.Number).
		Msg("Canary promoted")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    version,
	})
}

// RollbackCanary handles POST /jobs/{jobID}/canary/rollback.
func (h *APIHandler) RollbackCanary(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	if err := h.manager.RollbackCanary(r.Context(), jobID); err != nil {
		h.writeError(w, http.StatusBadRequest, "ROLLBACK_ERROR", err.Error())
		return
	}

	h.logger.Info().Str("job_id", jobID).Msg("Canary rolled back")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"status": "rolled_back"},
	})
}

// DeleteCanary handles DELETE /jobs/{jobID}/canary.
func (h *APIHandler) DeleteCanary(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")

	if err := h.manager.store.DeleteCanary(r.Context(), jobID); err != nil {
		h.writeError(w, http.StatusBadRequest, "DELETE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"deleted": jobID},
	})
}

// ListActiveCanaries handles GET /canaries.
func (h *APIHandler) ListActiveCanaries(w http.ResponseWriter, r *http.Request) {
	canaries, err := h.manager.store.ListActiveCanaries(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"canaries": canaries,
			"total":    len(canaries),
		},
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

// Helper interface check
var _ storage.JobStore = (storage.JobStore)(nil)
