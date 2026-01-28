package secrets

import (
	"context"
	"os"
	"testing"
)

func TestEnvProvider(t *testing.T) {
	// Set up test environment variable
	os.Setenv("TEST_SECRET_KEY", "test-value")
	defer os.Unsetenv("TEST_SECRET_KEY")

	provider := NewEnvProvider("")

	t.Run("Name", func(t *testing.T) {
		if provider.Name() != "env" {
			t.Errorf("expected name 'env', got %q", provider.Name())
		}
	})

	t.Run("GetSecret", func(t *testing.T) {
		value, err := provider.GetSecret(context.Background(), "TEST_SECRET_KEY")
		if err != nil {
			t.Fatalf("GetSecret failed: %v", err)
		}
		if value != "test-value" {
			t.Errorf("expected 'test-value', got %q", value)
		}
	})

	t.Run("GetSecretNotFound", func(t *testing.T) {
		_, err := provider.GetSecret(context.Background(), "NON_EXISTENT_KEY")
		if err == nil {
			t.Error("expected error for non-existent key")
		}
	})

	t.Run("GetSecretWithPrefix", func(t *testing.T) {
		os.Setenv("CHRONOS_DB_PASSWORD", "secret123")
		defer os.Unsetenv("CHRONOS_DB_PASSWORD")

		prefixProvider := NewEnvProvider("CHRONOS_")
		value, err := prefixProvider.GetSecret(context.Background(), "DB_PASSWORD")
		if err != nil {
			t.Fatalf("GetSecret failed: %v", err)
		}
		if value != "secret123" {
			t.Errorf("expected 'secret123', got %q", value)
		}
	})

	t.Run("ListSecrets", func(t *testing.T) {
		keys, err := provider.ListSecrets(context.Background(), "TEST_SECRET")
		if err != nil {
			t.Fatalf("ListSecrets failed: %v", err)
		}
		found := false
		for _, k := range keys {
			if k == "TEST_SECRET_KEY" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find TEST_SECRET_KEY in list")
		}
	})
}

func TestFileProvider(t *testing.T) {
	// Create temp directory and secret file
	tmpDir, err := os.MkdirTemp("", "chronos-secrets-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	secretFile := tmpDir + "/api-key"
	if err := os.WriteFile(secretFile, []byte("my-api-key\n"), 0600); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	provider := NewFileProvider(tmpDir)

	t.Run("Name", func(t *testing.T) {
		if provider.Name() != "file" {
			t.Errorf("expected name 'file', got %q", provider.Name())
		}
	})

	t.Run("GetSecret", func(t *testing.T) {
		value, err := provider.GetSecret(context.Background(), "api-key")
		if err != nil {
			t.Fatalf("GetSecret failed: %v", err)
		}
		if value != "my-api-key" {
			t.Errorf("expected 'my-api-key', got %q", value)
		}
	})

	t.Run("GetSecretNotFound", func(t *testing.T) {
		_, err := provider.GetSecret(context.Background(), "non-existent")
		if err == nil {
			t.Error("expected error for non-existent secret")
		}
	})

	t.Run("ListSecrets", func(t *testing.T) {
		keys, err := provider.ListSecrets(context.Background(), "api")
		if err != nil {
			t.Fatalf("ListSecrets failed: %v", err)
		}
		if len(keys) != 1 || keys[0] != "api-key" {
			t.Errorf("expected ['api-key'], got %v", keys)
		}
	})
}

func TestManager(t *testing.T) {
	manager := NewManager()

	// Set up test environment
	os.Setenv("TEST_DB_URL", "postgres://localhost:5432/test")
	os.Setenv("TEST_API_KEY", "key123")
	defer os.Unsetenv("TEST_DB_URL")
	defer os.Unsetenv("TEST_API_KEY")

	// Register env provider
	envProvider := NewEnvProvider("")
	if err := manager.RegisterProvider(envProvider); err != nil {
		t.Fatalf("RegisterProvider failed: %v", err)
	}

	t.Run("RegisterDuplicate", func(t *testing.T) {
		err := manager.RegisterProvider(envProvider)
		if err == nil {
			t.Error("expected error for duplicate provider")
		}
	})

	t.Run("GetProvider", func(t *testing.T) {
		provider, err := manager.GetProvider("env")
		if err != nil {
			t.Fatalf("GetProvider failed: %v", err)
		}
		if provider.Name() != "env" {
			t.Errorf("expected 'env', got %q", provider.Name())
		}
	})

	t.Run("GetProviderNotFound", func(t *testing.T) {
		_, err := manager.GetProvider("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent provider")
		}
	})

	t.Run("ResolveSecret", func(t *testing.T) {
		value, err := manager.ResolveSecret(context.Background(), SecretRef{
			Provider: "env",
			Key:      "TEST_DB_URL",
		})
		if err != nil {
			t.Fatalf("ResolveSecret failed: %v", err)
		}
		if value != "postgres://localhost:5432/test" {
			t.Errorf("unexpected value: %q", value)
		}
	})

	t.Run("InjectSecrets", func(t *testing.T) {
		input := "Connect to ${secret:env:TEST_DB_URL} with key ${secret:env:TEST_API_KEY}"
		result, err := manager.InjectSecrets(context.Background(), input)
		if err != nil {
			t.Fatalf("InjectSecrets failed: %v", err)
		}
		expected := "Connect to postgres://localhost:5432/test with key key123"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("InjectSecretsInMap", func(t *testing.T) {
		input := map[string]string{
			"database_url": "${secret:env:TEST_DB_URL}",
			"api_key":      "${secret:env:TEST_API_KEY}",
			"static":       "no-secrets-here",
		}
		result, err := manager.InjectSecretsInMap(context.Background(), input)
		if err != nil {
			t.Fatalf("InjectSecretsInMap failed: %v", err)
		}
		if result["database_url"] != "postgres://localhost:5432/test" {
			t.Errorf("unexpected database_url: %q", result["database_url"])
		}
		if result["api_key"] != "key123" {
			t.Errorf("unexpected api_key: %q", result["api_key"])
		}
		if result["static"] != "no-secrets-here" {
			t.Errorf("unexpected static: %q", result["static"])
		}
	})
}
