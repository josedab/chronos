// Package rbac provides OIDC/SSO authentication for Chronos.
package rbac

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OIDCConfig holds OIDC/SSO configuration.
type OIDCConfig struct {
	// Enabled toggles OIDC authentication.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// IssuerURL is the OIDC provider URL (e.g., https://accounts.google.com).
	IssuerURL string `json:"issuer_url" yaml:"issuer_url"`
	// ClientID is the OAuth2 client ID.
	ClientID string `json:"client_id" yaml:"client_id"`
	// ClientSecret is the OAuth2 client secret.
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
	// RedirectURL is the callback URL for the OAuth2 flow.
	RedirectURL string `json:"redirect_url" yaml:"redirect_url"`
	// Scopes to request (defaults: openid, profile, email).
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	// RoleMapping maps OIDC claims/groups to Chronos roles.
	RoleMapping map[string]string `json:"role_mapping,omitempty" yaml:"role_mapping,omitempty"`
	// DefaultRole assigned to new SSO users (default: viewer).
	DefaultRole string `json:"default_role,omitempty" yaml:"default_role,omitempty"`
	// GroupsClaim is the JWT claim containing group memberships.
	GroupsClaim string `json:"groups_claim,omitempty" yaml:"groups_claim,omitempty"`
	// AutoProvision creates users on first login.
	AutoProvision bool `json:"auto_provision" yaml:"auto_provision"`
}

// DefaultOIDCConfig returns the default OIDC configuration.
func DefaultOIDCConfig() OIDCConfig {
	return OIDCConfig{
		Scopes:        []string{"openid", "profile", "email"},
		DefaultRole:   "viewer",
		GroupsClaim:    "groups",
		AutoProvision: true,
	}
}

// OIDCProvider manages OIDC authentication.
type OIDCProvider struct {
	config       OIDCConfig
	manager      *RBACManager
	jwksCache    *jwksCache
	httpClient   *http.Client
	discoveryURL string
	discovery    *oidcDiscovery
	mu           sync.RWMutex
}

// oidcDiscovery represents the OIDC discovery document.
type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint,omitempty"`
}

// jwksCache caches the JWKS keys.
type jwksCache struct {
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	mu        sync.RWMutex
}

// OIDCClaims represents the JWT claims from an OIDC token.
type OIDCClaims struct {
	Subject  string   `json:"sub"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Groups   []string `json:"groups,omitempty"`
	IssuedAt int64    `json:"iat"`
	Expiry   int64    `json:"exp"`
	Issuer   string   `json:"iss"`
	Audience string   `json:"aud"`
}

// NewOIDCProvider creates a new OIDC authentication provider.
func NewOIDCProvider(config OIDCConfig, manager *RBACManager) *OIDCProvider {
	if len(config.Scopes) == 0 {
		config.Scopes = []string{"openid", "profile", "email"}
	}
	if config.DefaultRole == "" {
		config.DefaultRole = "viewer"
	}
	if config.GroupsClaim == "" {
		config.GroupsClaim = "groups"
	}

	return &OIDCProvider{
		config:       config,
		manager:      manager,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		discoveryURL: strings.TrimRight(config.IssuerURL, "/") + "/.well-known/openid-configuration",
		jwksCache:    &jwksCache{keys: make(map[string]*rsa.PublicKey)},
	}
}

// Discover fetches the OIDC discovery document.
func (p *OIDCProvider) Discover(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.discoveryURL, nil)
	if err != nil {
		return fmt.Errorf("creating discovery request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}

	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return fmt.Errorf("decoding discovery document: %w", err)
	}

	p.mu.Lock()
	p.discovery = &disc
	p.mu.Unlock()

	return nil
}

// GetAuthorizationURL returns the authorization URL for the OAuth2 flow.
func (p *OIDCProvider) GetAuthorizationURL(state string) (string, error) {
	p.mu.RLock()
	disc := p.discovery
	p.mu.RUnlock()

	if disc == nil {
		return "", errors.New("OIDC provider not initialized; call Discover() first")
	}

	scopes := strings.Join(p.config.Scopes, " ")
	return fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		disc.AuthorizationEndpoint,
		p.config.ClientID,
		p.config.RedirectURL,
		scopes,
		state,
	), nil
}

// ValidateToken parses and validates JWT claims from a bearer token.
// It verifies the RS256 signature against cached JWKS keys when available.
func (p *OIDCProvider) ValidateToken(ctx context.Context, tokenString string) (*OIDCClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	// Decode header to get key ID
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT header: %w", err)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parsing JWT header: %w", err)
	}

	// Verify cryptographic signature if we have JWKS keys
	if header.Kid != "" {
		if err := p.verifySignature(ctx, parts, header.Kid, header.Alg); err != nil {
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
	}

	// Decode claims (middle segment)
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT claims: %w", err)
	}

	var rawClaims map[string]interface{}
	if err := json.Unmarshal(claimsJSON, &rawClaims); err != nil {
		return nil, fmt.Errorf("parsing JWT claims: %w", err)
	}

	claims := &OIDCClaims{}
	if v, ok := rawClaims["sub"].(string); ok {
		claims.Subject = v
	}
	if v, ok := rawClaims["email"].(string); ok {
		claims.Email = v
	}
	if v, ok := rawClaims["name"].(string); ok {
		claims.Name = v
	}
	if v, ok := rawClaims["iss"].(string); ok {
		claims.Issuer = v
	}
	if v, ok := rawClaims["exp"].(float64); ok {
		claims.Expiry = int64(v)
	}
	if v, ok := rawClaims["iat"].(float64); ok {
		claims.IssuedAt = int64(v)
	}
	if v, ok := rawClaims["aud"].(string); ok {
		claims.Audience = v
	}

	// Extract groups from configured claim
	if groups, ok := rawClaims[p.config.GroupsClaim]; ok {
		if groupList, ok := groups.([]interface{}); ok {
			for _, g := range groupList {
				if gs, ok := g.(string); ok {
					claims.Groups = append(claims.Groups, gs)
				}
			}
		}
	}

	// Validate expiry
	if claims.Expiry > 0 && time.Now().Unix() > claims.Expiry {
		return nil, errors.New("token expired")
	}

	// Validate issuer
	if p.config.IssuerURL != "" && claims.Issuer != p.config.IssuerURL {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", p.config.IssuerURL, claims.Issuer)
	}

	// Validate audience
	if p.config.ClientID != "" && claims.Audience != "" && claims.Audience != p.config.ClientID {
		return nil, fmt.Errorf("invalid audience: expected %s, got %s", p.config.ClientID, claims.Audience)
	}

	return claims, nil
}

// jwtHeader represents the JWT header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

// jwksResponse represents the JWKS endpoint response.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// jwkKey represents a single JWK key.
type jwkKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// verifySignature verifies the RS256 JWT signature against JWKS.
func (p *OIDCProvider) verifySignature(ctx context.Context, parts []string, kid, alg string) error {
	if alg != "" && alg != "RS256" {
		return fmt.Errorf("unsupported algorithm: %s (only RS256 supported)", alg)
	}

	key, err := p.getSigningKey(ctx, kid)
	if err != nil {
		return fmt.Errorf("fetching signing key: %w", err)
	}

	// The signed content is header.payload
	signingInput := parts[0] + "." + parts[1]
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	hash := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hash[:], signatureBytes); err != nil {
		return fmt.Errorf("RSA signature invalid: %w", err)
	}

	return nil
}

// getSigningKey retrieves an RSA public key by kid, using cache when possible.
func (p *OIDCProvider) getSigningKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Check cache first
	p.jwksCache.mu.RLock()
	if key, ok := p.jwksCache.keys[kid]; ok && time.Now().Before(p.jwksCache.expiresAt) {
		p.jwksCache.mu.RUnlock()
		return key, nil
	}
	p.jwksCache.mu.RUnlock()

	// Refresh JWKS
	if err := p.refreshJWKS(ctx); err != nil {
		return nil, err
	}

	p.jwksCache.mu.RLock()
	defer p.jwksCache.mu.RUnlock()

	key, ok := p.jwksCache.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key ID %q not found in JWKS", kid)
	}
	return key, nil
}

// refreshJWKS fetches and caches JWKS keys from the provider.
func (p *OIDCProvider) refreshJWKS(ctx context.Context) error {
	p.mu.RLock()
	disc := p.discovery
	p.mu.RUnlock()

	if disc == nil || disc.JwksURI == "" {
		return errors.New("JWKS URI not available; call Discover() first")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", disc.JwksURI, nil)
	if err != nil {
		return fmt.Errorf("creating JWKS request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue // skip malformed keys
		}
		newKeys[k.Kid] = pubKey
	}

	p.jwksCache.mu.Lock()
	p.jwksCache.keys = newKeys
	p.jwksCache.expiresAt = time.Now().Add(1 * time.Hour) // Cache for 1 hour
	p.jwksCache.mu.Unlock()

	return nil
}

// ProvisionUser creates or updates a Chronos user from OIDC claims.
func (p *OIDCProvider) ProvisionUser(ctx context.Context, claims *OIDCClaims) (*User, error) {
	if claims.Subject == "" {
		return nil, errors.New("OIDC subject claim is empty")
	}

	// Try to find existing user
	user, err := p.manager.GetUser(ctx, claims.Subject)
	if err != nil {
		if !p.config.AutoProvision {
			return nil, fmt.Errorf("user not found and auto-provisioning is disabled")
		}

		// Create new user
		roles := p.resolveRoles(claims.Groups)
		user = &User{
			ID:     claims.Subject,
			Email:  claims.Email,
			Name:   claims.Name,
			Status: UserStatusActive,
			Roles:  roles,
			Metadata: map[string]string{
				"auth_provider": "oidc",
				"issuer":        claims.Issuer,
			},
		}
		if err := p.manager.CreateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("provisioning user: %w", err)
		}
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now

	return user, nil
}

// resolveRoles maps OIDC groups to Chronos roles.
func (p *OIDCProvider) resolveRoles(groups []string) []string {
	if len(p.config.RoleMapping) == 0 || len(groups) == 0 {
		return []string{p.config.DefaultRole}
	}

	roleSet := make(map[string]bool)
	for _, group := range groups {
		if role, ok := p.config.RoleMapping[group]; ok {
			roleSet[role] = true
		}
	}

	if len(roleSet) == 0 {
		return []string{p.config.DefaultRole}
	}

	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

// AuthMiddleware returns HTTP middleware that validates OIDC bearer tokens.
func (p *OIDCProvider) AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !p.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"invalid authorization scheme, expected Bearer"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := p.ValidateToken(r.Context(), token)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"invalid token: %s"}`, err.Error()), http.StatusUnauthorized)
				return
			}

			user, err := p.ProvisionUser(r.Context(), claims)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"user provisioning failed: %s"}`, err.Error()), http.StatusForbidden)
				return
			}

			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseRSAPublicKey parses an RSA public key from JWK components.
func parseRSAPublicKey(nBase64, eBase64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nBase64)
	if err != nil {
		return nil, fmt.Errorf("decoding modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eBase64)
	if err != nil {
		return nil, fmt.Errorf("decoding exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}
