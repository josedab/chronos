package secrets

import (
	"context"
	"testing"
)

func TestManagerInjectSecretsLegacyFormat(t *testing.T) {
	manager := NewManager()

	// Set up test environment
	t.Setenv("TEST_LEGACY_SECRET", "legacy-value")

	// Register env provider
	envProvider := NewEnvProvider("")
	if err := manager.RegisterProvider(envProvider); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	// Test standard format still works
	t.Run("StandardFormat", func(t *testing.T) {
		input := "Value: ${secret:env:TEST_LEGACY_SECRET}"
		result, err := manager.InjectSecrets(context.Background(), input)
		if err != nil {
			t.Fatalf("InjectSecrets failed: %v", err)
		}
		expected := "Value: legacy-value"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	// Test legacy format (${provider:key})
	t.Run("LegacyFormat", func(t *testing.T) {
		input := "Value: ${env:TEST_LEGACY_SECRET}"
		result, err := manager.InjectSecrets(context.Background(), input)
		if err != nil {
			t.Fatalf("InjectSecrets failed: %v", err)
		}
		expected := "Value: legacy-value"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	// Test mixed formats
	t.Run("MixedFormats", func(t *testing.T) {
		t.Setenv("TEST_MIXED_1", "value1")
		t.Setenv("TEST_MIXED_2", "value2")

		input := "First: ${secret:env:TEST_MIXED_1}, Second: ${env:TEST_MIXED_2}"
		result, err := manager.InjectSecrets(context.Background(), input)
		if err != nil {
			t.Fatalf("InjectSecrets failed: %v", err)
		}
		expected := "First: value1, Second: value2"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})
}

func TestManagerValidateSecretReferences(t *testing.T) {
	manager := NewManager()

	t.Setenv("VALID_SECRET", "valid")
	envProvider := NewEnvProvider("")
	manager.RegisterProvider(envProvider)

	t.Run("ValidReferences", func(t *testing.T) {
		input := "Connect with ${secret:env:VALID_SECRET}"
		errors := manager.ValidateSecretReferences(context.Background(), input)
		if len(errors) != 0 {
			t.Errorf("expected no errors, got %v", errors)
		}
	})

	t.Run("InvalidReferences", func(t *testing.T) {
		input := "Connect with ${secret:env:INVALID_SECRET}"
		errors := manager.ValidateSecretReferences(context.Background(), input)
		if len(errors) == 0 {
			t.Error("expected errors for invalid reference")
		}
	})

	t.Run("MixedValidInvalid", func(t *testing.T) {
		input := "Valid: ${secret:env:VALID_SECRET}, Invalid: ${secret:env:MISSING}"
		errors := manager.ValidateSecretReferences(context.Background(), input)
		if len(errors) != 1 {
			t.Errorf("expected 1 error, got %d: %v", len(errors), errors)
		}
	})
}

func TestManagerInjectSecretsInWebhookConfig(t *testing.T) {
	manager := NewManager()

	t.Setenv("API_TOKEN", "secret-token-123")
	t.Setenv("API_KEY", "api-key-456")
	t.Setenv("DB_PASS", "db-password")

	envProvider := NewEnvProvider("")
	manager.RegisterProvider(envProvider)

	config := &WebhookSecretConfig{
		URL:    "https://api.example.com/endpoint",
		Method: "POST",
		Headers: map[string]string{
			"Authorization": "Bearer ${secret:env:API_TOKEN}",
			"X-API-Key":     "${secret:env:API_KEY}",
		},
		Body: `{"password": "${secret:env:DB_PASS}"}`,
		Auth: &AuthSecretConfig{
			Type:  "bearer",
			Token: "${secret:env:API_TOKEN}",
		},
	}

	result, err := manager.InjectSecretsInWebhookConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("InjectSecretsInWebhookConfig failed: %v", err)
	}

	// Check URL (no secrets)
	if result.URL != config.URL {
		t.Errorf("URL mismatch: %q", result.URL)
	}

	// Check headers
	if result.Headers["Authorization"] != "Bearer secret-token-123" {
		t.Errorf("Authorization header mismatch: %q", result.Headers["Authorization"])
	}
	if result.Headers["X-API-Key"] != "api-key-456" {
		t.Errorf("X-API-Key header mismatch: %q", result.Headers["X-API-Key"])
	}

	// Check body
	expectedBody := `{"password": "db-password"}`
	if result.Body != expectedBody {
		t.Errorf("Body mismatch: got %q, want %q", result.Body, expectedBody)
	}

	// Check auth
	if result.Auth == nil {
		t.Fatal("Auth is nil")
	}
	if result.Auth.Token != "secret-token-123" {
		t.Errorf("Auth token mismatch: %q", result.Auth.Token)
	}
}

func TestManagerListProviders(t *testing.T) {
	manager := NewManager()

	envProvider := NewEnvProvider("")
	fileProvider := NewFileProvider("/tmp")

	manager.RegisterProvider(envProvider)
	manager.RegisterProvider(fileProvider)

	providers := manager.ListProviders()

	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}

	foundEnv := false
	foundFile := false
	for _, p := range providers {
		if p == "env" {
			foundEnv = true
		}
		if p == "file" {
			foundFile = true
		}
	}

	if !foundEnv {
		t.Error("env provider not found")
	}
	if !foundFile {
		t.Error("file provider not found")
	}
}

func TestVaultProviderParseSecretKey(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantPath  string
		wantField string
	}{
		{
			name:      "simple path",
			key:       "secret/data/myapp",
			wantPath:  "secret/data/myapp",
			wantField: "",
		},
		{
			name:      "path with field",
			key:       "secret/data/myapp#password",
			wantPath:  "secret/data/myapp",
			wantField: "password",
		},
		{
			name:      "nested path with field",
			key:       "secret/data/env/prod/db#connection_string",
			wantPath:  "secret/data/env/prod/db",
			wantField: "connection_string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, field := parseSecretKey(tt.key)
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if field != tt.wantField {
				t.Errorf("field = %q, want %q", field, tt.wantField)
			}
		})
	}
}

func TestAWSSecretsManagerConfig(t *testing.T) {
	cfg := DefaultAWSSecretsManagerConfig()

	if cfg.Region != "us-east-1" {
		t.Errorf("default region = %q, want us-east-1", cfg.Region)
	}

	if cfg.CacheTTL != 5*60*1000000000 { // 5 minutes in nanoseconds
		t.Errorf("default cache TTL = %v, want 5m", cfg.CacheTTL)
	}
}

func TestGCPSecretManagerConfig(t *testing.T) {
	cfg := DefaultGCPSecretManagerConfig()

	if cfg.DefaultVersion != "latest" {
		t.Errorf("default version = %q, want latest", cfg.DefaultVersion)
	}

	if cfg.CacheTTL != 5*60*1000000000 {
		t.Errorf("default cache TTL = %v, want 5m", cfg.CacheTTL)
	}
}

func TestAzureKeyVaultConfig(t *testing.T) {
	cfg := DefaultAzureKeyVaultConfig()

	if !cfg.UseManagedIdentity {
		t.Error("default should use managed identity")
	}

	if cfg.CacheTTL != 5*60*1000000000 {
		t.Errorf("default cache TTL = %v, want 5m", cfg.CacheTTL)
	}
}
