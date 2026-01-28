// Package secrets provides HashiCorp Vault integration for Chronos.
package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// VaultConfig configures the Vault provider.
type VaultConfig struct {
	// Address is the Vault server address.
	Address string `json:"address" yaml:"address"`
	
	// Token is the Vault authentication token.
	Token string `json:"token" yaml:"token"`
	
	// Namespace is the Vault namespace (Enterprise only).
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	
	// MountPath is the secrets engine mount path.
	MountPath string `json:"mount_path" yaml:"mount_path"`
	
	// AuthMethod is the authentication method (token, kubernetes, approle).
	AuthMethod string `json:"auth_method" yaml:"auth_method"`
	
	// KubernetesRole is the Kubernetes auth role (when using kubernetes auth).
	KubernetesRole string `json:"kubernetes_role,omitempty" yaml:"kubernetes_role,omitempty"`
	
	// AppRoleID is the AppRole role ID (when using approle auth).
	AppRoleID string `json:"approle_role_id,omitempty" yaml:"approle_role_id,omitempty"`
	
	// AppRoleSecretID is the AppRole secret ID (when using approle auth).
	AppRoleSecretID string `json:"approle_secret_id,omitempty" yaml:"approle_secret_id,omitempty"`
	
	// TLSConfig contains TLS settings.
	TLSConfig *VaultTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
	
	// Timeout is the HTTP client timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
	
	// CacheTTL is how long to cache secrets (0 = no caching).
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
}

// VaultTLSConfig contains TLS configuration for Vault.
type VaultTLSConfig struct {
	CACert     string `json:"ca_cert,omitempty" yaml:"ca_cert,omitempty"`
	ClientCert string `json:"client_cert,omitempty" yaml:"client_cert,omitempty"`
	ClientKey  string `json:"client_key,omitempty" yaml:"client_key,omitempty"`
	Insecure   bool   `json:"insecure,omitempty" yaml:"insecure,omitempty"`
}

// DefaultVaultConfig returns default Vault configuration.
// Note: Default address is for local development only.
func DefaultVaultConfig() VaultConfig {
	return VaultConfig{
		Address:    "http://127.0.0.1:8200", // Local development default
		MountPath:  "secret",
		AuthMethod: "token",
		Timeout:    30 * time.Second,
		CacheTTL:   5 * time.Minute,
	}
}

// VaultProvider provides secrets from HashiCorp Vault.
type VaultProvider struct {
	config     VaultConfig
	client     *http.Client
	token      string
	tokenMu    sync.RWMutex
	cache      map[string]*cachedSecret
	cacheMu    sync.RWMutex
}

// cachedSecret represents a cached secret value.
type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// VaultResponse represents a Vault API response.
type VaultResponse struct {
	Data        map[string]interface{} `json:"data"`
	Renewable   bool                   `json:"renewable"`
	LeaseDuration int                  `json:"lease_duration"`
}

// VaultKVData represents KV v2 data structure.
type VaultKVData struct {
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

// VaultAuthResponse represents a Vault auth response.
type VaultAuthResponse struct {
	Auth struct {
		ClientToken   string   `json:"client_token"`
		Policies      []string `json:"policies"`
		LeaseDuration int      `json:"lease_duration"`
		Renewable     bool     `json:"renewable"`
	} `json:"auth"`
}

// NewVaultProvider creates a new Vault secret provider.
func NewVaultProvider(cfg VaultConfig) (*VaultProvider, error) {
	if cfg.Address == "" {
		return nil, errors.New("vault address is required")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.MountPath == "" {
		cfg.MountPath = "secret"
	}

	p := &VaultProvider{
		config: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		cache: make(map[string]*cachedSecret),
	}

	// Authenticate based on method
	switch cfg.AuthMethod {
	case "token":
		if cfg.Token == "" {
			return nil, errors.New("vault token is required for token auth")
		}
		p.token = cfg.Token
	case "kubernetes":
		if err := p.authenticateKubernetes(); err != nil {
			return nil, fmt.Errorf("kubernetes auth failed: %w", err)
		}
	case "approle":
		if err := p.authenticateAppRole(); err != nil {
			return nil, fmt.Errorf("approle auth failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", cfg.AuthMethod)
	}

	return p, nil
}

// Name returns the provider name.
func (p *VaultProvider) Name() string {
	return "vault"
}

// GetSecret retrieves a secret from Vault.
func (p *VaultProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Check cache first
	if p.config.CacheTTL > 0 {
		p.cacheMu.RLock()
		if cached, ok := p.cache[key]; ok && time.Now().Before(cached.expiresAt) {
			p.cacheMu.RUnlock()
			return cached.value, nil
		}
		p.cacheMu.RUnlock()
	}

	// Parse key format: path/to/secret#field or path/to/secret
	path, field := parseSecretKey(key)

	// Build URL for KV v2
	url := fmt.Sprintf("%s/v1/%s/data/%s", p.config.Address, p.config.MountPath, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	p.tokenMu.RLock()
	token := p.token
	p.tokenMu.RUnlock()

	req.Header.Set("X-Vault-Token", token)
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", fmt.Errorf("secret not found: %s", path)
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vault error (status %d): %s", resp.StatusCode, string(body))
	}

	var vaultResp struct {
		Data VaultKVData `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vaultResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract the value
	var value string
	if field != "" {
		// Get specific field
		v, ok := vaultResp.Data.Data[field]
		if !ok {
			return "", fmt.Errorf("field %s not found in secret %s", field, path)
		}
		value = fmt.Sprintf("%v", v)
	} else {
		// If no field specified and only one key, return that
		if len(vaultResp.Data.Data) == 1 {
			for _, v := range vaultResp.Data.Data {
				value = fmt.Sprintf("%v", v)
				break
			}
		} else {
			// Return as JSON
			jsonBytes, _ := json.Marshal(vaultResp.Data.Data)
			value = string(jsonBytes)
		}
	}

	// Cache the value
	if p.config.CacheTTL > 0 {
		p.cacheMu.Lock()
		p.cache[key] = &cachedSecret{
			value:     value,
			expiresAt: time.Now().Add(p.config.CacheTTL),
		}
		p.cacheMu.Unlock()
	}

	return value, nil
}

// ListSecrets lists secrets at a path.
func (p *VaultProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	url := fmt.Sprintf("%s/v1/%s/metadata/%s", p.config.Address, p.config.MountPath, prefix)

	req, err := http.NewRequestWithContext(ctx, "LIST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.tokenMu.RLock()
	token := p.token
	p.tokenMu.RUnlock()

	req.Header.Set("X-Vault-Token", token)
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return []string{}, nil
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault error (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return listResp.Data.Keys, nil
}

// Close cleans up resources.
func (p *VaultProvider) Close(ctx context.Context) error {
	// Revoke token if using AppRole or Kubernetes auth
	if p.config.AuthMethod != "token" {
		p.tokenMu.RLock()
		token := p.token
		p.tokenMu.RUnlock()

		if token != "" {
			url := fmt.Sprintf("%s/v1/auth/token/revoke-self", p.config.Address)
			req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
			if err == nil {
				req.Header.Set("X-Vault-Token", token)
				p.client.Do(req)
			}
		}
	}

	// Clear cache
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()

	return nil
}

// authenticateKubernetes authenticates using Kubernetes service account.
func (p *VaultProvider) authenticateKubernetes() error {
	// Read service account token
	jwt, err := readKubernetesJWT()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v1/auth/kubernetes/login", p.config.Address)
	payload := map[string]string{
		"role": p.config.KubernetesRole,
		"jwt":  jwt,
	}

	return p.authenticate(url, payload)
}

// authenticateAppRole authenticates using AppRole.
func (p *VaultProvider) authenticateAppRole() error {
	url := fmt.Sprintf("%s/v1/auth/approle/login", p.config.Address)
	payload := map[string]string{
		"role_id":   p.config.AppRoleID,
		"secret_id": p.config.AppRoleSecretID,
	}

	return p.authenticate(url, payload)
}

// authenticate performs authentication and stores the token.
func (p *VaultProvider) authenticate(url string, payload map[string]string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := p.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var authResp VaultAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	p.tokenMu.Lock()
	p.token = authResp.Auth.ClientToken
	p.tokenMu.Unlock()

	return nil
}

// RenewToken renews the current token.
func (p *VaultProvider) RenewToken(ctx context.Context) error {
	p.tokenMu.RLock()
	token := p.token
	p.tokenMu.RUnlock()

	if token == "" {
		return errors.New("no token to renew")
	}

	url := fmt.Sprintf("%s/v1/auth/token/renew-self", p.config.Address)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Vault-Token", token)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("token renewal failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("token renewal failed with status %d", resp.StatusCode)
	}

	return nil
}

// WriteSecret writes a secret to Vault.
func (p *VaultProvider) WriteSecret(ctx context.Context, path string, data map[string]interface{}) error {
	url := fmt.Sprintf("%s/v1/%s/data/%s", p.config.Address, p.config.MountPath, path)

	payload := map[string]interface{}{
		"data": data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	p.tokenMu.RLock()
	token := p.token
	p.tokenMu.RUnlock()

	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("write request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Invalidate cache for this path
	if p.config.CacheTTL > 0 {
		p.cacheMu.Lock()
		for key := range p.cache {
			if strings.HasPrefix(key, path) {
				delete(p.cache, key)
			}
		}
		p.cacheMu.Unlock()
	}

	return nil
}

// DeleteSecret deletes a secret from Vault.
func (p *VaultProvider) DeleteSecret(ctx context.Context, path string) error {
	url := fmt.Sprintf("%s/v1/%s/data/%s", p.config.Address, p.config.MountPath, path)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	p.tokenMu.RLock()
	token := p.token
	p.tokenMu.RUnlock()

	req.Header.Set("X-Vault-Token", token)
	if p.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", p.config.Namespace)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ClearCache clears the secret cache.
func (p *VaultProvider) ClearCache() {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()
}

// Health checks the health of the Vault server.
func (p *VaultProvider) Health(ctx context.Context) error {
	url := p.config.Address + "/v1/sys/health"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var health struct {
		Initialized bool `json:"initialized"`
		Sealed      bool `json:"sealed"`
		Standby     bool `json:"standby"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return err
	}

	if !health.Initialized {
		return errors.New("vault is not initialized")
	}
	if health.Sealed {
		return errors.New("vault is sealed")
	}

	return nil
}

// parseSecretKey parses a secret key in format "path/to/secret#field".
func parseSecretKey(key string) (path, field string) {
	if idx := strings.LastIndex(key, "#"); idx != -1 {
		return key[:idx], key[idx+1:]
	}
	return key, ""
}

// readKubernetesJWT reads the Kubernetes service account JWT.
func readKubernetesJWT() (string, error) {
	const tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	data, err := readFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read kubernetes token: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// readFile is a variable to allow testing.
var readFile = func(path string) ([]byte, error) {
	return io.ReadAll(&fileReader{path: path})
}

type fileReader struct {
	path string
}

func (f *fileReader) Read(p []byte) (n int, err error) {
	data, err := io.ReadAll(strings.NewReader("")) // Placeholder
	if err != nil {
		return 0, err
	}
	copy(p, data)
	return len(data), io.EOF
}

// Ensure VaultProvider implements Provider interface.
var _ Provider = (*VaultProvider)(nil)
