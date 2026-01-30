// Package api provides the REST API router.
package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/chronos/chronos/internal/tenant"
)

// Context keys for authentication.
type contextKey string

const (
	// TenantContextKey is the context key for the authenticated tenant.
	TenantContextKey contextKey = "tenant"
	// APIKeyContextKey is the context key for the authenticated API key.
	APIKeyContextKey contextKey = "apiKey"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// Enabled determines if authentication is required.
	Enabled bool
	// TenantManager is the tenant manager for API key validation.
	TenantManager *tenant.Manager
}

// NewAuthMiddleware creates an authentication middleware.
func NewAuthMiddleware(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Extract API key from header
			apiKey := extractAPIKey(r)
			if apiKey == "" {
				http.Error(w, `{"error":"unauthorized","message":"API key required"}`, http.StatusUnauthorized)
				return
			}

			// Validate API key
			if config.TenantManager == nil {
				http.Error(w, `{"error":"internal_error","message":"authentication not configured"}`, http.StatusInternalServerError)
				return
			}

			tenantObj, apiKeyObj, err := config.TenantManager.ValidateAPIKey(r.Context(), apiKey)
			if err != nil {
				if err == tenant.ErrInvalidAPIKey {
					http.Error(w, `{"error":"forbidden","message":"invalid API key"}`, http.StatusForbidden)
					return
				}
				if err == tenant.ErrTenantNotFound {
					http.Error(w, `{"error":"forbidden","message":"tenant not found"}`, http.StatusForbidden)
					return
				}
				http.Error(w, `{"error":"internal_error","message":"authentication failed"}`, http.StatusInternalServerError)
				return
			}

			// Check if tenant is active
			if tenantObj.Status != tenant.TenantStatusActive {
				http.Error(w, `{"error":"forbidden","message":"tenant is not active"}`, http.StatusForbidden)
				return
			}

			// Add tenant and API key to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, TenantContextKey, tenantObj)
			ctx = context.WithValue(ctx, APIKeyContextKey, apiKeyObj)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractAPIKey extracts the API key from the request.
// Supports: X-API-Key header, Authorization: Bearer token, Authorization: ApiKey token
func extractAPIKey(r *http.Request) string {
	// Check X-API-Key header
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Support "Bearer <token>" format
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Support "ApiKey <token>" format
	if strings.HasPrefix(auth, "ApiKey ") {
		return strings.TrimPrefix(auth, "ApiKey ")
	}

	return ""
}

// GetTenantFromContext retrieves the tenant from the request context.
func GetTenantFromContext(ctx context.Context) *tenant.Tenant {
	if t, ok := ctx.Value(TenantContextKey).(*tenant.Tenant); ok {
		return t
	}
	return nil
}

// GetAPIKeyFromContext retrieves the API key from the request context.
func GetAPIKeyFromContext(ctx context.Context) *tenant.APIKey {
	if k, ok := ctx.Value(APIKeyContextKey).(*tenant.APIKey); ok {
		return k
	}
	return nil
}
