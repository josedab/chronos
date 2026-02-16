package rbac

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testKeyPair holds a generated RSA key pair for testing.
type testKeyPair struct {
	Private *rsa.PrivateKey
	Public  *rsa.PublicKey
	Kid     string
}

func generateTestKeyPair(t *testing.T) *testKeyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}
	return &testKeyPair{
		Private: key,
		Public:  &key.PublicKey,
		Kid:     "test-key-1",
	}
}

// makeSignedJWT creates a JWT signed with a real RSA key.
func makeSignedJWT(t *testing.T, kp *testKeyPair, claims map[string]interface{}) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": kp.Kid}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	hash := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, kp.Private, crypto.SHA256, hash[:])
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return headerB64 + "." + claimsB64 + "." + sigB64
}

// makeUnsignedJWT creates a JWT without a kid (no signature verification).
func makeTestJWT(claims map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature := base64.RawURLEncoding.EncodeToString([]byte("test-signature"))
	return header + "." + payload + "." + signature
}

// startJWKSServer starts a mock OIDC discovery + JWKS server.
func startJWKSServer(t *testing.T, kp *testKeyPair) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// Use the request host to build the jwks_uri dynamically
		scheme := "http"
		disc := map[string]string{
			"issuer":                 fmt.Sprintf("%s://%s", scheme, r.Host),
			"authorization_endpoint": fmt.Sprintf("%s://%s/authorize", scheme, r.Host),
			"token_endpoint":         fmt.Sprintf("%s://%s/token", scheme, r.Host),
			"userinfo_endpoint":      fmt.Sprintf("%s://%s/userinfo", scheme, r.Host),
			"jwks_uri":               fmt.Sprintf("%s://%s/.well-known/jwks.json", scheme, r.Host),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(disc)
	})

	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		nBytes := kp.Public.N.Bytes()
		eBytes := big.NewInt(int64(kp.Public.E)).Bytes()
		jwks := map[string]interface{}{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": kp.Kid,
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(nBytes),
					"e":   base64.RawURLEncoding.EncodeToString(eBytes),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})

	return httptest.NewServer(mux)
}

func TestOIDCProvider_ValidateToken(t *testing.T) {
	provider := NewOIDCProvider(OIDCConfig{
		IssuerURL: "https://accounts.example.com",
		ClientID:  "test-client",
	}, NewRBACManager(nil))

	t.Run("valid token", func(t *testing.T) {
		token := makeTestJWT(map[string]interface{}{
			"sub":    "user-123",
			"email":  "test@example.com",
			"name":   "Test User",
			"iss":    "https://accounts.example.com",
			"exp":    float64(time.Now().Add(1 * time.Hour).Unix()),
			"groups": []interface{}{"engineering", "platform"},
		})

		claims, err := provider.ValidateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Subject != "user-123" {
			t.Errorf("expected subject user-123, got %s", claims.Subject)
		}
		if claims.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", claims.Email)
		}
		if len(claims.Groups) != 2 {
			t.Errorf("expected 2 groups, got %d", len(claims.Groups))
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token := makeTestJWT(map[string]interface{}{
			"sub": "user-123",
			"iss": "https://accounts.example.com",
			"exp": float64(time.Now().Add(-1 * time.Hour).Unix()),
		})

		_, err := provider.ValidateToken(context.Background(), token)
		if err == nil {
			t.Fatal("expected error for expired token")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		token := makeTestJWT(map[string]interface{}{
			"sub": "user-123",
			"iss": "https://wrong-issuer.com",
			"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		_, err := provider.ValidateToken(context.Background(), token)
		if err == nil {
			t.Fatal("expected error for wrong issuer")
		}
	})

	t.Run("invalid JWT format", func(t *testing.T) {
		_, err := provider.ValidateToken(context.Background(), "not-a-jwt")
		if err == nil {
			t.Fatal("expected error for invalid JWT")
		}
	})
}

func TestOIDCProvider_ProvisionUser(t *testing.T) {
	manager := NewRBACManager(nil)
	provider := NewOIDCProvider(OIDCConfig{
		AutoProvision: true,
		DefaultRole:   "viewer",
		RoleMapping: map[string]string{
			"platform-admins": "admin",
			"developers":      "developer",
		},
	}, manager)

	t.Run("auto-provision new user", func(t *testing.T) {
		claims := &OIDCClaims{
			Subject: "new-user-1",
			Email:   "new@example.com",
			Name:    "New User",
			Groups:  []string{"developers"},
		}

		user, err := provider.ProvisionUser(context.Background(), claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != "new-user-1" {
			t.Errorf("expected user ID new-user-1, got %s", user.ID)
		}
		if len(user.Roles) != 1 || user.Roles[0] != "developer" {
			t.Errorf("expected roles [developer], got %v", user.Roles)
		}
	})

	t.Run("existing user login", func(t *testing.T) {
		claims := &OIDCClaims{
			Subject: "new-user-1",
			Email:   "new@example.com",
			Name:    "New User",
		}

		user, err := provider.ProvisionUser(context.Background(), claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.LastLoginAt == nil {
			t.Error("last_login_at should be set")
		}
	})

	t.Run("default role when no groups match", func(t *testing.T) {
		claims := &OIDCClaims{
			Subject: "default-user",
			Email:   "default@example.com",
			Groups:  []string{"random-group"},
		}

		user, err := provider.ProvisionUser(context.Background(), claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(user.Roles) != 1 || user.Roles[0] != "viewer" {
			t.Errorf("expected default role [viewer], got %v", user.Roles)
		}
	})

	t.Run("admin group mapping", func(t *testing.T) {
		claims := &OIDCClaims{
			Subject: "admin-sso-user",
			Email:   "admin@example.com",
			Groups:  []string{"platform-admins", "developers"},
		}

		user, err := provider.ProvisionUser(context.Background(), claims)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(user.Roles) != 2 {
			t.Errorf("expected 2 roles, got %d: %v", len(user.Roles), user.Roles)
		}
	})
}

func TestOIDCProvider_AuthMiddleware(t *testing.T) {
	manager := NewRBACManager(nil)
	provider := NewOIDCProvider(OIDCConfig{
		Enabled:       true,
		AutoProvision: true,
		DefaultRole:   "viewer",
		IssuerURL:     "https://test-issuer.com",
	}, manager)

	handler := provider.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok {
			t.Error("expected user in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(user.ID))
	}))

	t.Run("missing auth header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
		req.Header.Set("Authorization", "Basic abc123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid bearer token", func(t *testing.T) {
		token := makeTestJWT(map[string]interface{}{
			"sub":   "sso-user-1",
			"email": "sso@example.com",
			"name":  "SSO User",
			"iss":   "https://test-issuer.com",
			"exp":   float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		req := httptest.NewRequest("GET", "/api/v1/jobs", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "sso-user-1" {
			t.Errorf("expected user ID sso-user-1, got %s", rec.Body.String())
		}
	})

	t.Run("disabled OIDC passes through", func(t *testing.T) {
		disabledProvider := NewOIDCProvider(OIDCConfig{Enabled: false}, manager)
		h := disabledProvider.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 when OIDC disabled, got %d", rec.Code)
		}
	})
}

func TestOIDCProvider_ResolveRoles(t *testing.T) {
	provider := NewOIDCProvider(OIDCConfig{
		DefaultRole: "viewer",
		RoleMapping: map[string]string{
			"admins": "admin",
			"devs":   "developer",
		},
	}, NewRBACManager(nil))

	t.Run("no groups defaults", func(t *testing.T) {
		roles := provider.resolveRoles(nil)
		if len(roles) != 1 || roles[0] != "viewer" {
			t.Errorf("expected [viewer], got %v", roles)
		}
	})

	t.Run("mapped groups", func(t *testing.T) {
		roles := provider.resolveRoles([]string{"admins", "devs"})
		if len(roles) != 2 {
			t.Errorf("expected 2 roles, got %d", len(roles))
		}
	})

	t.Run("unmatched groups", func(t *testing.T) {
		roles := provider.resolveRoles([]string{"random"})
		if len(roles) != 1 || roles[0] != "viewer" {
			t.Errorf("expected [viewer], got %v", roles)
		}
	})
}

func TestDefaultOIDCConfig(t *testing.T) {
	cfg := DefaultOIDCConfig()
	if cfg.DefaultRole != "viewer" {
		t.Errorf("expected default role viewer, got %s", cfg.DefaultRole)
	}
	if cfg.GroupsClaim != "groups" {
		t.Errorf("expected groups claim, got %s", cfg.GroupsClaim)
	}
	if len(cfg.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(cfg.Scopes))
	}
}

func TestOIDCProvider_JWKS_DiscoverAndValidate(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksServer := startJWKSServer(t, kp)
	defer jwksServer.Close()

	provider := NewOIDCProvider(OIDCConfig{
		IssuerURL: jwksServer.URL,
		ClientID:  "test-client",
	}, NewRBACManager(nil))

	ctx := context.Background()

	// Discover should succeed
	if err := provider.Discover(ctx); err != nil {
		t.Fatalf("discovery failed: %v", err)
	}

	t.Run("valid signed token", func(t *testing.T) {
		token := makeSignedJWT(t, kp, map[string]interface{}{
			"sub":   "jwks-user-1",
			"email": "jwks@example.com",
			"name":  "JWKS User",
			"iss":   jwksServer.URL,
			"aud":   "test-client",
			"exp":   float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		claims, err := provider.ValidateToken(ctx, token)
		if err != nil {
			t.Fatalf("expected valid token, got error: %v", err)
		}
		if claims.Subject != "jwks-user-1" {
			t.Errorf("expected subject jwks-user-1, got %s", claims.Subject)
		}
		if claims.Email != "jwks@example.com" {
			t.Errorf("expected email jwks@example.com, got %s", claims.Email)
		}
	})

	t.Run("tampered token rejected", func(t *testing.T) {
		token := makeSignedJWT(t, kp, map[string]interface{}{
			"sub": "legitimate-user",
			"iss": jwksServer.URL,
			"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		// Tamper with the claims by modifying the payload
		parts := splitJWT(token)
		tamperedClaims := map[string]interface{}{
			"sub": "evil-user",
			"iss": jwksServer.URL,
			"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
		}
		tamperedJSON, _ := json.Marshal(tamperedClaims)
		parts[1] = base64.RawURLEncoding.EncodeToString(tamperedJSON)
		tamperedToken := parts[0] + "." + parts[1] + "." + parts[2]

		_, err := provider.ValidateToken(ctx, tamperedToken)
		if err == nil {
			t.Fatal("expected error for tampered token")
		}
	})

	t.Run("wrong key rejected", func(t *testing.T) {
		otherKP := generateTestKeyPair(t)
		otherKP.Kid = kp.Kid // same kid but different key

		token := makeSignedJWT(t, otherKP, map[string]interface{}{
			"sub": "wrong-key-user",
			"iss": jwksServer.URL,
			"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		_, err := provider.ValidateToken(ctx, token)
		if err == nil {
			t.Fatal("expected error for token signed with wrong key")
		}
	})

	t.Run("unknown kid rejected", func(t *testing.T) {
		otherKP := generateTestKeyPair(t)
		otherKP.Kid = "unknown-key-id"

		token := makeSignedJWT(t, otherKP, map[string]interface{}{
			"sub": "unknown-kid-user",
			"iss": jwksServer.URL,
			"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		_, err := provider.ValidateToken(ctx, token)
		if err == nil {
			t.Fatal("expected error for unknown kid")
		}
	})

	t.Run("audience validation", func(t *testing.T) {
		token := makeSignedJWT(t, kp, map[string]interface{}{
			"sub": "aud-user",
			"iss": jwksServer.URL,
			"aud": "wrong-client-id",
			"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
		})

		_, err := provider.ValidateToken(ctx, token)
		if err == nil {
			t.Fatal("expected error for wrong audience")
		}
	})
}

func TestOIDCProvider_JWKS_CacheRefresh(t *testing.T) {
	kp := generateTestKeyPair(t)
	jwksServer := startJWKSServer(t, kp)
	defer jwksServer.Close()

	provider := NewOIDCProvider(OIDCConfig{
		IssuerURL: jwksServer.URL,
	}, NewRBACManager(nil))
	provider.Discover(context.Background())

	// First call populates cache
	token1 := makeSignedJWT(t, kp, map[string]interface{}{
		"sub": "cache-user",
		"iss": jwksServer.URL,
		"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
	})

	_, err := provider.ValidateToken(context.Background(), token1)
	if err != nil {
		t.Fatalf("first validation failed: %v", err)
	}

	// Verify cache is populated
	provider.jwksCache.mu.RLock()
	cachedKeys := len(provider.jwksCache.keys)
	provider.jwksCache.mu.RUnlock()
	if cachedKeys == 0 {
		t.Error("expected JWKS cache to be populated")
	}

	// Second call should use cache (no new HTTP request needed)
	token2 := makeSignedJWT(t, kp, map[string]interface{}{
		"sub": "cache-user-2",
		"iss": jwksServer.URL,
		"exp": float64(time.Now().Add(1 * time.Hour).Unix()),
	})
	_, err = provider.ValidateToken(context.Background(), token2)
	if err != nil {
		t.Fatalf("cached validation failed: %v", err)
	}
}

func TestOIDCProvider_JWKS_DiscoveryFailure(t *testing.T) {
	provider := NewOIDCProvider(OIDCConfig{
		IssuerURL: "http://nonexistent-server.invalid",
	}, NewRBACManager(nil))

	err := provider.Discover(context.Background())
	if err == nil {
		t.Fatal("expected error for nonexistent server")
	}
}

func TestParseRSAPublicKey(t *testing.T) {
	kp := generateTestKeyPair(t)

	nB64 := base64.RawURLEncoding.EncodeToString(kp.Public.N.Bytes())
	eB64 := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(kp.Public.E)).Bytes())

	key, err := parseRSAPublicKey(nB64, eB64)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.N.Cmp(kp.Public.N) != 0 {
		t.Error("modulus mismatch")
	}
	if key.E != kp.Public.E {
		t.Error("exponent mismatch")
	}
}

func TestParseRSAPublicKey_Invalid(t *testing.T) {
	_, err := parseRSAPublicKey("!!!invalid!!!", "AQAB")
	if err == nil {
		t.Error("expected error for invalid modulus")
	}

	_, err = parseRSAPublicKey("AQAB", "!!!invalid!!!")
	if err == nil {
		t.Error("expected error for invalid exponent")
	}
}

func splitJWT(token string) []string {
	parts := make([]string, 3)
	idx1 := 0
	idx2 := 0
	count := 0
	for i, c := range token {
		if c == '.' {
			if count == 0 {
				idx1 = i
			} else {
				idx2 = i
			}
			count++
		}
	}
	parts[0] = token[:idx1]
	parts[1] = token[idx1+1 : idx2]
	parts[2] = token[idx2+1:]
	return parts
}
