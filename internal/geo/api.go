// Package geo provides REST API handlers for federation management.
package geo

import (
	"encoding/json"
	"net/http"

	"github.com/chronos/chronos/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// APIHandler provides HTTP handlers for federation management.
type APIHandler struct {
	federation *FederationManager
}

// NewAPIHandler creates a new federation API handler.
func NewAPIHandler(federation *FederationManager) *APIHandler {
	return &APIHandler{federation: federation}
}

// Routes returns a chi router with federation endpoints.
func (h *APIHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/clusters", h.ListClusters)
	r.Post("/clusters", h.AddCluster)
	r.Get("/clusters/{id}", h.GetCluster)
	r.Get("/clusters/{id}/status", h.GetClusterStatus)
	r.Delete("/clusters/{id}", h.RemoveCluster)
	return r
}

func (h *APIHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	clusters := h.federation.ListClusters()
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"clusters": clusters, "total": len(clusters)},
	})
}

func (h *APIHandler) AddCluster(w http.ResponseWriter, r *http.Request) {
	var cluster FederatedCluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "invalid_body", "message": err.Error()},
		})
		return
	}

	if err := h.federation.AddCluster(&cluster); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "add_cluster_error", "message": err.Error()},
		})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    cluster,
	})
}

func (h *APIHandler) GetCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cluster, err := h.federation.GetCluster(id)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": err.Error()},
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": cluster})
}

func (h *APIHandler) GetClusterStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := h.federation.GetClusterStatus(id)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": err.Error()},
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": status})
}

func (h *APIHandler) RemoveCluster(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.federation.RemoveCluster(id); err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": err.Error()},
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}
