package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultVaultConfig(t *testing.T) {
	cfg := DefaultVaultConfig()

	if cfg.Address != "http://127.0.0.1:8200" {
		t.Errorf("expected default address http://127.0.0.1:8200, got %s", cfg.Address)
	}
	if cfg.MountPath != "secret" {
		t.Errorf("expected default mount path secret, got %s", cfg.MountPath)
	}
	if cfg.AuthMethod != "token" {
		t.Errorf("expected default auth method token, got %s", cfg.AuthMethod)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", cfg.Timeout)
	}
	if cfg.CacheTTL != 5*time.Minute {
		t.Errorf("expected default cache TTL 5m, got %v", cfg.CacheTTL)
	}
}

func TestNewVaultProvider(t *testing.T) {
	cfg := VaultConfig{
		Address:    "http://vault:8200",
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if provider.config.Address != "http://vault:8200" {
		t.Errorf("expected address http://vault:8200, got %s", provider.config.Address)
	}
}

func TestVaultProvider_Name(t *testing.T) {
	provider, _ := NewVaultProvider(VaultConfig{
		Address:    "http://vault:8200",
		Token:      "test-token",
		AuthMethod: "token",
	})
	if provider.Name() != "vault" {
		t.Errorf("expected name vault, got %s", provider.Name())
	}
}

func TestVaultProvider_GetSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		response := VaultResponse{
			Data: map[string]interface{}{
				"data": map[string]interface{}{
					"value": "secret-value",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
		CacheTTL:   0,
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	value, err := provider.GetSecret(context.Background(), "my-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != "secret-value" {
		t.Errorf("expected secret-value, got %s", value)
	}
}

func TestVaultProvider_GetSecret_WithField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := VaultResponse{
			Data: map[string]interface{}{
				"data": map[string]interface{}{
					"username": "admin",
					"password": "secret123",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	value, err := provider.GetSecret(context.Background(), "my-secret#password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != "secret123" {
		t.Errorf("expected secret123, got %s", value)
	}
}

func TestVaultProvider_GetSecret_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors": ["secret not found"]}`))
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	_, err = provider.GetSecret(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent secret")
	}
}

func TestVaultProvider_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := VaultResponse{
			Data: map[string]interface{}{
				"data": map[string]interface{}{
					"value": "cached-secret",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
		CacheTTL:   1 * time.Minute,
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	// First call
	_, err = provider.GetSecret(context.Background(), "cached-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should use cache
	_, err = provider.GetSecret(context.Background(), "cached-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 API call (due to caching), got %d", callCount)
	}
}

func TestVaultProvider_ListSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "LIST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"keys": []string{"secret1", "secret2", "secret3"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	keys, err := provider.ListSecrets(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestVaultProvider_WriteSecret(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)

		response := VaultResponse{
			Data: map[string]interface{}{
				"version": 1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	err = provider.WriteSecret(context.Background(), "new-secret", map[string]interface{}{
		"username": "admin",
		"password": "secret123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVaultProvider_DeleteSecret(t *testing.T) {
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	err = provider.DeleteSecret(context.Background(), "old-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleted {
		t.Error("expected DELETE request to be made")
	}
}

func TestVaultProvider_Health(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			response := map[string]interface{}{
				"initialized": true,
				"sealed":      false,
				"standby":     false,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	err = provider.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVaultProvider_HealthSealed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"initialized": true,
			"sealed":      true,
			"standby":     false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := VaultConfig{
		Address:    server.URL,
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	err = provider.Health(context.Background())
	if err == nil {
		t.Error("expected error for sealed vault")
	}
}

func TestVaultProvider_ClearCache(t *testing.T) {
	cfg := VaultConfig{
		Address:    "http://vault:8200",
		Token:      "test-token",
		MountPath:  "secret",
		AuthMethod: "token",
		CacheTTL:   5 * time.Minute,
	}

	provider, err := NewVaultProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating provider: %v", err)
	}

	// Manually add to cache
	provider.cacheMu.Lock()
	provider.cache["test-key"] = &cachedSecret{
		value:     "test-value",
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	provider.cacheMu.Unlock()

	// Clear cache
	provider.ClearCache()

	// Verify cache is empty
	provider.cacheMu.RLock()
	defer provider.cacheMu.RUnlock()
	if len(provider.cache) != 0 {
		t.Error("expected cache to be empty after clearing")
	}
}
