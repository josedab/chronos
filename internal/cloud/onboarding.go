// Package cloud provides self-serve onboarding for the Chronos cloud platform.
package cloud

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Onboarding errors.
var (
	ErrEmailRequired       = errors.New("email is required")
	ErrEmailAlreadyUsed    = errors.New("email already in use")
	ErrInvalidSlug         = errors.New("invalid organization slug")
	ErrSlugTaken           = errors.New("organization slug already taken")
	ErrOnboardingNotFound  = errors.New("onboarding session not found")
	ErrOnboardingExpired   = errors.New("onboarding session expired")
	ErrOnboardingCompleted = errors.New("onboarding already completed")
)

// OnboardingStep represents steps in the onboarding flow.
type OnboardingStep string

const (
	StepCreateAccount  OnboardingStep = "create_account"
	StepCreateOrg      OnboardingStep = "create_organization"
	StepSelectPlan     OnboardingStep = "select_plan"
	StepCreateFirstJob OnboardingStep = "create_first_job"
	StepOnboardDone    OnboardingStep = "done"
)

// OnboardingSession tracks a user's onboarding progress.
type OnboardingSession struct {
	ID             string         `json:"id"`
	Email          string         `json:"email"`
	Step           OnboardingStep `json:"step"`
	OrganizationID string         `json:"organization_id,omitempty"`
	APIKey         string         `json:"api_key,omitempty"`
	Plan           Plan           `json:"plan,omitempty"`
	CompletedSteps []string       `json:"completed_steps"`
	ExpiresAt      time.Time      `json:"expires_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// OnboardingService manages the self-serve onboarding flow.
type OnboardingService struct {
	mu       sync.RWMutex
	sessions map[string]*OnboardingSession
	emails   map[string]string // email -> org ID
	slugs    map[string]bool   // taken slugs
	orgs     map[string]*Organization
	apiKeys  map[string]*APIKey
}

// APIKey represents an API key for a workspace.
type APIKey struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"organization_id"`
	Scopes         []string  `json:"scopes"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

// NewOnboardingService creates a new onboarding service.
func NewOnboardingService() *OnboardingService {
	return &OnboardingService{
		sessions: make(map[string]*OnboardingSession),
		emails:   make(map[string]string),
		slugs:    make(map[string]bool),
		orgs:     make(map[string]*Organization),
		apiKeys:  make(map[string]*APIKey),
	}
}

// StartOnboarding begins a new onboarding session.
func (s *OnboardingService) StartOnboarding(_ context.Context, email string) (*OnboardingSession, error) {
	if email == "" {
		return nil, ErrEmailRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.emails[email]; exists {
		return nil, ErrEmailAlreadyUsed
	}

	session := &OnboardingSession{
		ID:             uuid.New().String(),
		Email:          email,
		Step:           StepCreateOrg,
		CompletedSteps: []string{string(StepCreateAccount)},
		ExpiresAt:      time.Now().Add(24 * time.Hour),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	s.sessions[session.ID] = session
	return session, nil
}

// CreateOrganization creates an org during onboarding.
func (s *OnboardingService) CreateOrganization(_ context.Context, sessionID, name, slug string) (*OnboardingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Step != StepCreateOrg {
		return nil, fmt.Errorf("expected step %s, got %s", StepCreateOrg, session.Step)
	}

	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || len(slug) < 3 {
		return nil, ErrInvalidSlug
	}
	if s.slugs[slug] {
		return nil, ErrSlugTaken
	}

	org := &Organization{
		ID:           uuid.New().String(),
		Name:         name,
		Slug:         slug,
		Plan:         PlanFree,
		BillingEmail: session.Email,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		CreatedBy:    session.Email,
	}

	s.orgs[org.ID] = org
	s.slugs[slug] = true
	s.emails[session.Email] = org.ID

	session.OrganizationID = org.ID
	session.Step = StepSelectPlan
	session.CompletedSteps = append(session.CompletedSteps, string(StepCreateOrg))
	session.UpdatedAt = time.Now()

	return session, nil
}

// SelectPlan sets the plan and generates an API key.
func (s *OnboardingService) SelectPlan(_ context.Context, sessionID string, plan Plan) (*OnboardingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session.Step != StepSelectPlan {
		return nil, fmt.Errorf("expected step %s, got %s", StepSelectPlan, session.Step)
	}

	// Update org plan
	org, ok := s.orgs[session.OrganizationID]
	if !ok {
		return nil, ErrOrganizationNotFound
	}
	org.Plan = plan
	org.UpdatedAt = time.Now()

	// Generate API key
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	rawKey := "chk_" + hex.EncodeToString(keyBytes)

	apiKey := &APIKey{
		ID:             uuid.New().String(),
		Key:            rawKey,
		Name:           "Default API Key",
		OrganizationID: session.OrganizationID,
		Scopes:         []string{"jobs:*", "executions:*", "workflows:*"},
		CreatedAt:      time.Now(),
	}
	s.apiKeys[apiKey.ID] = apiKey

	session.Plan = plan
	session.APIKey = rawKey
	session.Step = StepCreateFirstJob
	session.CompletedSteps = append(session.CompletedSteps, string(StepSelectPlan))
	session.UpdatedAt = time.Now()

	return session, nil
}

// CompleteOnboarding marks the onboarding as done.
func (s *OnboardingService) CompleteOnboarding(_ context.Context, sessionID string) (*OnboardingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.Step = StepOnboardDone
	session.CompletedSteps = append(session.CompletedSteps, string(StepCreateFirstJob))
	session.UpdatedAt = time.Now()

	return session, nil
}

// GetSession retrieves an onboarding session.
func (s *OnboardingService) GetSession(_ context.Context, sessionID string) (*OnboardingSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSession(sessionID)
}

// getSession retrieves a session (must hold at least read lock).
func (s *OnboardingService) getSession(sessionID string) (*OnboardingSession, error) {
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrOnboardingNotFound
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrOnboardingExpired
	}
	if session.Step == StepOnboardDone {
		return session, nil
	}
	return session, nil
}

// GetOrganization retrieves an organization by ID.
func (s *OnboardingService) GetOrganization(_ context.Context, orgID string) (*Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	org, ok := s.orgs[orgID]
	if !ok {
		return nil, ErrOrganizationNotFound
	}
	return org, nil
}

// PlanQuotas returns the quotas for a given plan.
func PlanQuotas(plan Plan) map[string]int {
	switch plan {
	case PlanFree:
		return map[string]int{
			"max_jobs":             5,
			"max_executions_day":   100,
			"max_concurrent":       2,
			"retention_days":       7,
		}
	case PlanStarter:
		return map[string]int{
			"max_jobs":             50,
			"max_executions_day":   5000,
			"max_concurrent":       10,
			"retention_days":       30,
		}
	case PlanPro:
		return map[string]int{
			"max_jobs":             500,
			"max_executions_day":   50000,
			"max_concurrent":       50,
			"retention_days":       90,
		}
	case PlanEnterprise:
		return map[string]int{
			"max_jobs":             -1, // unlimited
			"max_executions_day":   -1,
			"max_concurrent":       -1,
			"retention_days":       365,
		}
	default:
		return PlanQuotas(PlanFree)
	}
}
