// Package cloud provides control plane API handlers.
package cloud

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// APIHandler handles control plane API requests.
type APIHandler struct {
	cp *ControlPlane
}

// NewAPIHandler creates a new control plane API handler.
func NewAPIHandler(cp *ControlPlane) *APIHandler {
	return &APIHandler{cp: cp}
}

// APIResponse is the standard API response format.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Routes returns the chi router with all control plane routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Public routes
	r.Get("/regions", h.ListRegions)
	r.Get("/plans", h.ListPlans)
	r.Get("/plans/{plan}", h.GetPlan)

	// Tenant management
	r.Post("/tenants", h.CreateTenant)
	r.Get("/tenants/{tenantID}", h.GetTenant)
	r.Put("/tenants/{tenantID}/plan", h.UpdateTenantPlan)
	r.Get("/tenants/{tenantID}/usage", h.GetTenantUsage)
	r.Get("/tenants/{tenantID}/export", h.ExportTenantData)

	// Cluster management
	r.Post("/tenants/{tenantID}/clusters", h.CreateCluster)
	r.Get("/tenants/{tenantID}/clusters", h.ListClusters)
	r.Get("/clusters/{clusterID}", h.GetCluster)
	r.Delete("/clusters/{clusterID}", h.DeleteCluster)
	r.Post("/clusters/{clusterID}/api-key", h.GenerateAPIKey)

	// Quota check
	r.Get("/tenants/{tenantID}/quota", h.CheckQuota)

	// Usage recording (internal)
	r.Post("/usage", h.RecordUsage)

	return r
}

// ListRegions returns available regions.
func (h *APIHandler) ListRegions(w http.ResponseWriter, r *http.Request) {
	regions := h.cp.ListRegions()
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: regions})
}

// ListPlans returns available plans.
func (h *APIHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans := []PlanType{PlanTypeFree, PlanTypeStarter, PlanTypePro, PlanTypeEnterprise}
	details := make([]*PlanDetails, 0, len(plans))

	for _, plan := range plans {
		d, err := h.cp.GetPlanDetails(plan)
		if err == nil {
			details = append(details, d)
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: details})
}

// GetPlan returns details for a specific plan.
func (h *APIHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	plan := PlanType(chi.URLParam(r, "plan"))
	details, err := h.cp.GetPlanDetails(plan)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: details})
}

// CreateTenantRequest is a request to create a tenant.
type CreateTenantRequest struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Organization string `json:"organization,omitempty"`
}

// CreateTenant creates a new tenant.
func (h *APIHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if req.Name == "" || req.Email == "" {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "name and email are required"})
		return
	}

	tenant, err := h.cp.CreateTenant(r.Context(), req.Name, req.Email, req.Organization)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrTenantExists {
			status = http.StatusConflict
		}
		h.writeJSON(w, status, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: tenant})
}

// GetTenant retrieves a tenant.
func (h *APIHandler) GetTenant(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	tenant, err := h.cp.GetTenant(tenantID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: tenant})
}

// UpdatePlanRequest is a request to update a tenant's plan.
type UpdatePlanRequest struct {
	Plan PlanType `json:"plan"`
}

// UpdateTenantPlan updates a tenant's plan.
func (h *APIHandler) UpdateTenantPlan(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req UpdatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if err := h.cp.UpdateTenantPlan(tenantID, req.Plan); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	tenant, _ := h.cp.GetTenant(tenantID)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: tenant})
}

// GetTenantUsage retrieves usage metrics for a tenant.
func (h *APIHandler) GetTenantUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().Format("2006-01")
	}

	usage, err := h.cp.GetUsage(tenantID, period)
	if err != nil {
		h.writeJSON(w, http.StatusInternalServerError, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"tenant_id": tenantID,
			"period":    period,
			"usage":     usage,
		},
	})
}

// ExportTenantData exports all tenant data.
func (h *APIHandler) ExportTenantData(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	data, err := h.cp.ExportTenantData(tenantID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=tenant-export.json")
	w.Write(data)
}

// CreateClusterRequest is a request to create a cluster.
type CreateClusterRequest struct {
	Name   string      `json:"name"`
	Region string      `json:"region"`
	Tier   ClusterTier `json:"tier"`
}

// CreateCluster creates a new cluster.
func (h *APIHandler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	if req.Name == "" || req.Region == "" {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "name and region are required"})
		return
	}

	if req.Tier == "" {
		req.Tier = ClusterTierDev
	}

	cluster, err := h.cp.CreateCluster(r.Context(), tenantID, req.Name, req.Region, req.Tier)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrTenantNotFound {
			status = http.StatusNotFound
		} else if err == ErrQuotaExceeded {
			status = http.StatusForbidden
		}
		h.writeJSON(w, status, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: cluster})
}

// ListClusters lists clusters for a tenant.
func (h *APIHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	clusters, err := h.cp.ListTenantClusters(tenantID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: clusters})
}

// GetCluster retrieves a cluster.
func (h *APIHandler) GetCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterID")

	cluster, err := h.cp.GetCluster(clusterID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: cluster})
}

// DeleteCluster deletes a cluster.
func (h *APIHandler) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterID")

	if err := h.cp.DeleteCluster(r.Context(), clusterID); err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

// GenerateAPIKey generates a new API key for a cluster.
func (h *APIHandler) GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	clusterID := chi.URLParam(r, "clusterID")

	apiKey, err := h.cp.GenerateAPIKey(clusterID)
	if err != nil {
		h.writeJSON(w, http.StatusNotFound, APIResponse{Error: err.Error()})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]string{
			"api_key": apiKey,
		},
	})
}

// CheckQuota checks if a tenant has quota remaining.
func (h *APIHandler) CheckQuota(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	operation := r.URL.Query().Get("operation")

	err := h.cp.CheckQuota(tenantID, operation)
	if err != nil {
		status := http.StatusForbidden
		if err == ErrTenantNotFound {
			status = http.StatusNotFound
		}
		h.writeJSON(w, status, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]bool{
			"has_quota": true,
		},
	})
}

// RecordUsageRequest is a request to record usage.
type RecordUsageRequest struct {
	TenantID        string `json:"tenant_id"`
	ClusterID       string `json:"cluster_id"`
	Executions      int64  `json:"executions"`
	ExecutionTimeMs int64  `json:"execution_time_ms"`
}

// RecordUsage records usage metrics.
func (h *APIHandler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	var req RecordUsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSON(w, http.StatusBadRequest, APIResponse{Error: "invalid request body"})
		return
	}

	h.cp.RecordUsage(req.TenantID, req.ClusterID, req.Executions, req.ExecutionTimeMs)
	h.writeJSON(w, http.StatusOK, APIResponse{Success: true})
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
