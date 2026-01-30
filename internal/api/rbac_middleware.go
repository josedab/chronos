// Package api provides the REST API router.
package api

import (
	"context"
	"net/http"

	"github.com/chronos/chronos/internal/rbac"
)

// RBACConfig holds RBAC configuration.
type RBACConfig struct {
	// Enabled determines if RBAC checks are enforced.
	Enabled bool
	// Manager is the RBAC manager instance.
	Manager *rbac.RBACManager
}

// RBACContextKey is the context key for RBAC user.
type RBACContextKey string

const (
	// RBACUserContextKey is the context key for the RBAC user.
	RBACUserContextKey RBACContextKey = "rbac_user"
)

// NewRBACMiddleware creates middleware that loads the RBAC user from the API key's tenant.
func NewRBACMiddleware(config RBACConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled || config.Manager == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Get API key from context (set by AuthMiddleware)
			apiKey := GetAPIKeyFromContext(r.Context())
			if apiKey == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Try to load RBAC user by tenant ID (API key's tenant becomes the user context)
			user, err := config.Manager.GetUser(r.Context(), apiKey.TenantID)
			if err != nil {
				// No RBAC user found - create a default user context based on API key scopes
				user = &rbac.User{
					ID:     apiKey.TenantID,
					Status: rbac.UserStatusActive,
					Roles:  mapScopesToRoles(apiKey.Scopes),
				}
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), RBACUserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission creates middleware that checks for a specific permission.
func RequirePermission(config RBACConfig, permission rbac.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled || config.Manager == nil {
				next.ServeHTTP(w, r)
				return
			}

			user := GetRBACUserFromContext(r.Context())
			if user == nil {
				http.Error(w, `{"error":"unauthorized","message":"authentication required"}`, http.StatusUnauthorized)
				return
			}

			// Extract namespace from request (could be path param or header)
			namespace := extractNamespace(r)

			// Check permission
			err := config.Manager.CheckPermissionOrFail(r.Context(), user.ID, permission, namespace)
			if err != nil {
				if rbac.IsPermissionDenied(err) {
					http.Error(w, `{"error":"forbidden","message":"insufficient permissions"}`, http.StatusForbidden)
					return
				}
				http.Error(w, `{"error":"internal_error","message":"permission check failed"}`, http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetRBACUserFromContext retrieves the RBAC user from the request context.
func GetRBACUserFromContext(ctx context.Context) *rbac.User {
	if user, ok := ctx.Value(RBACUserContextKey).(*rbac.User); ok {
		return user
	}
	return nil
}

// mapScopesToRoles maps API key scopes to RBAC roles.
func mapScopesToRoles(scopes []string) []string {
	// Map common scope patterns to roles
	roles := make([]string, 0)
	hasRead := false
	hasWrite := false
	hasAdmin := false

	for _, scope := range scopes {
		switch scope {
		case "read", "jobs:read", "executions:read":
			hasRead = true
		case "write", "jobs:write", "jobs:create", "jobs:update", "jobs:delete":
			hasWrite = true
		case "admin", "*":
			hasAdmin = true
		}
	}

	if hasAdmin {
		roles = append(roles, "admin")
	} else if hasWrite {
		roles = append(roles, "operator")
	} else if hasRead {
		roles = append(roles, "viewer")
	}

	return roles
}

// extractNamespace extracts the namespace from the request.
func extractNamespace(r *http.Request) string {
	// Check header first
	if ns := r.Header.Get("X-Namespace"); ns != "" {
		return ns
	}

	// Check query parameter
	if ns := r.URL.Query().Get("namespace"); ns != "" {
		return ns
	}

	// Default namespace
	return "default"
}

// PermissionMiddlewareFactory creates permission-checking middleware for specific endpoints.
type PermissionMiddlewareFactory struct {
	config RBACConfig
}

// NewPermissionMiddlewareFactory creates a new factory.
func NewPermissionMiddlewareFactory(config RBACConfig) *PermissionMiddlewareFactory {
	return &PermissionMiddlewareFactory{config: config}
}

// Jobs returns middleware for job operations.
func (f *PermissionMiddlewareFactory) Jobs() JobPermissions {
	return JobPermissions{config: f.config}
}

// JobPermissions provides permission middleware for job endpoints.
type JobPermissions struct {
	config RBACConfig
}

// Create returns middleware requiring job:create permission.
func (p JobPermissions) Create() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobCreate)
}

// Read returns middleware requiring job:read permission.
func (p JobPermissions) Read() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobRead)
}

// Update returns middleware requiring job:update permission.
func (p JobPermissions) Update() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobUpdate)
}

// Delete returns middleware requiring job:delete permission.
func (p JobPermissions) Delete() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobDelete)
}

// Trigger returns middleware requiring job:trigger permission.
func (p JobPermissions) Trigger() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobTrigger)
}

// Enable returns middleware requiring job:enable permission.
func (p JobPermissions) Enable() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobEnable)
}

// Disable returns middleware requiring job:disable permission.
func (p JobPermissions) Disable() func(http.Handler) http.Handler {
	return RequirePermission(p.config, rbac.PermJobDisable)
}
