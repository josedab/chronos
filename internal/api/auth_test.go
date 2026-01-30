package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chronos/chronos/internal/tenant"
)

func TestNewAuthMiddleware_Disabled(t *testing.T) {
	config := AuthConfig{Enabled: false}
	middleware := NewAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestNewAuthMiddleware_MissingAPIKey(t *testing.T) {
	mgr := tenant.NewManager()
	config := AuthConfig{Enabled: true, TenantManager: mgr}
	middleware := NewAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}

func TestNewAuthMiddleware_InvalidAPIKey(t *testing.T) {
	mgr := tenant.NewManager()
	config := AuthConfig{Enabled: true, TenantManager: mgr}
	middleware := NewAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "invalid_key_12345")
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestNewAuthMiddleware_ValidAPIKey(t *testing.T) {
	mgr := tenant.NewManager()

	// Create tenant and API key
	tenantObj := &tenant.Tenant{
		Name:   "Test Tenant",
		Email:  "test@example.com",
		Status: tenant.TenantStatusActive,
	}
	if err := mgr.CreateTenant(context.Background(), tenantObj); err != nil {
		t.Fatalf("failed to create tenant: %v", err)
	}

	_, keyValue, err := mgr.CreateAPIKey(context.Background(), tenantObj.ID, "test-key", []string{"read"})
	if err != nil {
		t.Fatalf("failed to create API key: %v", err)
	}

	config := AuthConfig{Enabled: true, TenantManager: mgr}
	middleware := NewAuthMiddleware(config)

	var capturedTenant *tenant.Tenant
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = GetTenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", keyValue)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if capturedTenant == nil {
		t.Error("expected tenant in context")
	} else if capturedTenant.ID != tenantObj.ID {
		t.Errorf("expected tenant ID %s, got %s", tenantObj.ID, capturedTenant.ID)
	}
}

func TestNewAuthMiddleware_BearerToken(t *testing.T) {
	mgr := tenant.NewManager()

	// Create tenant and API key
	tenantObj := &tenant.Tenant{
		Name:   "Test Tenant",
		Email:  "test@example.com",
		Status: tenant.TenantStatusActive,
	}
	mgr.CreateTenant(context.Background(), tenantObj)
	_, keyValue, _ := mgr.CreateAPIKey(context.Background(), tenantObj.ID, "test-key", []string{"read"})

	config := AuthConfig{Enabled: true, TenantManager: mgr}
	middleware := NewAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+keyValue)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestNewAuthMiddleware_InactiveTenant(t *testing.T) {
	mgr := tenant.NewManager()

	// Create tenant and API key
	tenantObj := &tenant.Tenant{
		Name:   "Test Tenant",
		Email:  "test@example.com",
		Status: tenant.TenantStatusSuspended, // Inactive
	}
	mgr.CreateTenant(context.Background(), tenantObj)
	_, keyValue, _ := mgr.CreateAPIKey(context.Background(), tenantObj.ID, "test-key", []string{"read"})

	config := AuthConfig{Enabled: true, TenantManager: mgr}
	middleware := NewAuthMiddleware(config)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", keyValue)
	rr := httptest.NewRecorder()

	middleware(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for inactive tenant, got %d", rr.Code)
	}
}

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		value     string
		expected  string
	}{
		{"X-API-Key header", "X-API-Key", "my_api_key", "my_api_key"},
		{"Bearer token", "Authorization", "Bearer my_bearer_token", "my_bearer_token"},
		{"ApiKey token", "Authorization", "ApiKey my_api_key", "my_api_key"},
		{"No header", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}

			result := extractAPIKey(req)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetTenantFromContext(t *testing.T) {
	// Test with no tenant
	ctx := context.Background()
	if GetTenantFromContext(ctx) != nil {
		t.Error("expected nil tenant from empty context")
	}

	// Test with tenant
	tenantObj := &tenant.Tenant{ID: "test-id", Name: "Test"}
	ctx = context.WithValue(ctx, TenantContextKey, tenantObj)
	result := GetTenantFromContext(ctx)
	if result == nil || result.ID != "test-id" {
		t.Error("expected tenant from context")
	}
}

func TestGetAPIKeyFromContext(t *testing.T) {
	// Test with no API key
	ctx := context.Background()
	if GetAPIKeyFromContext(ctx) != nil {
		t.Error("expected nil API key from empty context")
	}

	// Test with API key
	apiKey := &tenant.APIKey{ID: "key-id", Name: "Test Key"}
	ctx = context.WithValue(ctx, APIKeyContextKey, apiKey)
	result := GetAPIKeyFromContext(ctx)
	if result == nil || result.ID != "key-id" {
		t.Error("expected API key from context")
	}
}
