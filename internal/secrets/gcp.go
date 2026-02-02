// Package secrets provides Google Cloud Secret Manager integration for Chronos.
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCPSecretManagerConfig configures the GCP Secret Manager provider.
type GCPSecretManagerConfig struct {
	// ProjectID is the GCP project ID.
	ProjectID string `json:"project_id" yaml:"project_id"`

	// CredentialsFile is the path to a service account key file.
	// If empty, uses Application Default Credentials.
	CredentialsFile string `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`

	// CredentialsJSON is the raw service account key JSON.
	// Takes precedence over CredentialsFile if both are set.
	CredentialsJSON string `json:"credentials_json,omitempty" yaml:"credentials_json,omitempty"`

	// CacheTTL is how long to cache secrets (0 = no caching).
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Prefix is a prefix to prepend to all secret names.
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`

	// DefaultVersion is the default secret version to use (default: "latest").
	DefaultVersion string `json:"default_version,omitempty" yaml:"default_version,omitempty"`
}

// DefaultGCPSecretManagerConfig returns default configuration.
func DefaultGCPSecretManagerConfig() GCPSecretManagerConfig {
	return GCPSecretManagerConfig{
		CacheTTL:       5 * time.Minute,
		DefaultVersion: "latest",
	}
}

// GCPSecretManagerProvider provides secrets from Google Cloud Secret Manager.
type GCPSecretManagerProvider struct {
	client  *secretmanager.Client
	config  GCPSecretManagerConfig
	cache   map[string]*cachedSecret
	cacheMu sync.RWMutex
}

// NewGCPSecretManagerProvider creates a new GCP Secret Manager provider.
func NewGCPSecretManagerProvider(ctx context.Context, cfg GCPSecretManagerConfig) (*GCPSecretManagerProvider, error) {
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	if cfg.DefaultVersion == "" {
		cfg.DefaultVersion = "latest"
	}

	// Build client options
	var opts []option.ClientOption

	if cfg.CredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(cfg.CredentialsJSON)))
	} else if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithCredentialsFile(cfg.CredentialsFile))
	}
	// Otherwise uses Application Default Credentials

	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	return &GCPSecretManagerProvider{
		client: client,
		config: cfg,
		cache:  make(map[string]*cachedSecret),
	}, nil
}

// Name returns the provider name.
func (p *GCPSecretManagerProvider) Name() string {
	return "gcp"
}

// GetSecret retrieves a secret from GCP Secret Manager.
func (p *GCPSecretManagerProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Parse key format: secret-name, secret-name@version, or secret-name#field
	name, version, field := p.parseSecretKey(key)

	// Apply prefix
	if p.config.Prefix != "" {
		name = p.config.Prefix + name
	}

	cacheKey := fmt.Sprintf("%s@%s", name, version)

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

	// Build resource name
	resourceName := fmt.Sprintf("projects/%s/secrets/%s/versions/%s",
		p.config.ProjectID, name, version)

	// Access secret version
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	}

	result, err := p.client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %s: %w", name, err)
	}

	value := string(result.Payload.Data)

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

// parseSecretKey parses a secret key in format "name", "name@version", or "name#field" or "name@version#field".
func (p *GCPSecretManagerProvider) parseSecretKey(key string) (name, version, field string) {
	version = p.config.DefaultVersion

	// Check for field separator
	if idx := strings.LastIndex(key, "#"); idx != -1 {
		field = key[idx+1:]
		key = key[:idx]
	}

	// Check for version separator
	if idx := strings.LastIndex(key, "@"); idx != -1 {
		version = key[idx+1:]
		key = key[:idx]
	}

	return key, version, field
}

// extractField extracts a field from a JSON secret value.
func (p *GCPSecretManagerProvider) extractField(value, field string) (string, error) {
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
func (p *GCPSecretManagerProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := prefix
	if p.config.Prefix != "" {
		fullPrefix = p.config.Prefix + prefix
	}

	parent := fmt.Sprintf("projects/%s", p.config.ProjectID)

	req := &secretmanagerpb.ListSecretsRequest{
		Parent: parent,
	}

	// Add filter if prefix is specified
	if fullPrefix != "" {
		req.Filter = fmt.Sprintf("name:%s", fullPrefix)
	}

	var secrets []string
	it := p.client.ListSecrets(ctx, req)

	for {
		secret, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		// Extract secret name from resource name
		// Format: projects/{project}/secrets/{name}
		parts := strings.Split(secret.Name, "/")
		name := parts[len(parts)-1]

		// Remove provider prefix before returning
		if p.config.Prefix != "" {
			name = strings.TrimPrefix(name, p.config.Prefix)
		}

		secrets = append(secrets, name)
	}

	return secrets, nil
}

// Close cleans up resources.
func (p *GCPSecretManagerProvider) Close(ctx context.Context) error {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()

	return p.client.Close()
}

// ClearCache clears the secret cache.
func (p *GCPSecretManagerProvider) ClearCache() {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()
}

// Ensure GCPSecretManagerProvider implements Provider interface.
var _ Provider = (*GCPSecretManagerProvider)(nil)
