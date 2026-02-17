package cloud

import (
	"context"
	"testing"
)

func TestOnboarding_FullFlow(t *testing.T) {
	svc := NewOnboardingService()
	ctx := context.Background()

	// Step 1: Start onboarding
	session, err := svc.StartOnboarding(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if session.Step != StepCreateOrg {
		t.Errorf("expected step %s, got %s", StepCreateOrg, session.Step)
	}

	// Step 2: Create org
	session, err = svc.CreateOrganization(ctx, session.ID, "Acme Corp", "acme-corp")
	if err != nil {
		t.Fatalf("create org failed: %v", err)
	}
	if session.OrganizationID == "" {
		t.Error("expected org ID")
	}
	if session.Step != StepSelectPlan {
		t.Errorf("expected step %s, got %s", StepSelectPlan, session.Step)
	}

	// Step 3: Select plan
	session, err = svc.SelectPlan(ctx, session.ID, PlanStarter)
	if err != nil {
		t.Fatalf("select plan failed: %v", err)
	}
	if session.APIKey == "" {
		t.Error("expected API key to be generated")
	}
	if session.Plan != PlanStarter {
		t.Errorf("expected plan starter, got %s", session.Plan)
	}

	// Step 4: Complete onboarding
	session, err = svc.CompleteOnboarding(ctx, session.ID)
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if session.Step != StepOnboardDone {
		t.Errorf("expected step %s, got %s", StepOnboardDone, session.Step)
	}
	if len(session.CompletedSteps) != 4 {
		t.Errorf("expected 4 completed steps, got %d", len(session.CompletedSteps))
	}

	// Verify org
	org, err := svc.GetOrganization(ctx, session.OrganizationID)
	if err != nil {
		t.Fatalf("get org failed: %v", err)
	}
	if org.Name != "Acme Corp" {
		t.Errorf("expected name Acme Corp, got %s", org.Name)
	}
	if org.Plan != PlanStarter {
		t.Errorf("expected plan starter, got %s", org.Plan)
	}
}

func TestOnboarding_DuplicateEmail(t *testing.T) {
	svc := NewOnboardingService()
	ctx := context.Background()

	session, _ := svc.StartOnboarding(ctx, "dup@example.com")
	svc.CreateOrganization(ctx, session.ID, "Org1", "org-one")

	_, err := svc.StartOnboarding(ctx, "dup@example.com")
	if err != ErrEmailAlreadyUsed {
		t.Errorf("expected ErrEmailAlreadyUsed, got %v", err)
	}
}

func TestOnboarding_DuplicateSlug(t *testing.T) {
	svc := NewOnboardingService()
	ctx := context.Background()

	s1, _ := svc.StartOnboarding(ctx, "a@example.com")
	svc.CreateOrganization(ctx, s1.ID, "Org", "my-org")

	s2, _ := svc.StartOnboarding(ctx, "b@example.com")
	_, err := svc.CreateOrganization(ctx, s2.ID, "Org2", "my-org")
	if err != ErrSlugTaken {
		t.Errorf("expected ErrSlugTaken, got %v", err)
	}
}

func TestOnboarding_InvalidSlug(t *testing.T) {
	svc := NewOnboardingService()
	ctx := context.Background()

	session, _ := svc.StartOnboarding(ctx, "test@example.com")
	_, err := svc.CreateOrganization(ctx, session.ID, "Org", "ab")
	if err != ErrInvalidSlug {
		t.Errorf("expected ErrInvalidSlug, got %v", err)
	}
}

func TestOnboarding_EmptyEmail(t *testing.T) {
	svc := NewOnboardingService()
	_, err := svc.StartOnboarding(context.Background(), "")
	if err != ErrEmailRequired {
		t.Errorf("expected ErrEmailRequired, got %v", err)
	}
}

func TestOnboarding_SessionNotFound(t *testing.T) {
	svc := NewOnboardingService()
	_, err := svc.GetSession(context.Background(), "nonexistent")
	if err != ErrOnboardingNotFound {
		t.Errorf("expected ErrOnboardingNotFound, got %v", err)
	}
}

func TestOnboarding_WrongStep(t *testing.T) {
	svc := NewOnboardingService()
	ctx := context.Background()

	session, _ := svc.StartOnboarding(ctx, "test@example.com")

	// Try selecting plan before creating org
	_, err := svc.SelectPlan(ctx, session.ID, PlanPro)
	if err == nil {
		t.Error("expected error for wrong step")
	}
}

func TestPlanQuotas(t *testing.T) {
	free := PlanQuotas(PlanFree)
	if free["max_jobs"] != 5 {
		t.Errorf("expected free max_jobs=5, got %d", free["max_jobs"])
	}

	enterprise := PlanQuotas(PlanEnterprise)
	if enterprise["max_jobs"] != -1 {
		t.Errorf("expected enterprise max_jobs=-1 (unlimited), got %d", enterprise["max_jobs"])
	}

	starter := PlanQuotas(PlanStarter)
	if starter["max_executions_day"] != 5000 {
		t.Errorf("expected starter max_executions_day=5000, got %d", starter["max_executions_day"])
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	svc := NewOnboardingService()
	ctx := context.Background()

	session, _ := svc.StartOnboarding(ctx, "key@example.com")
	session, _ = svc.CreateOrganization(ctx, session.ID, "KeyOrg", "key-org")
	session, _ = svc.SelectPlan(ctx, session.ID, PlanFree)

	if session.APIKey == "" {
		t.Fatal("expected API key")
	}
	if len(session.APIKey) < 20 {
		t.Error("API key seems too short")
	}
	if session.APIKey[:4] != "chk_" {
		t.Errorf("expected API key prefix 'chk_', got %s", session.APIKey[:4])
	}
}
