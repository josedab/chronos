// Package routing provides API handlers for routing operations.
package routing

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// APIHandler handles routing API requests.
type APIHandler struct {
	router *Router
	logger zerolog.Logger
}

// NewAPIHandler creates a new routing API handler.
func NewAPIHandler(router *Router, logger zerolog.Logger) *APIHandler {
	return &APIHandler{
		router: router,
		logger: logger.With().Str("component", "routing-api").Logger(),
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

// RegisterRegionRequest is the request for registering a region.
type RegisterRegionRequest struct {
	Name      string            `json:"name"`
	Provider  CloudProvider     `json:"provider"`
	Location  GeoLocation       `json:"location"`
	Endpoints []*Endpoint       `json:"endpoints,omitempty"`
	Cost      *RegionCost       `json:"cost,omitempty"`
	Priority  int               `json:"priority"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SetPolicyRequest is the request for setting a routing policy.
type SetPolicyRequest struct {
	Name        string            `json:"name"`
	Strategy    RoutingStrategy   `json:"strategy"`
	Preferences *RoutePreferences `json:"preferences,omitempty"`
	Constraints *RouteConstraints `json:"constraints,omitempty"`
	Failover    *FailoverConfig   `json:"failover,omitempty"`
	IsDefault   bool              `json:"is_default"`
}

// RouteRequest is the request for routing a job.
type RouteRequest struct {
	JobID    string `json:"job_id"`
	PolicyID string `json:"policy_id,omitempty"`
}

// UpdateHealthRequest is the request for updating region health.
type UpdateHealthRequest struct {
	Status       HealthStatus `json:"status"`
	Latency      string       `json:"latency"` // Duration string
	SuccessRate  float64      `json:"success_rate"`
	Availability float64      `json:"availability"`
}

// Routes registers the routing API routes.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Region management
	r.Get("/regions", h.ListRegions)
	r.Post("/regions", h.RegisterRegion)
	r.Get("/regions/{regionID}", h.GetRegion)
	r.Put("/regions/{regionID}", h.UpdateRegion)
	r.Delete("/regions/{regionID}", h.UnregisterRegion)
	r.Put("/regions/{regionID}/health", h.UpdateRegionHealth)
	r.Put("/regions/{regionID}/endpoints/{endpointID}/health", h.UpdateEndpointHealth)

	// Policy management
	r.Get("/policies", h.ListPolicies)
	r.Post("/policies", h.SetPolicy)
	r.Get("/policies/{policyID}", h.GetPolicy)
	r.Delete("/policies/{policyID}", h.DeletePolicy)
	r.Post("/policies/default", h.SetDefaultPolicy)

	// Routing operations
	r.Post("/route", h.Route)
	r.Post("/simulate", h.SimulateRoute)

	// Client location
	r.Post("/client-location", h.SetClientLocation)

	return r
}

// ListRegions handles GET /regions.
func (h *APIHandler) ListRegions(w http.ResponseWriter, r *http.Request) {
	regions := h.router.ListRegions()

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"regions": regions,
			"total":   len(regions),
		},
	})
}

// RegisterRegion handles POST /regions.
func (h *APIHandler) RegisterRegion(w http.ResponseWriter, r *http.Request) {
	var req RegisterRegionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "region name is required")
		return
	}

	region := &Region{
		Name:      req.Name,
		Provider:  req.Provider,
		Location:  req.Location,
		Endpoints: req.Endpoints,
		Cost:      req.Cost,
		Priority:  req.Priority,
		Metadata:  req.Metadata,
		Enabled:   true,
	}

	if err := h.router.RegisterRegion(region); err != nil {
		h.writeError(w, http.StatusInternalServerError, "REGISTER_ERROR", err.Error())
		return
	}

	h.logger.Info().
		Str("region_id", region.ID).
		Str("name", region.Name).
		Msg("Region registered")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    region,
	})
}

// GetRegion handles GET /regions/{regionID}.
func (h *APIHandler) GetRegion(w http.ResponseWriter, r *http.Request) {
	regionID := chi.URLParam(r, "regionID")

	region, err := h.router.GetRegion(regionID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    region,
	})
}

// UpdateRegion handles PUT /regions/{regionID}.
func (h *APIHandler) UpdateRegion(w http.ResponseWriter, r *http.Request) {
	regionID := chi.URLParam(r, "regionID")

	var req RegisterRegionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	h.router.mu.Lock()
	defer h.router.mu.Unlock()

	region, exists := h.router.regions[regionID]
	if !exists {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "region not found")
		return
	}

	if req.Name != "" {
		region.Name = req.Name
	}
	if req.Provider != "" {
		region.Provider = req.Provider
	}
	if req.Endpoints != nil {
		region.Endpoints = req.Endpoints
	}
	if req.Cost != nil {
		region.Cost = req.Cost
	}
	if req.Priority != 0 {
		region.Priority = req.Priority
	}
	region.UpdatedAt = time.Now().UTC()

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    region,
	})
}

// UnregisterRegion handles DELETE /regions/{regionID}.
func (h *APIHandler) UnregisterRegion(w http.ResponseWriter, r *http.Request) {
	regionID := chi.URLParam(r, "regionID")

	if err := h.router.UnregisterRegion(regionID); err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.logger.Info().Str("region_id", regionID).Msg("Region unregistered")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"deleted": regionID},
	})
}

// UpdateRegionHealth handles PUT /regions/{regionID}/health.
func (h *APIHandler) UpdateRegionHealth(w http.ResponseWriter, r *http.Request) {
	regionID := chi.URLParam(r, "regionID")

	var req UpdateHealthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	latency, _ := time.ParseDuration(req.Latency)

	health := &RegionHealth{
		Status:       req.Status,
		Latency:      latency,
		SuccessRate:  req.SuccessRate,
		Availability: req.Availability,
		LastCheck:    time.Now().UTC(),
	}

	if err := h.router.UpdateRegionHealth(regionID, health); err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    health,
	})
}

// UpdateEndpointHealth handles PUT /regions/{regionID}/endpoints/{endpointID}/health.
func (h *APIHandler) UpdateEndpointHealth(w http.ResponseWriter, r *http.Request) {
	regionID := chi.URLParam(r, "regionID")
	endpointID := chi.URLParam(r, "endpointID")

	var req UpdateHealthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	latency, _ := time.ParseDuration(req.Latency)

	health := &EndpointHealth{
		Status:      req.Status,
		Latency:     latency,
		SuccessRate: req.SuccessRate,
		LastCheck:   time.Now().UTC(),
	}

	if err := h.router.UpdateEndpointHealth(regionID, endpointID, health); err != nil {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    health,
	})
}

// ListPolicies handles GET /policies.
func (h *APIHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	h.router.mu.RLock()
	policies := make([]*RoutingPolicy, 0, len(h.router.policies))
	for _, p := range h.router.policies {
		policies = append(policies, p)
	}
	defaultPolicy := h.router.defaultPolicy
	h.router.mu.RUnlock()

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"policies":       policies,
			"default_policy": defaultPolicy,
			"total":          len(policies),
		},
	})
}

// SetPolicy handles POST /policies.
func (h *APIHandler) SetPolicy(w http.ResponseWriter, r *http.Request) {
	var req SetPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "policy name is required")
		return
	}

	policy := &RoutingPolicy{
		Name:        req.Name,
		Strategy:    req.Strategy,
		Preferences: req.Preferences,
		Constraints: req.Constraints,
		Failover:    req.Failover,
	}

	h.router.SetPolicy(policy)

	if req.IsDefault {
		h.router.SetDefaultPolicy(policy)
	}

	h.logger.Info().
		Str("policy_id", policy.ID).
		Str("name", policy.Name).
		Str("strategy", string(policy.Strategy)).
		Msg("Policy created")

	h.writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    policy,
	})
}

// GetPolicy handles GET /policies/{policyID}.
func (h *APIHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policyID")

	h.router.mu.RLock()
	policy, exists := h.router.policies[policyID]
	h.router.mu.RUnlock()

	if !exists {
		h.writeError(w, http.StatusNotFound, "NOT_FOUND", "policy not found")
		return
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    policy,
	})
}

// DeletePolicy handles DELETE /policies/{policyID}.
func (h *APIHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := chi.URLParam(r, "policyID")

	h.router.mu.Lock()
	delete(h.router.policies, policyID)
	h.router.mu.Unlock()

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    map[string]string{"deleted": policyID},
	})
}

// SetDefaultPolicy handles POST /policies/default.
func (h *APIHandler) SetDefaultPolicy(w http.ResponseWriter, r *http.Request) {
	var req SetPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	policy := &RoutingPolicy{
		Name:        req.Name,
		Strategy:    req.Strategy,
		Preferences: req.Preferences,
		Constraints: req.Constraints,
		Failover:    req.Failover,
	}

	h.router.SetDefaultPolicy(policy)

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    policy,
	})
}

// Route handles POST /route.
func (h *APIHandler) Route(w http.ResponseWriter, r *http.Request) {
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.JobID == "" {
		h.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "job_id is required")
		return
	}

	decision, err := h.router.Route(r.Context(), req.JobID, req.PolicyID)
	if err != nil {
		h.writeError(w, http.StatusServiceUnavailable, "ROUTING_ERROR", err.Error())
		return
	}

	h.logger.Debug().
		Str("job_id", req.JobID).
		Str("region", decision.SelectedRegion.ID).
		Str("reason", decision.Reason).
		Msg("Routing decision made")

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    decision,
	})
}

// SimulateRoute handles POST /simulate.
func (h *APIHandler) SimulateRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RouteRequest
		ClientLocation *GeoLocation `json:"client_location,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// Temporarily set client location if provided
	if req.ClientLocation != nil {
		h.router.SetClientLocation(req.ClientLocation)
		defer h.router.SetClientLocation(nil)
	}

	decision, err := h.router.Route(r.Context(), req.JobID, req.PolicyID)
	if err != nil {
		h.writeError(w, http.StatusServiceUnavailable, "ROUTING_ERROR", err.Error())
		return
	}

	// Get all scores for simulation
	h.router.mu.RLock()
	policy := h.router.defaultPolicy
	if req.PolicyID != "" {
		if p, exists := h.router.policies[req.PolicyID]; exists {
			policy = p
		}
	}
	available := h.router.getAvailableRegions(policy)
	scored := h.router.scoreRegions(available, policy)
	h.router.mu.RUnlock()

	simulationResult := struct {
		Decision    *RouteDecision   `json:"decision"`
		AllScores   []map[string]interface{} `json:"all_scores"`
		PolicyUsed  *RoutingPolicy   `json:"policy_used"`
	}{
		Decision:   decision,
		AllScores:  make([]map[string]interface{}, len(scored)),
		PolicyUsed: policy,
	}

	for i, s := range scored {
		simulationResult.AllScores[i] = map[string]interface{}{
			"region_id": s.region.ID,
			"name":      s.region.Name,
			"score":     s.score,
			"provider":  s.region.Provider,
		}
	}

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    simulationResult,
	})
}

// SetClientLocation handles POST /client-location.
func (h *APIHandler) SetClientLocation(w http.ResponseWriter, r *http.Request) {
	var loc GeoLocation
	if err := json.NewDecoder(r.Body).Decode(&loc); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	h.router.SetClientLocation(&loc)

	h.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    loc,
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
