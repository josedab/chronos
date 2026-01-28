// Package tenant provides multi-tenant SaaS capabilities for Chronos.
package tenant

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Common errors.
var (
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTenantExists       = errors.New("tenant already exists")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrTenantDisabled     = errors.New("tenant is disabled")
	ErrInvalidTenant      = errors.New("invalid tenant configuration")
	ErrInvalidAPIKey      = errors.New("invalid API key")
)

// Tier represents a subscription tier.
type Tier string

const (
	TierFree       Tier = "free"
	TierStarter    Tier = "starter"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// Tenant represents a tenant in the system.
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Email       string            `json:"email"`
	Tier        Tier              `json:"tier"`
	Status      TenantStatus      `json:"status"`
	
	// Quotas
	Quota       Quota             `json:"quota"`
	Usage       Usage             `json:"usage"`
	
	// Billing
	BillingID   string            `json:"billing_id,omitempty"`
	BillingEmail string           `json:"billing_email,omitempty"`
	
	// Settings
	Settings    TenantSettings    `json:"settings"`
	
	// Metadata
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
}

// TenantStatus represents the status of a tenant.
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusPending   TenantStatus = "pending"
	TenantStatusDisabled  TenantStatus = "disabled"
)

// Quota defines resource limits for a tenant.
type Quota struct {
	MaxJobs              int   `json:"max_jobs"`
	MaxExecutionsPerDay  int   `json:"max_executions_per_day"`
	MaxDAGs              int   `json:"max_dags"`
	MaxTriggers          int   `json:"max_triggers"`
	MaxConcurrentRuns    int   `json:"max_concurrent_runs"`
	MaxRetentionDays     int   `json:"max_retention_days"`
	MaxWebhookTimeout    int   `json:"max_webhook_timeout_seconds"`
	AllowedRegions       []string `json:"allowed_regions,omitempty"`
	Features             []string `json:"features,omitempty"`
}

// Usage tracks current resource usage.
type Usage struct {
	Jobs              int       `json:"jobs"`
	ExecutionsToday   int       `json:"executions_today"`
	DAGs              int       `json:"dags"`
	Triggers          int       `json:"triggers"`
	ConcurrentRuns    int       `json:"concurrent_runs"`
	StorageBytes      int64     `json:"storage_bytes"`
	LastUpdated       time.Time `json:"last_updated"`
}

// TenantSettings contains tenant-specific settings.
type TenantSettings struct {
	DefaultTimezone   string `json:"default_timezone"`
	DefaultRetries    int    `json:"default_retries"`
	WebhookSigningKey string `json:"webhook_signing_key,omitempty"`
	NotifyOnFailure   bool   `json:"notify_on_failure"`
	NotifyEmail       string `json:"notify_email,omitempty"`
}

// APIKey represents an API key for a tenant.
type APIKey struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	KeyHash     string    `json:"-"`
	KeyPrefix   string    `json:"key_prefix"`
	Scopes      []string  `json:"scopes"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Enabled     bool      `json:"enabled"`
}

// UsageEvent represents a billable usage event.
type UsageEvent struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Type        string    `json:"type"`
	Quantity    int       `json:"quantity"`
	Timestamp   time.Time `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Manager manages tenants and their resources.
type Manager struct {
	mu       sync.RWMutex
	tenants  map[string]*Tenant
	apiKeys  map[string]*APIKey // key prefix -> APIKey
	usage    []UsageEvent
}

// NewManager creates a new tenant manager.
func NewManager() *Manager {
	return &Manager{
		tenants: make(map[string]*Tenant),
		apiKeys: make(map[string]*APIKey),
		usage:   make([]UsageEvent, 0),
	}
}

// CreateTenant creates a new tenant.
func (m *Manager) CreateTenant(ctx context.Context, t *Tenant) error {
	if t.Name == "" || t.Email == "" {
		return ErrInvalidTenant
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate slug if not provided
	if t.Slug == "" {
		t.Slug = generateSlug(t.Name)
	}

	// Check for duplicate
	for _, existing := range m.tenants {
		if existing.Slug == t.Slug {
			return ErrTenantExists
		}
	}

	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Status == "" {
		t.Status = TenantStatusActive
	}
	if t.Tier == "" {
		t.Tier = TierFree
	}
	
	// Set default quota based on tier
	t.Quota = m.defaultQuotaForTier(t.Tier)
	t.Usage = Usage{LastUpdated: time.Now()}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()

	m.tenants[t.ID] = t
	return nil
}

// generateSlug creates a URL-safe slug from a name.
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}

// GetTenant retrieves a tenant by ID.
func (m *Manager) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[id]
	if !ok {
		return nil, ErrTenantNotFound
	}
	return t, nil
}

// GetTenantBySlug retrieves a tenant by slug.
func (m *Manager) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, ErrTenantNotFound
}

// UpdateTenant updates a tenant.
func (m *Manager) UpdateTenant(ctx context.Context, t *Tenant) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tenants[t.ID]; !ok {
		return ErrTenantNotFound
	}

	t.UpdatedAt = time.Now()
	m.tenants[t.ID] = t
	return nil
}

// DeleteTenant deletes a tenant.
func (m *Manager) DeleteTenant(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tenants[id]; !ok {
		return ErrTenantNotFound
	}

	delete(m.tenants, id)
	return nil
}

// ListTenants lists all tenants.
func (m *Manager) ListTenants(ctx context.Context) ([]*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		result = append(result, t)
	}
	return result, nil
}

// CheckQuota checks if an operation is within quota.
func (m *Manager) CheckQuota(ctx context.Context, tenantID string, resource string, delta int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[tenantID]
	if !ok {
		return ErrTenantNotFound
	}

	if t.Status != TenantStatusActive {
		return ErrTenantDisabled
	}

	switch resource {
	case "jobs":
		if t.Usage.Jobs+delta > t.Quota.MaxJobs {
			return ErrQuotaExceeded
		}
	case "executions":
		if t.Usage.ExecutionsToday+delta > t.Quota.MaxExecutionsPerDay {
			return ErrQuotaExceeded
		}
	case "dags":
		if t.Usage.DAGs+delta > t.Quota.MaxDAGs {
			return ErrQuotaExceeded
		}
	case "triggers":
		if t.Usage.Triggers+delta > t.Quota.MaxTriggers {
			return ErrQuotaExceeded
		}
	case "concurrent_runs":
		if t.Usage.ConcurrentRuns+delta > t.Quota.MaxConcurrentRuns {
			return ErrQuotaExceeded
		}
	}

	return nil
}

// IncrementUsage increments usage for a resource.
func (m *Manager) IncrementUsage(ctx context.Context, tenantID string, resource string, delta int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[tenantID]
	if !ok {
		return ErrTenantNotFound
	}

	switch resource {
	case "jobs":
		t.Usage.Jobs += delta
	case "executions":
		t.Usage.ExecutionsToday += delta
	case "dags":
		t.Usage.DAGs += delta
	case "triggers":
		t.Usage.Triggers += delta
	case "concurrent_runs":
		t.Usage.ConcurrentRuns += delta
	}
	t.Usage.LastUpdated = time.Now()

	// Record usage event for billing
	m.usage = append(m.usage, UsageEvent{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Type:      resource,
		Quantity:  delta,
		Timestamp: time.Now(),
	})

	return nil
}

// CreateAPIKey creates a new API key for a tenant.
func (m *Manager) CreateAPIKey(ctx context.Context, tenantID, name string, scopes []string) (*APIKey, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tenants[tenantID]; !ok {
		return nil, "", ErrTenantNotFound
	}

	// Generate key
	keyID := uuid.New().String()
	keyValue := "chron_" + uuid.New().String()
	keyPrefix := keyValue[:12]

	keyHash, err := hashKey(keyValue)
	if err != nil {
		return nil, "", err
	}

	key := &APIKey{
		ID:        keyID,
		TenantID:  tenantID,
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		Scopes:    scopes,
		CreatedAt: time.Now(),
		Enabled:   true,
	}

	m.apiKeys[keyPrefix] = key
	return key, keyValue, nil
}

// ValidateAPIKey validates an API key and returns the tenant.
func (m *Manager) ValidateAPIKey(ctx context.Context, keyValue string) (*Tenant, *APIKey, error) {
	if len(keyValue) < 12 {
		return nil, nil, ErrInvalidAPIKey
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := keyValue[:12]
	key, ok := m.apiKeys[prefix]
	if !ok {
		return nil, nil, ErrInvalidAPIKey
	}

	if !key.Enabled {
		return nil, nil, ErrInvalidAPIKey
	}

	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, nil, ErrInvalidAPIKey
	}

	// Verify hash using bcrypt
	if !verifyKey(keyValue, key.KeyHash) {
		return nil, nil, ErrInvalidAPIKey
	}

	tenant, ok := m.tenants[key.TenantID]
	if !ok {
		return nil, nil, ErrTenantNotFound
	}

	// Update last used timestamp (async to avoid slowing down validation)
	go m.updateLastUsed(prefix)

	return tenant, key, nil
}

// updateLastUsed updates the LastUsedAt timestamp for an API key.
func (m *Manager) updateLastUsed(keyPrefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if key, ok := m.apiKeys[keyPrefix]; ok {
		now := time.Now()
		key.LastUsedAt = &now
	}
}

// GetUsageReport returns usage for billing.
func (m *Manager) GetUsageReport(ctx context.Context, tenantID string, from, to time.Time) ([]UsageEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []UsageEvent
	for _, e := range m.usage {
		if e.TenantID == tenantID && e.Timestamp.After(from) && e.Timestamp.Before(to) {
			result = append(result, e)
		}
	}
	return result, nil
}

// defaultQuotaForTier returns default quota for a tier.
func (m *Manager) defaultQuotaForTier(tier Tier) Quota {
	switch tier {
	case TierFree:
		return Quota{
			MaxJobs:             5,
			MaxExecutionsPerDay: 100,
			MaxDAGs:             1,
			MaxTriggers:         2,
			MaxConcurrentRuns:   2,
			MaxRetentionDays:    7,
			MaxWebhookTimeout:   30,
			Features:            []string{"basic"},
		}
	case TierStarter:
		return Quota{
			MaxJobs:             25,
			MaxExecutionsPerDay: 1000,
			MaxDAGs:             5,
			MaxTriggers:         10,
			MaxConcurrentRuns:   5,
			MaxRetentionDays:    30,
			MaxWebhookTimeout:   60,
			Features:            []string{"basic", "webhooks", "notifications"},
		}
	case TierPro:
		return Quota{
			MaxJobs:             100,
			MaxExecutionsPerDay: 10000,
			MaxDAGs:             50,
			MaxTriggers:         50,
			MaxConcurrentRuns:   20,
			MaxRetentionDays:    90,
			MaxWebhookTimeout:   300,
			Features:            []string{"basic", "webhooks", "notifications", "dag", "triggers", "analytics"},
		}
	case TierEnterprise:
		return Quota{
			MaxJobs:             -1, // Unlimited
			MaxExecutionsPerDay: -1,
			MaxDAGs:             -1,
			MaxTriggers:         -1,
			MaxConcurrentRuns:   100,
			MaxRetentionDays:    365,
			MaxWebhookTimeout:   3600,
			Features:            []string{"basic", "webhooks", "notifications", "dag", "triggers", "analytics", "sso", "audit", "sla"},
		}
	default:
		return m.defaultQuotaForTier(TierFree)
	}
}

// ResetDailyUsage resets daily execution counts (call at midnight).
func (m *Manager) ResetDailyUsage(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.tenants {
		t.Usage.ExecutionsToday = 0
		t.Usage.LastUpdated = time.Now()
	}
	return nil
}

// hashKey securely hashes an API key using bcrypt.
func hashKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// verifyKey verifies an API key against its bcrypt hash.
func verifyKey(key, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key)) == nil
}
