package cloud

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// Handler handles SaaS control plane API requests.
type Handler struct {
	controlPlane *ControlPlane
	billing      *BillingService
	logger       zerolog.Logger
}

// NewHandler creates a new cloud API handler.
func NewHandler(cp *ControlPlane, billing *BillingService, logger zerolog.Logger) *Handler {
	return &Handler{
		controlPlane: cp,
		billing:      billing,
		logger:       logger.With().Str("component", "cloud").Logger(),
	}
}

// Response wraps API responses.
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

// RegisterRoutes registers cloud API routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/cloud", func(r chi.Router) {
		// Organizations
		r.Route("/organizations", func(r chi.Router) {
			r.Post("/", h.CreateOrganization)
			r.Get("/", h.ListOrganizations)
			
			r.Route("/{orgId}", func(r chi.Router) {
				r.Get("/", h.GetOrganization)
				r.Put("/", h.UpdateOrganization)
				r.Delete("/", h.DeleteOrganization)
				
				// Workspaces
				r.Route("/workspaces", func(r chi.Router) {
					r.Get("/", h.ListWorkspaces)
					r.Post("/", h.CreateWorkspace)
					
					r.Route("/{wsId}", func(r chi.Router) {
						r.Get("/", h.GetWorkspace)
						r.Put("/", h.UpdateWorkspace)
						r.Delete("/", h.DeleteWorkspace)
					})
				})
				
				// Members
				r.Route("/members", func(r chi.Router) {
					r.Get("/", h.ListMembers)
					r.Post("/", h.AddMember)
					
					r.Route("/{memberId}", func(r chi.Router) {
						r.Get("/", h.GetMember)
						r.Put("/", h.UpdateMember)
						r.Delete("/", h.RemoveMember)
					})
				})
				
				// Invitations
				r.Post("/invitations", h.CreateInvitation)
				
				// Billing
				r.Route("/billing", func(r chi.Router) {
					r.Get("/subscription", h.GetSubscription)
					r.Post("/subscription", h.CreateSubscription)
					r.Put("/subscription", h.UpdateSubscription)
					r.Delete("/subscription", h.CancelSubscription)
					
					r.Get("/invoices", h.ListInvoices)
					r.Get("/invoices/{invoiceId}", h.GetInvoice)
					
					r.Get("/usage", h.GetUsageSummary)
				})
			})
		})
		
		// Invitations (public endpoint for accepting)
		r.Post("/invitations/accept", h.AcceptInvitation)
		
		// Plans
		r.Get("/plans", h.ListPlans)
	})
}

// Organization handlers

func (h *Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	var org Organization
	if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	// Set creator from auth context (placeholder)
	org.CreatedBy = r.Header.Get("X-User-ID")

	if err := h.controlPlane.CreateOrganization(r.Context(), &org); err != nil {
		if err == ErrOrganizationExists {
			h.writeError(w, http.StatusConflict, "ORG_EXISTS", "Organization with this slug already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}

	// Create initial subscription
	if h.billing != nil {
		h.billing.CreateSubscription(r.Context(), org.ID, org.Plan)
	}

	h.writeJSON(w, http.StatusCreated, Response{Success: true, Data: org})
}

func (h *Handler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	org, err := h.controlPlane.GetOrganization(r.Context(), orgID)
	if err != nil {
		if err == ErrOrganizationNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: org})
}

func (h *Handler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	
	orgs, err := h.controlPlane.ListOrganizations(r.Context(), userID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: orgs})
}

func (h *Handler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var org Organization
	if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}
	org.ID = orgID

	if err := h.controlPlane.UpdateOrganization(r.Context(), &org); err != nil {
		if err == ErrOrganizationNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: org})
}

func (h *Handler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	if err := h.controlPlane.DeleteOrganization(r.Context(), orgID); err != nil {
		if err == ErrOrganizationNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "DELETE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"id": orgID}})
}

// Workspace handlers

func (h *Handler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var ws Workspace
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}
	ws.OrganizationID = orgID
	ws.CreatedBy = r.Header.Get("X-User-ID")

	if err := h.controlPlane.CreateWorkspace(r.Context(), &ws); err != nil {
		if err == ErrWorkspaceExists {
			h.writeError(w, http.StatusConflict, "WS_EXISTS", "Workspace with this slug already exists")
			return
		}
		if err == ErrOrganizationNotFound {
			h.writeError(w, http.StatusNotFound, "ORG_NOT_FOUND", "Organization not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, Response{Success: true, Data: ws})
}

func (h *Handler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	ws, err := h.controlPlane.GetWorkspace(r.Context(), wsID)
	if err != nil {
		if err == ErrWorkspaceNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: ws})
}

func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	workspaces, err := h.controlPlane.ListWorkspaces(r.Context(), orgID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: workspaces})
}

func (h *Handler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	var ws Workspace
	if err := json.NewDecoder(r.Body).Decode(&ws); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}
	ws.ID = wsID

	if err := h.controlPlane.UpdateWorkspace(r.Context(), &ws); err != nil {
		if err == ErrWorkspaceNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: ws})
}

func (h *Handler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	wsID := chi.URLParam(r, "wsId")

	if err := h.controlPlane.DeleteWorkspace(r.Context(), wsID); err != nil {
		if err == ErrWorkspaceNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "DELETE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"id": wsID}})
}

// Member handlers

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var member Member
	if err := json.NewDecoder(r.Body).Decode(&member); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}
	member.OrganizationID = orgID

	if err := h.controlPlane.AddMember(r.Context(), &member); err != nil {
		if err == ErrMemberExists {
			h.writeError(w, http.StatusConflict, "MEMBER_EXISTS", "Member already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "ADD_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, Response{Success: true, Data: member})
}

func (h *Handler) GetMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberId")

	member, err := h.controlPlane.GetMember(r.Context(), memberID)
	if err != nil {
		if err == ErrMemberNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Member not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: member})
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	members, err := h.controlPlane.ListMembers(r.Context(), orgID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: members})
}

func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberId")

	var member Member
	if err := json.NewDecoder(r.Body).Decode(&member); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}
	member.ID = memberID

	if err := h.controlPlane.UpdateMember(r.Context(), &member); err != nil {
		if err == ErrMemberNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Member not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: member})
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	memberID := chi.URLParam(r, "memberId")

	if err := h.controlPlane.RemoveMember(r.Context(), memberID); err != nil {
		if err == ErrMemberNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Member not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "REMOVE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"id": memberID}})
}

// Invitation handlers

func (h *Handler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var inv Invitation
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}
	inv.OrganizationID = orgID
	inv.InvitedBy = r.Header.Get("X-User-ID")

	if err := h.controlPlane.CreateInvitation(r.Context(), &inv); err != nil {
		h.writeError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, Response{Success: true, Data: inv})
}

func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		UserID   string `json:"user_id"`
		UserName string `json:"user_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	member, err := h.controlPlane.AcceptInvitation(r.Context(), req.Token, req.UserID, req.UserName)
	if err != nil {
		if err == ErrInvitationNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Invitation not found")
			return
		}
		if err == ErrInvitationExpired {
			h.writeError(w, http.StatusGone, "EXPIRED", "Invitation has expired")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "ACCEPT_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: member})
}

// Billing handlers

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	sub, err := h.billing.GetSubscriptionByOrg(r.Context(), orgID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Subscription not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: sub})
}

func (h *Handler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var req struct {
		Plan Plan `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	sub, err := h.billing.CreateSubscription(r.Context(), orgID, req.Plan)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "CREATE_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, Response{Success: true, Data: sub})
}

func (h *Handler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var req struct {
		Plan Plan `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	sub, err := h.billing.GetSubscriptionByOrg(r.Context(), orgID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Subscription not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	if err := h.billing.ChangePlan(r.Context(), sub.ID, req.Plan); err != nil {
		h.writeError(w, http.StatusInternalServerError, "UPDATE_ERROR", err.Error())
		return
	}

	sub, _ = h.billing.GetSubscription(r.Context(), sub.ID)
	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: sub})
}

func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	var req struct {
		Immediate bool `json:"immediate"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	sub, err := h.billing.GetSubscriptionByOrg(r.Context(), orgID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Subscription not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	if err := h.billing.CancelSubscription(r.Context(), sub.ID, req.Immediate); err != nil {
		h.writeError(w, http.StatusInternalServerError, "CANCEL_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"status": "canceled"}})
}

func (h *Handler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	invoices, err := h.billing.ListInvoices(r.Context(), orgID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "LIST_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: invoices})
}

func (h *Handler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "invoiceId")

	invoice, err := h.billing.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		if err == ErrInvoiceNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Invoice not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: invoice})
}

func (h *Handler) GetUsageSummary(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgId")

	// Get current month by default
	sub, err := h.billing.GetSubscriptionByOrg(r.Context(), orgID)
	if err != nil {
		if err == ErrSubscriptionNotFound {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", "Subscription not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "GET_ERROR", err.Error())
		return
	}

	summary, err := h.billing.GetUsageSummary(r.Context(), orgID, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "SUMMARY_ERROR", err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: summary})
}

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans := h.controlPlane.GetAllPlans()
	h.writeJSON(w, http.StatusOK, Response{Success: true, Data: plans})
}

// Helper methods

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
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
