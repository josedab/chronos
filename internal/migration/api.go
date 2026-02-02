// Package migration provides API handlers for migration operations.
package migration

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// APIHandler handles migration API requests.
type APIHandler struct {
	migrator *Migrator
	logger   zerolog.Logger
}

// NewAPIHandler creates a new migration API handler.
func NewAPIHandler(logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		migrator: NewMigrator(),
		logger:   logger.With().Str("component", "migration-api").Logger(),
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

// MigrateRequest is the request body for migration.
type MigrateRequest struct {
	SourceType SourceType `json:"source_type"`
	Data       string     `json:"data"` // Base64 or raw JSON
}

// Routes registers the migration API routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/sources", h.ListSources)
	r.Post("/migrate", h.Migrate)
	r.Post("/validate", h.Validate)
	r.Post("/preview", h.Preview)

	// Source-specific endpoints
	r.Post("/kubernetes", h.MigrateKubernetes)
	r.Post("/airflow", h.MigrateAirflow)
	r.Post("/eventbridge", h.MigrateEventBridge)
	r.Post("/temporal", h.MigrateTemporal)
	r.Post("/github-actions", h.MigrateGitHubActions)

	return r
}

// SourceInfo contains information about a supported source.
type SourceInfo struct {
	Type        SourceType `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	DocsURL     string     `json:"docs_url"`
	InputFormat string     `json:"input_format"`
}

// ListSources handles GET /migration/sources.
func (h *APIHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources := []SourceInfo{
		{
			Type:        SourceKubernetesCronJob,
			Name:        "Kubernetes CronJob",
			Description: "Import Kubernetes CronJob manifests (YAML/JSON)",
			DocsURL:     "https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/",
			InputFormat: "kubernetes_manifest",
		},
		{
			Type:        SourceAirflowDAG,
			Name:        "Apache Airflow DAG",
			Description: "Import Airflow DAG definitions (exported JSON)",
			DocsURL:     "https://airflow.apache.org/docs/",
			InputFormat: "airflow_dag_json",
		},
		{
			Type:        SourceAWSEventBridge,
			Name:        "AWS EventBridge",
			Description: "Import AWS EventBridge scheduled rules",
			DocsURL:     "https://docs.aws.amazon.com/eventbridge/",
			InputFormat: "eventbridge_rule",
		},
		{
			Type:        SourceTemporalSchedule,
			Name:        "Temporal Schedule",
			Description: "Import Temporal workflow schedules",
			DocsURL:     "https://docs.temporal.io/",
			InputFormat: "temporal_schedule",
		},
		{
			Type:        SourceGitHubActions,
			Name:        "GitHub Actions",
			Description: "Import GitHub Actions workflow schedules",
			DocsURL:     "https://docs.github.com/en/actions",
			InputFormat: "github_workflow",
		},
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    sources,
	})
}

// Migrate handles POST /migration/migrate.
func (h *APIHandler) Migrate(w http.ResponseWriter, r *http.Request) {
	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	result, err := h.migrator.Migrate(r.Context(), req.SourceType, []byte(req.Data))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "MIGRATION_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("source_type", string(req.SourceType)).
		Int("items_migrated", result.ItemsMigrated).
		Int("items_failed", result.ItemsFailed).
		Msg("Migration completed")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: result.Success,
		Data:    result,
	})
}

// Validate handles POST /migration/validate.
func (h *APIHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	converter, ok := h.migrator.converters[req.SourceType]
	if !ok {
		h.writeError(w, http.StatusBadRequest, "INVALID_SOURCE", "unsupported source type")
		return
	}

	if err := converter.Validate([]byte(req.Data)); err != nil {
		h.writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"valid":   false,
				"error":   err.Error(),
			},
		})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"valid": true,
		},
	})
}

// Preview handles POST /migration/preview.
func (h *APIHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	result, err := h.migrator.Migrate(r.Context(), req.SourceType, []byte(req.Data))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "MIGRATION_ERROR", err.Error())
		return
	}

	// Return preview without persisting
	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"preview":    true,
			"jobs":       result.Jobs,
			"workflows":  result.Workflows,
			"errors":     result.Errors,
			"warnings":   result.Warnings,
			"total":      result.ItemsTotal,
			"can_import": result.ItemsMigrated,
			"will_fail":  result.ItemsFailed,
		},
	})
}

// MigrateKubernetes handles POST /migration/kubernetes.
func (h *APIHandler) MigrateKubernetes(w http.ResponseWriter, r *http.Request) {
	h.migrateFromBody(w, r, SourceKubernetesCronJob)
}

// MigrateAirflow handles POST /migration/airflow.
func (h *APIHandler) MigrateAirflow(w http.ResponseWriter, r *http.Request) {
	h.migrateFromBody(w, r, SourceAirflowDAG)
}

// MigrateEventBridge handles POST /migration/eventbridge.
func (h *APIHandler) MigrateEventBridge(w http.ResponseWriter, r *http.Request) {
	h.migrateFromBody(w, r, SourceAWSEventBridge)
}

// MigrateTemporal handles POST /migration/temporal.
func (h *APIHandler) MigrateTemporal(w http.ResponseWriter, r *http.Request) {
	h.migrateFromBody(w, r, SourceTemporalSchedule)
}

// MigrateGitHubActions handles POST /migration/github-actions.
func (h *APIHandler) MigrateGitHubActions(w http.ResponseWriter, r *http.Request) {
	h.migrateFromBody(w, r, SourceGitHubActions)
}

func (h *APIHandler) migrateFromBody(w http.ResponseWriter, r *http.Request, sourceType SourceType) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "READ_ERROR", err.Error())
		return
	}

	result, err := h.migrator.Migrate(r.Context(), sourceType, body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "MIGRATION_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("source_type", string(sourceType)).
		Int("items_migrated", result.ItemsMigrated).
		Int("items_failed", result.ItemsFailed).
		Msg("Migration completed")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: result.Success,
		Data:    result,
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
