package tenant

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("expected manager, got nil")
	}
	if manager.tenants == nil {
		t.Error("expected tenants map to be initialized")
	}
}

func TestManager_CreateTenant(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{
		Name:  "Test Tenant",
		Email: "test@example.com",
		Tier:  TierStarter,
	}

	err := manager.CreateTenant(context.Background(), tenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.ID == "" {
		t.Error("expected ID to be set")
	}
	if tenant.Slug == "" {
		t.Error("expected slug to be set")
	}
	if tenant.Status != TenantStatusActive {
		t.Errorf("expected status active, got %s", tenant.Status)
	}
	if tenant.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestManager_CreateTenant_WithSlug(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{
		Name:  "Test Tenant",
		Email: "test@example.com",
		Slug:  "custom-slug",
		Tier:  TierPro,
	}

	err := manager.CreateTenant(context.Background(), tenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tenant.Slug != "custom-slug" {
		t.Errorf("expected slug custom-slug, got %s", tenant.Slug)
	}
}

func TestManager_CreateTenant_DuplicateSlug(t *testing.T) {
	manager := NewManager()

	tenant1 := &Tenant{Name: "Tenant 1", Email: "t1@example.com", Slug: "same-slug", Tier: TierFree}
	tenant2 := &Tenant{Name: "Tenant 2", Email: "t2@example.com", Slug: "same-slug", Tier: TierFree}

	manager.CreateTenant(context.Background(), tenant1)
	err := manager.CreateTenant(context.Background(), tenant2)

	if err != ErrTenantExists {
		t.Errorf("expected ErrTenantExists, got %v", err)
	}
}

func TestManager_GetTenant(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Test Tenant", Email: "test@example.com", Tier: TierStarter}
	manager.CreateTenant(context.Background(), tenant)

	retrieved, err := manager.GetTenant(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Name != "Test Tenant" {
		t.Errorf("expected name 'Test Tenant', got %s", retrieved.Name)
	}
}

func TestManager_GetTenant_NotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.GetTenant(context.Background(), "nonexistent")
	if err != ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestManager_GetTenantBySlug(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Test Tenant", Email: "test@example.com", Slug: "test-tenant", Tier: TierStarter}
	manager.CreateTenant(context.Background(), tenant)

	retrieved, err := manager.GetTenantBySlug(context.Background(), "test-tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.Slug != "test-tenant" {
		t.Errorf("expected slug test-tenant, got %s", retrieved.Slug)
	}
}

func TestManager_UpdateTenant(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Original Name", Email: "test@example.com", Tier: TierFree}
	manager.CreateTenant(context.Background(), tenant)

	tenant.Name = "Updated Name"
	tenant.Tier = TierPro

	err := manager.UpdateTenant(context.Background(), tenant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := manager.GetTenant(context.Background(), tenant.ID)
	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", updated.Name)
	}
	if updated.Tier != TierPro {
		t.Errorf("expected tier pro, got %s", updated.Tier)
	}
}

func TestManager_UpdateTenant_NotFound(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{ID: "nonexistent", Name: "Test"}
	err := manager.UpdateTenant(context.Background(), tenant)

	if err != ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestManager_DeleteTenant(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Test Tenant", Email: "test@example.com", Tier: TierFree}
	manager.CreateTenant(context.Background(), tenant)

	err := manager.DeleteTenant(context.Background(), tenant.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = manager.GetTenant(context.Background(), tenant.ID)
	if err != ErrTenantNotFound {
		t.Error("expected tenant to be deleted")
	}
}

func TestManager_ListTenants(t *testing.T) {
	manager := NewManager()

	for i := 0; i < 5; i++ {
		tenant := &Tenant{
			Name:  "Tenant " + string(rune('A'+i)),
			Email: "tenant" + string(rune('a'+i)) + "@example.com",
			Tier:  TierFree,
		}
		manager.CreateTenant(context.Background(), tenant)
	}

	tenants, err := manager.ListTenants(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tenants) != 5 {
		t.Errorf("expected 5 tenants, got %d", len(tenants))
	}
}

func TestManager_CreateAPIKey(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Test Tenant", Email: "test@example.com", Tier: TierStarter}
	manager.CreateTenant(context.Background(), tenant)

	apiKey, keyValue, err := manager.CreateAPIKey(context.Background(), tenant.ID, "test-key", []string{"read", "write"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if keyValue == "" {
		t.Error("expected API key value to be set")
	}
	if apiKey.Name != "test-key" {
		t.Errorf("expected name test-key, got %s", apiKey.Name)
	}
}

func TestManager_CreateAPIKey_TenantNotFound(t *testing.T) {
	manager := NewManager()

	_, _, err := manager.CreateAPIKey(context.Background(), "nonexistent", "key", nil)
	if err != ErrTenantNotFound {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestManager_ValidateAPIKey(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Test Tenant", Email: "test@example.com", Tier: TierStarter}
	manager.CreateTenant(context.Background(), tenant)

	_, keyValue, _ := manager.CreateAPIKey(context.Background(), tenant.ID, "test-key", []string{"read"})

	validatedTenant, _, err := manager.ValidateAPIKey(context.Background(), keyValue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if validatedTenant.ID != tenant.ID {
		t.Errorf("expected tenant ID %s, got %s", tenant.ID, validatedTenant.ID)
	}
}

func TestManager_ValidateAPIKey_Invalid(t *testing.T) {
	manager := NewManager()

	_, _, err := manager.ValidateAPIKey(context.Background(), "invalid-key")
	if err != ErrInvalidAPIKey {
		t.Errorf("expected ErrInvalidAPIKey, got %v", err)
	}
}

func TestManager_CheckQuota(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{
		Name:  "Test Tenant",
		Email: "test@example.com",
		Tier:  TierStarter,
	}
	manager.CreateTenant(context.Background(), tenant)

	err := manager.CheckQuota(context.Background(), tenant.ID, "jobs", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_IncrementUsage(t *testing.T) {
	manager := NewManager()

	tenant := &Tenant{Name: "Test Tenant", Email: "test@example.com", Tier: TierStarter}
	manager.CreateTenant(context.Background(), tenant)

	err := manager.IncrementUsage(context.Background(), tenant.ID, "executions", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := manager.GetTenant(context.Background(), tenant.ID)
	if updated.Usage.ExecutionsToday != 1 {
		t.Errorf("expected 1 execution, got %d", updated.Usage.ExecutionsToday)
	}
}

func TestTiers(t *testing.T) {
	tiers := []Tier{TierFree, TierStarter, TierPro, TierEnterprise}

	for _, tier := range tiers {
		if tier == "" {
			t.Error("tier should not be empty")
		}
	}
}

func TestTenantStatuses(t *testing.T) {
	statuses := []TenantStatus{
		TenantStatusActive,
		TenantStatusSuspended,
		TenantStatusPending,
		TenantStatusDisabled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("status should not be empty")
		}
	}
}

func TestQuota_Fields(t *testing.T) {
	quota := Quota{
		MaxJobs:             100,
		MaxExecutionsPerDay: 10000,
		MaxDAGs:             50,
		MaxConcurrentRuns:   10,
	}

	if quota.MaxJobs != 100 {
		t.Errorf("expected max jobs 100, got %d", quota.MaxJobs)
	}
}

func TestUsage_Fields(t *testing.T) {
	usage := Usage{
		Jobs:            10,
		ExecutionsToday: 500,
		DAGs:            5,
	}

	if usage.Jobs != 10 {
		t.Errorf("expected 10 jobs, got %d", usage.Jobs)
	}
}

func TestTenant_Fields(t *testing.T) {
	expires := time.Now().Add(30 * 24 * time.Hour)
	tenant := Tenant{
		ID:        "tenant-1",
		Name:      "Test Tenant",
		Slug:      "test-tenant",
		Email:     "test@example.com",
		Tier:      TierPro,
		Status:    TenantStatusActive,
		ExpiresAt: &expires,
		Labels:    map[string]string{"env": "prod"},
	}

	if tenant.Tier != TierPro {
		t.Errorf("expected tier pro, got %s", tenant.Tier)
	}
	if tenant.ExpiresAt == nil {
		t.Error("expected expires_at to be set")
	}
}
