package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

func createTestOrg(cp *ControlPlane, name, email string) *Organization {
	org := &Organization{
		Name:         name,
		Slug:         strings.ReplaceAll(strings.ToLower(name), " ", "-"),
		BillingEmail: email,
	}
	cp.CreateOrganization(context.Background(), org)
	return org
}

func TestControlPlane_CreateOrganization(t *testing.T) {
	cp := NewControlPlane()

	org := &Organization{
		Name:         "Test Org",
		Slug:         "test-org",
		BillingEmail: "admin@test.com",
	}

	err := cp.CreateOrganization(context.Background(), org)
	if err != nil {
		t.Fatalf("CreateOrganization failed: %v", err)
	}

	if org.Name != "Test Org" {
		t.Errorf("expected name 'Test Org', got %q", org.Name)
	}
	if org.ID == "" {
		t.Error("expected ID to be generated")
	}
	if org.Plan != PlanFree {
		t.Errorf("expected plan Free, got %q", org.Plan)
	}
}

func TestControlPlane_CreateWorkspace(t *testing.T) {
	cp := NewControlPlane()

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	ws := &Workspace{
		Name:           "Production",
		Slug:           "production",
		OrganizationID: org.ID,
		Environment:    EnvProduction,
	}
	err := cp.CreateWorkspace(context.Background(), ws)
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	if ws.Name != "Production" {
		t.Errorf("expected name 'Production', got %q", ws.Name)
	}
	if ws.Environment != EnvProduction {
		t.Errorf("expected environment 'production', got %q", ws.Environment)
	}
}

func TestControlPlane_WorkspaceQuota(t *testing.T) {
	cp := NewControlPlane()

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	// Free plan allows 1 workspace
	ws1 := &Workspace{Name: "Workspace 1", Slug: "ws1", OrganizationID: org.ID, Environment: EnvDevelopment}
	err := cp.CreateWorkspace(context.Background(), ws1)
	if err != nil {
		t.Fatalf("First workspace should succeed: %v", err)
	}

	// Second workspace should fail on free plan
	ws2 := &Workspace{Name: "Workspace 2", Slug: "ws2", OrganizationID: org.ID, Environment: EnvStaging}
	err = cp.CreateWorkspace(context.Background(), ws2)
	if err == nil {
		t.Error("expected error for exceeding workspace limit")
	}
	if !strings.Contains(err.Error(), "limit exceeded") {
		t.Errorf("expected limit exceeded error, got %v", err)
	}
}

func TestControlPlane_AddMember(t *testing.T) {
	cp := NewControlPlane()

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	member := &Member{
		OrganizationID: org.ID,
		Email:          "user@test.com",
		Role:           RoleMember,
	}
	err := cp.AddMember(context.Background(), member)
	if err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	if member.Email != "user@test.com" {
		t.Errorf("expected email 'user@test.com', got %q", member.Email)
	}
	if member.Role != RoleMember {
		t.Errorf("expected role Member, got %q", member.Role)
	}
}

func TestControlPlane_MemberQuota(t *testing.T) {
	cp := NewControlPlane()

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	// Free plan allows 2 members
	for i := 1; i <= 2; i++ {
		m := &Member{
			OrganizationID: org.ID,
			Email:          "user" + string(rune('0'+i)) + "@test.com",
			Role:           RoleMember,
		}
		err := cp.AddMember(context.Background(), m)
		if err != nil {
			t.Fatalf("Member %d should succeed: %v", i, err)
		}
	}

	// Third member should fail on free plan
	m3 := &Member{OrganizationID: org.ID, Email: "user3@test.com", Role: RoleMember}
	err := cp.AddMember(context.Background(), m3)
	if err == nil {
		t.Error("expected error for exceeding member limit")
	}
	if !strings.Contains(err.Error(), "limit exceeded") {
		t.Errorf("expected limit exceeded error, got %v", err)
	}
}

func TestControlPlane_GetOrganization(t *testing.T) {
	cp := NewControlPlane()

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	retrieved, err := cp.GetOrganization(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("GetOrganization failed: %v", err)
	}

	if retrieved.ID != org.ID {
		t.Errorf("expected ID %q, got %q", org.ID, retrieved.ID)
	}
}

func TestControlPlane_GetOrganization_NotFound(t *testing.T) {
	cp := NewControlPlane()

	_, err := cp.GetOrganization(context.Background(), "nonexistent")
	if err != ErrOrganizationNotFound {
		t.Errorf("expected ErrOrganizationNotFound, got %v", err)
	}
}

func TestControlPlane_DeleteOrganization(t *testing.T) {
	cp := NewControlPlane()

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	err := cp.DeleteOrganization(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("DeleteOrganization failed: %v", err)
	}

	_, err = cp.GetOrganization(context.Background(), org.ID)
	if err != ErrOrganizationNotFound {
		t.Errorf("expected ErrOrganizationNotFound after delete, got %v", err)
	}
}

func TestBillingService_CreateSubscription(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	sub, err := billing.CreateSubscription(context.Background(), org.ID, PlanPro)
	if err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}

	if sub.Plan != PlanPro {
		t.Errorf("expected plan Pro, got %q", sub.Plan)
	}
}

func TestBillingService_GetSubscription(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)

	org := createTestOrg(cp, "Test Org", "admin@test.com")
	sub, _ := billing.CreateSubscription(context.Background(), org.ID, PlanStarter)

	retrieved, err := billing.GetSubscription(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("GetSubscription failed: %v", err)
	}

	if retrieved.Plan != PlanStarter {
		t.Errorf("expected plan Starter, got %q", retrieved.Plan)
	}
}

func TestBillingService_GetSubscriptionByOrg(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)

	org := createTestOrg(cp, "Test Org", "admin@test.com")
	billing.CreateSubscription(context.Background(), org.ID, PlanPro)

	sub, err := billing.GetSubscriptionByOrg(context.Background(), org.ID)
	if err != nil {
		t.Fatalf("GetSubscriptionByOrg failed: %v", err)
	}

	if sub.Plan != PlanPro {
		t.Errorf("expected plan Pro, got %q", sub.Plan)
	}
}

func TestBillingService_GenerateInvoice(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)

	org := createTestOrg(cp, "Test Org", "admin@test.com")
	sub, _ := billing.CreateSubscription(context.Background(), org.ID, PlanPro)

	invoice, err := billing.GenerateInvoice(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("GenerateInvoice failed: %v", err)
	}

	if invoice.Status != InvoiceOpen {
		t.Errorf("expected status Open, got %q", invoice.Status)
	}
}

func TestHandler_CreateOrganization(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)
	logger := zerolog.Nop()
	handler := NewHandler(cp, billing, logger)

	body := `{"name": "Test Org", "slug": "test-org", "billing_email": "admin@test.com"}`
	req := httptest.NewRequest("POST", "/cloud/organizations", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success response")
	}
}

func TestHandler_GetOrganization(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)
	logger := zerolog.Nop()
	handler := NewHandler(cp, billing, logger)

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	req := httptest.NewRequest("GET", "/cloud/organizations/"+org.ID, nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_CreateWorkspace(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)
	logger := zerolog.Nop()
	handler := NewHandler(cp, billing, logger)

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	body := `{"name": "Production", "slug": "production", "environment": "production"}`
	req := httptest.NewRequest("POST", "/cloud/organizations/"+org.ID+"/workspaces", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_AddMember(t *testing.T) {
	cp := NewControlPlane()
	billing := NewBillingService(cp)
	logger := zerolog.Nop()
	handler := NewHandler(cp, billing, logger)

	org := createTestOrg(cp, "Test Org", "admin@test.com")

	body := `{"email": "user@test.com", "role": "member"}`
	req := httptest.NewRequest("POST", "/cloud/organizations/"+org.ID+"/members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}
