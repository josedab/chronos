// Package secrets provides Azure Key Vault integration for Chronos.
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// AzureKeyVaultConfig configures the Azure Key Vault provider.
type AzureKeyVaultConfig struct {
	// VaultURL is the Azure Key Vault URL (e.g., https://myvault.vault.azure.net/).
	VaultURL string `json:"vault_url" yaml:"vault_url"`

	// TenantID is the Azure tenant ID (optional for managed identity).
	TenantID string `json:"tenant_id,omitempty" yaml:"tenant_id,omitempty"`

	// ClientID is the Azure client ID (optional for managed identity).
	ClientID string `json:"client_id,omitempty" yaml:"client_id,omitempty"`

	// ClientSecret is the Azure client secret (for service principal auth).
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`

	// UseManagedIdentity uses Azure managed identity for authentication.
	UseManagedIdentity bool `json:"use_managed_identity" yaml:"use_managed_identity"`

	// CacheTTL is how long to cache secrets (0 = no caching).
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Prefix is a prefix to prepend to all secret names.
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
}

// DefaultAzureKeyVaultConfig returns default configuration.
func DefaultAzureKeyVaultConfig() AzureKeyVaultConfig {
	return AzureKeyVaultConfig{
		CacheTTL:           5 * time.Minute,
		UseManagedIdentity: true,
	}
}

// AzureKeyVaultProvider provides secrets from Azure Key Vault.
type AzureKeyVaultProvider struct {
	client  *azsecrets.Client
	config  AzureKeyVaultConfig
	cache   map[string]*cachedSecret
	cacheMu sync.RWMutex
}

// NewAzureKeyVaultProvider creates a new Azure Key Vault provider.
func NewAzureKeyVaultProvider(ctx context.Context, cfg AzureKeyVaultConfig) (*AzureKeyVaultProvider, error) {
	if cfg.VaultURL == "" {
		return nil, fmt.Errorf("vault_url is required")
	}

	var cred *azidentity.DefaultAzureCredential
	var err error

	// Determine authentication method
	if cfg.ClientID != "" && cfg.ClientSecret != "" && cfg.TenantID != "" {
		// Service principal authentication
		spCred, spErr := azidentity.NewClientSecretCredential(
			cfg.TenantID,
			cfg.ClientID,
			cfg.ClientSecret,
			nil,
		)
		if spErr != nil {
			return nil, fmt.Errorf("failed to create Azure credential: %w", spErr)
		}
		client, err := azsecrets.NewClient(cfg.VaultURL, spCred, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
		}
		return &AzureKeyVaultProvider{
			client: client,
			config: cfg,
			cache:  make(map[string]*cachedSecret),
		}, nil
	} else if cfg.UseManagedIdentity {
		// Managed identity authentication
		opts := &azidentity.ManagedIdentityCredentialOptions{}
		if cfg.ClientID != "" {
			opts.ID = azidentity.ClientID(cfg.ClientID)
		}
		miCred, miErr := azidentity.NewManagedIdentityCredential(opts)
		if miErr != nil {
			return nil, fmt.Errorf("failed to create Azure managed identity credential: %w", miErr)
		}
		client, err := azsecrets.NewClient(cfg.VaultURL, miCred, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
		}
		return &AzureKeyVaultProvider{
			client: client,
			config: cfg,
			cache:  make(map[string]*cachedSecret),
		}, nil
	}
	
	// Default credential chain (tries multiple methods)
	cred, err = azidentity.NewDefaultAzureCredential(nil)

	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	client, err := azsecrets.NewClient(cfg.VaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
	}

	return &AzureKeyVaultProvider{
		client: client,
		config: cfg,
		cache:  make(map[string]*cachedSecret),
	}, nil
}

// Name returns the provider name.
func (p *AzureKeyVaultProvider) Name() string {
	return "azure"
}

// GetSecret retrieves a secret from Azure Key Vault.
func (p *AzureKeyVaultProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Parse key format: secret-name, secret-name/version, or secret-name#field
	name, version, field := p.parseSecretKey(key)

	// Apply prefix (convert dashes as Azure doesn't allow certain characters)
	if p.config.Prefix != "" {
		name = p.config.Prefix + name
	}

	cacheKey := name
	if version != "" {
		cacheKey = fmt.Sprintf("%s/%s", name, version)
	}

	// Check cache first
	if p.config.CacheTTL > 0 {
		p.cacheMu.RLock()
		if cached, ok := p.cache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
			p.cacheMu.RUnlock()
			if field != "" {
				return p.extractField(cached.value, field)
			}
			return cached.value, nil
		}
		p.cacheMu.RUnlock()
	}

	// Get secret from Azure Key Vault
	resp, err := p.client.GetSecret(ctx, name, version, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", name, err)
	}

	value := ""
	if resp.Value != nil {
		value = *resp.Value
	}

	// Cache the raw value
	if p.config.CacheTTL > 0 {
		p.cacheMu.Lock()
		p.cache[cacheKey] = &cachedSecret{
			value:     value,
			expiresAt: time.Now().Add(p.config.CacheTTL),
		}
		p.cacheMu.Unlock()
	}

	// Extract field if specified
	if field != "" {
		return p.extractField(value, field)
	}

	return value, nil
}

// parseSecretKey parses a secret key in format "name", "name/version", or "name#field".
func (p *AzureKeyVaultProvider) parseSecretKey(key string) (name, version, field string) {
	// Check for field separator
	if idx := strings.LastIndex(key, "#"); idx != -1 {
		field = key[idx+1:]
		key = key[:idx]
	}

	// Check for version separator (Azure uses / for versions)
	if idx := strings.LastIndex(key, "/"); idx != -1 {
		version = key[idx+1:]
		key = key[:idx]
	}

	return key, version, field
}

// extractField extracts a field from a JSON secret value.
func (p *AzureKeyVaultProvider) extractField(value, field string) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return "", fmt.Errorf("failed to parse secret as JSON: %w", err)
	}

	fieldValue, ok := data[field]
	if !ok {
		return "", fmt.Errorf("field %s not found in secret", field)
	}

	return fmt.Sprintf("%v", fieldValue), nil
}

// ListSecrets lists secrets with the given prefix.
func (p *AzureKeyVaultProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := prefix
	if p.config.Prefix != "" {
		fullPrefix = p.config.Prefix + prefix
	}

	var secrets []string
	pager := p.client.NewListSecretPropertiesPager(nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secret := range page.Value {
			// Extract secret name from ID
			// Format: https://{vault}.vault.azure.net/secrets/{name}
			id := string(*secret.ID)
			parts := strings.Split(id, "/")
			name := parts[len(parts)-1]

			// Filter by prefix
			if fullPrefix != "" && !strings.HasPrefix(name, fullPrefix) {
				continue
			}

			// Remove provider prefix before returning
			if p.config.Prefix != "" {
				name = strings.TrimPrefix(name, p.config.Prefix)
			}

			secrets = append(secrets, name)
		}
	}

	return secrets, nil
}

// Close cleans up resources.
func (p *AzureKeyVaultProvider) Close(ctx context.Context) error {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()
	return nil
}

// ClearCache clears the secret cache.
func (p *AzureKeyVaultProvider) ClearCache() {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()
}

// Ensure AzureKeyVaultProvider implements Provider interface.
var _ Provider = (*AzureKeyVaultProvider)(nil)
