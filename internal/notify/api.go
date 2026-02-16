// Package notify provides REST API handlers for alert management.
package notify

import (
	"encoding/json"
	"net/http"

	"github.com/chronos/chronos/internal/httputil"
	"github.com/go-chi/chi/v5"
)

// AlertHandler provides HTTP handlers for alert rule management.
type AlertHandler struct {
	engine *AlertEngine
}

// NewAlertHandler creates a new alert handler.
func NewAlertHandler(engine *AlertEngine) *AlertHandler {
	return &AlertHandler{engine: engine}
}

// Routes returns a chi router with alert management endpoints.
func (h *AlertHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/rules", h.ListRules)
	r.Post("/rules", h.CreateRule)
	r.Get("/rules/{id}", h.GetRule)
	r.Delete("/rules/{id}", h.DeleteRule)
	r.Get("/history", h.DeliveryHistory)
	r.Post("/test", h.TestAlert)

	return r
}

func (h *AlertHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules := h.engine.ListRules()
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"rules": rules, "total": len(rules)},
	})
}

func (h *AlertHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "invalid_body", "message": err.Error()},
		})
		return
	}

	if err := h.engine.AddRule(&rule); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "validation_error", "message": err.Error()},
		})
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    rule,
	})
}

func (h *AlertHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.engine.GetRule(id)
	if err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": err.Error()},
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": rule})
}

func (h *AlertHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.engine.RemoveRule(id); err != nil {
		httputil.WriteJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "not_found", "message": err.Error()},
		})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *AlertHandler) DeliveryHistory(w http.ResponseWriter, r *http.Request) {
	history := h.engine.DeliveryHistory(100)
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"records": history, "total": len(history)},
	})
}

func (h *AlertHandler) TestAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Event   EventType `json:"event"`
		JobID   string    `json:"job_id"`
		JobName string    `json:"job_name"`
		Error   string    `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   map[string]string{"code": "invalid_body", "message": err.Error()},
		})
		return
	}

	records := h.engine.Evaluate(r.Context(), req.Event, req.JobID, req.JobName, "test-exec", req.Error)
	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    map[string]interface{}{"deliveries": records, "matched": len(records)},
	})
}
