// Package cloud provides managed SaaS control plane capabilities.
package cloud

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrOrganizationExists    = errors.New("organization already exists")
	ErrWorkspaceNotFound     = errors.New("workspace not found")
	ErrWorkspaceExists       = errors.New("workspace already exists")
	ErrInvitationNotFound    = errors.New("invitation not found")
	ErrInvitationExpired     = errors.New("invitation expired")
	ErrMemberNotFound        = errors.New("member not found")
	ErrMemberExists          = errors.New("member already exists")
	ErrInsufficientPermission = errors.New("insufficient permission")
)

// Organization represents a customer organization.
type Organization struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description string            `json:"description,omitempty"`
	
	// Plan information
	Plan        Plan              `json:"plan"`
	PlanDetails *PlanDetails      `json:"plan_details,omitempty"`
	
	// Billing
	BillingEmail    string        `json:"billing_email"`
	BillingAddress  *Address      `json:"billing_address,omitempty"`
	StripeCustomerID string       `json:"stripe_customer_id,omitempty"`
	
	// Settings
	Settings    OrgSettings       `json:"settings"`
	
	// Metadata
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedBy   string            `json:"created_by"`
}

// Plan represents a subscription plan.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanStarter    Plan = "starter"
	PlanPro        Plan = "pro"
	PlanEnterprise Plan = "enterprise"
)

// PlanDetails contains plan-specific limits and features.
type PlanDetails struct {
	// Resource limits
	MaxWorkspaces        int   `json:"max_workspaces"`
	MaxJobsPerWorkspace  int   `json:"max_jobs_per_workspace"`
	MaxExecutionsPerDay  int   `json:"max_executions_per_day"`
	MaxMembers           int   `json:"max_members"`
	MaxDAGs              int   `json:"max_dags"`
	MaxRetentionDays     int   `json:"max_retention_days"`
	MaxConcurrentRuns    int   `json:"max_concurrent_runs"`
	
	// Features
	Features             []Feature `json:"features"`
	
	// Pricing (monthly)
	BasePriceCents       int   `json:"base_price_cents"`
	PricePerJobCents     int   `json:"price_per_job_cents"`
	PricePerExecutionCents int `json:"price_per_execution_cents"`
}

// Feature represents a feature flag.
type Feature string

const (
	FeatureBasicScheduling Feature = "basic_scheduling"
	FeatureDAG             Feature = "dag"
	FeatureWebhooks        Feature = "webhooks"
	FeatureNotifications   Feature = "notifications"
	FeatureAnalytics       Feature = "analytics"
	FeatureAuditLog        Feature = "audit_log"
	FeatureSSO             Feature = "sso"
	FeatureAdvancedRBAC    Feature = "advanced_rbac"
	FeatureSLA             Feature = "sla"
	FeatureMultiRegion     Feature = "multi_region"
	FeatureDedicatedSupport Feature = "dedicated_support"
	FeatureCustomIntegrations Feature = "custom_integrations"
)

// OrgSettings contains organization-wide settings.
type OrgSettings struct {
	DefaultTimezone   string `json:"default_timezone"`
	EnforceSSO        bool   `json:"enforce_sso"`
	AllowedDomains    []string `json:"allowed_domains,omitempty"`
	IPAllowlist       []string `json:"ip_allowlist,omitempty"`
	RequireMFA        bool   `json:"require_mfa"`
	SessionTimeout    int    `json:"session_timeout_minutes"`
}

// Address represents a billing address.
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// Workspace represents a project workspace within an organization.
type Workspace struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organization_id"`
	Name           string            `json:"name"`
	Slug           string            `json:"slug"`
	Description    string            `json:"description,omitempty"`
	Environment    Environment       `json:"environment"`
	
	// Resource usage
	Usage          WorkspaceUsage    `json:"usage"`
	
	// Settings
	Settings       WorkspaceSettings `json:"settings"`
	
	// Metadata
	Labels         map[string]string `json:"labels,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CreatedBy      string            `json:"created_by"`
}

// Environment represents a workspace environment type.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// WorkspaceUsage tracks resource usage for a workspace.
type WorkspaceUsage struct {
	Jobs            int       `json:"jobs"`
	DAGs            int       `json:"dags"`
	ExecutionsToday int       `json:"executions_today"`
	ConcurrentRuns  int       `json:"concurrent_runs"`
	StorageBytes    int64     `json:"storage_bytes"`
	LastUpdated     time.Time `json:"last_updated"`
}

// WorkspaceSettings contains workspace-specific settings.
type WorkspaceSettings struct {
	DefaultTimezone    string            `json:"default_timezone"`
	DefaultRetries     int               `json:"default_retries"`
	DefaultTimeout     string            `json:"default_timeout"`
	WebhookSigningKey  string            `json:"webhook_signing_key,omitempty"`
	AlertChannels      []AlertChannel    `json:"alert_channels,omitempty"`
	SecretProviders    []string          `json:"secret_providers,omitempty"`
}

// AlertChannel represents a notification channel.
type AlertChannel struct {
	Type      string            `json:"type"` // slack, email, webhook, pagerduty
	Name      string            `json:"name"`
	Config    map[string]string `json:"config"`
	OnFailure bool              `json:"on_failure"`
	OnSuccess bool              `json:"on_success"`
}

// Member represents an organization member.
type Member struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	UserID         string    `json:"user_id"`
	Email          string    `json:"email"`
	Name           string    `json:"name,omitempty"`
	Role           Role      `json:"role"`
	Status         MemberStatus `json:"status"`
	InvitedBy      string    `json:"invited_by,omitempty"`
	JoinedAt       time.Time `json:"joined_at"`
	LastActiveAt   time.Time `json:"last_active_at,omitempty"`
}

// Role represents a member's role.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// MemberStatus represents a member's status.
type MemberStatus string

const (
	MemberStatusActive   MemberStatus = "active"
	MemberStatusInvited  MemberStatus = "invited"
	MemberStatusDisabled MemberStatus = "disabled"
)

// Invitation represents a pending member invitation.
type Invitation struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Email          string    `json:"email"`
	Role           Role      `json:"role"`
	Token          string    `json:"token,omitempty"`
	InvitedBy      string    `json:"invited_by"`
	ExpiresAt      time.Time `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// ControlPlane manages organizations, workspaces, and members.
type ControlPlane struct {
	mu            sync.RWMutex
	organizations map[string]*Organization
	workspaces    map[string]*Workspace
	members       map[string]*Member
	invitations   map[string]*Invitation
	planDetails   map[Plan]*PlanDetails
}

// NewControlPlane creates a new control plane instance.
func NewControlPlane() *ControlPlane {
	cp := &ControlPlane{
		organizations: make(map[string]*Organization),
		workspaces:    make(map[string]*Workspace),
		members:       make(map[string]*Member),
		invitations:   make(map[string]*Invitation),
		planDetails:   make(map[Plan]*PlanDetails),
	}
	cp.initPlanDetails()
	return cp
}

func (cp *ControlPlane) initPlanDetails() {
	cp.planDetails[PlanFree] = &PlanDetails{
		MaxWorkspaces:       1,
		MaxJobsPerWorkspace: 5,
		MaxExecutionsPerDay: 100,
		MaxMembers:          2,
		MaxDAGs:             0,
		MaxRetentionDays:    7,
		MaxConcurrentRuns:   2,
		Features:            []Feature{FeatureBasicScheduling, FeatureWebhooks},
		BasePriceCents:      0,
	}

	cp.planDetails[PlanStarter] = &PlanDetails{
		MaxWorkspaces:       3,
		MaxJobsPerWorkspace: 25,
		MaxExecutionsPerDay: 5000,
		MaxMembers:          5,
		MaxDAGs:             5,
		MaxRetentionDays:    30,
		MaxConcurrentRuns:   10,
		Features:            []Feature{FeatureBasicScheduling, FeatureWebhooks, FeatureDAG, FeatureNotifications},
		BasePriceCents:      2900, // $29/month
		PricePerExecutionCents: 1, // $0.01/execution over limit
	}

	cp.planDetails[PlanPro] = &PlanDetails{
		MaxWorkspaces:       10,
		MaxJobsPerWorkspace: 100,
		MaxExecutionsPerDay: 50000,
		MaxMembers:          25,
		MaxDAGs:             50,
		MaxRetentionDays:    90,
		MaxConcurrentRuns:   50,
		Features:            []Feature{FeatureBasicScheduling, FeatureWebhooks, FeatureDAG, FeatureNotifications, FeatureAnalytics, FeatureAuditLog, FeatureAdvancedRBAC},
		BasePriceCents:      9900, // $99/month
		PricePerExecutionCents: 1,
	}

	cp.planDetails[PlanEnterprise] = &PlanDetails{
		MaxWorkspaces:       -1, // Unlimited
		MaxJobsPerWorkspace: -1,
		MaxExecutionsPerDay: -1,
		MaxMembers:          -1,
		MaxDAGs:             -1,
		MaxRetentionDays:    365,
		MaxConcurrentRuns:   200,
		Features:            []Feature{FeatureBasicScheduling, FeatureWebhooks, FeatureDAG, FeatureNotifications, FeatureAnalytics, FeatureAuditLog, FeatureAdvancedRBAC, FeatureSSO, FeatureSLA, FeatureMultiRegion, FeatureDedicatedSupport, FeatureCustomIntegrations},
		BasePriceCents:      49900, // $499/month base
		PricePerExecutionCents: 0,
	}
}

// CreateOrganization creates a new organization.
func (cp *ControlPlane) CreateOrganization(ctx context.Context, org *Organization) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Check for duplicate slug
	for _, existing := range cp.organizations {
		if existing.Slug == org.Slug {
			return ErrOrganizationExists
		}
	}

	if org.ID == "" {
		org.ID = uuid.New().String()
	}
	if org.Plan == "" {
		org.Plan = PlanFree
	}
	org.PlanDetails = cp.planDetails[org.Plan]
	org.CreatedAt = time.Now()
	org.UpdatedAt = time.Now()

	cp.organizations[org.ID] = org
	return nil
}

// GetOrganization retrieves an organization by ID.
func (cp *ControlPlane) GetOrganization(ctx context.Context, id string) (*Organization, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	org, ok := cp.organizations[id]
	if !ok {
		return nil, ErrOrganizationNotFound
	}
	return org, nil
}

// GetOrganizationBySlug retrieves an organization by slug.
func (cp *ControlPlane) GetOrganizationBySlug(ctx context.Context, slug string) (*Organization, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	for _, org := range cp.organizations {
		if org.Slug == slug {
			return org, nil
		}
	}
	return nil, ErrOrganizationNotFound
}

// UpdateOrganization updates an organization.
func (cp *ControlPlane) UpdateOrganization(ctx context.Context, org *Organization) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.organizations[org.ID]; !ok {
		return ErrOrganizationNotFound
	}

	org.UpdatedAt = time.Now()
	cp.organizations[org.ID] = org
	return nil
}

// DeleteOrganization deletes an organization and all its workspaces.
func (cp *ControlPlane) DeleteOrganization(ctx context.Context, id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.organizations[id]; !ok {
		return ErrOrganizationNotFound
	}

	// Delete all workspaces
	for wsID, ws := range cp.workspaces {
		if ws.OrganizationID == id {
			delete(cp.workspaces, wsID)
		}
	}

	// Delete all members
	for mID, m := range cp.members {
		if m.OrganizationID == id {
			delete(cp.members, mID)
		}
	}

	delete(cp.organizations, id)
	return nil
}

// ListOrganizations lists all organizations for a user.
func (cp *ControlPlane) ListOrganizations(ctx context.Context, userID string) ([]*Organization, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]*Organization, 0)
	for _, org := range cp.organizations {
		// Check if user is a member
		for _, member := range cp.members {
			if member.OrganizationID == org.ID && member.UserID == userID {
				result = append(result, org)
				break
			}
		}
	}
	return result, nil
}

// ChangePlan changes an organization's plan.
func (cp *ControlPlane) ChangePlan(ctx context.Context, orgID string, newPlan Plan) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	org, ok := cp.organizations[orgID]
	if !ok {
		return ErrOrganizationNotFound
	}

	org.Plan = newPlan
	org.PlanDetails = cp.planDetails[newPlan]
	org.UpdatedAt = time.Now()

	return nil
}

// CreateWorkspace creates a new workspace.
func (cp *ControlPlane) CreateWorkspace(ctx context.Context, ws *Workspace) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Verify organization exists
	org, ok := cp.organizations[ws.OrganizationID]
	if !ok {
		return ErrOrganizationNotFound
	}

	// Check workspace limit
	if org.PlanDetails.MaxWorkspaces > 0 {
		count := 0
		for _, existing := range cp.workspaces {
			if existing.OrganizationID == ws.OrganizationID {
				count++
			}
		}
		if count >= org.PlanDetails.MaxWorkspaces {
			return errors.New("workspace limit exceeded")
		}
	}

	// Check for duplicate slug within org
	for _, existing := range cp.workspaces {
		if existing.OrganizationID == ws.OrganizationID && existing.Slug == ws.Slug {
			return ErrWorkspaceExists
		}
	}

	if ws.ID == "" {
		ws.ID = uuid.New().String()
	}
	if ws.Environment == "" {
		ws.Environment = EnvDevelopment
	}
	ws.Usage = WorkspaceUsage{LastUpdated: time.Now()}
	ws.CreatedAt = time.Now()
	ws.UpdatedAt = time.Now()

	cp.workspaces[ws.ID] = ws
	return nil
}

// GetWorkspace retrieves a workspace by ID.
func (cp *ControlPlane) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	ws, ok := cp.workspaces[id]
	if !ok {
		return nil, ErrWorkspaceNotFound
	}
	return ws, nil
}

// GetWorkspaceBySlug retrieves a workspace by organization and slug.
func (cp *ControlPlane) GetWorkspaceBySlug(ctx context.Context, orgID, slug string) (*Workspace, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	for _, ws := range cp.workspaces {
		if ws.OrganizationID == orgID && ws.Slug == slug {
			return ws, nil
		}
	}
	return nil, ErrWorkspaceNotFound
}

// UpdateWorkspace updates a workspace.
func (cp *ControlPlane) UpdateWorkspace(ctx context.Context, ws *Workspace) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.workspaces[ws.ID]; !ok {
		return ErrWorkspaceNotFound
	}

	ws.UpdatedAt = time.Now()
	cp.workspaces[ws.ID] = ws
	return nil
}

// DeleteWorkspace deletes a workspace.
func (cp *ControlPlane) DeleteWorkspace(ctx context.Context, id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.workspaces[id]; !ok {
		return ErrWorkspaceNotFound
	}

	delete(cp.workspaces, id)
	return nil
}

// ListWorkspaces lists all workspaces for an organization.
func (cp *ControlPlane) ListWorkspaces(ctx context.Context, orgID string) ([]*Workspace, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]*Workspace, 0)
	for _, ws := range cp.workspaces {
		if ws.OrganizationID == orgID {
			result = append(result, ws)
		}
	}
	return result, nil
}

// AddMember adds a member to an organization.
func (cp *ControlPlane) AddMember(ctx context.Context, member *Member) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Verify organization exists
	org, ok := cp.organizations[member.OrganizationID]
	if !ok {
		return ErrOrganizationNotFound
	}

	// Check member limit
	if org.PlanDetails.MaxMembers > 0 {
		count := 0
		for _, existing := range cp.members {
			if existing.OrganizationID == member.OrganizationID {
				count++
			}
		}
		if count >= org.PlanDetails.MaxMembers {
			return errors.New("member limit exceeded")
		}
	}

	// Check for duplicate
	for _, existing := range cp.members {
		if existing.OrganizationID == member.OrganizationID && existing.Email == member.Email {
			return ErrMemberExists
		}
	}

	if member.ID == "" {
		member.ID = uuid.New().String()
	}
	member.JoinedAt = time.Now()

	cp.members[member.ID] = member
	return nil
}

// GetMember retrieves a member by ID.
func (cp *ControlPlane) GetMember(ctx context.Context, id string) (*Member, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	member, ok := cp.members[id]
	if !ok {
		return nil, ErrMemberNotFound
	}
	return member, nil
}

// UpdateMember updates a member's details.
func (cp *ControlPlane) UpdateMember(ctx context.Context, member *Member) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.members[member.ID]; !ok {
		return ErrMemberNotFound
	}

	cp.members[member.ID] = member
	return nil
}

// RemoveMember removes a member from an organization.
func (cp *ControlPlane) RemoveMember(ctx context.Context, id string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.members[id]; !ok {
		return ErrMemberNotFound
	}

	delete(cp.members, id)
	return nil
}

// ListMembers lists all members of an organization.
func (cp *ControlPlane) ListMembers(ctx context.Context, orgID string) ([]*Member, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	result := make([]*Member, 0)
	for _, member := range cp.members {
		if member.OrganizationID == orgID {
			result = append(result, member)
		}
	}
	return result, nil
}

// CreateInvitation creates a new invitation.
func (cp *ControlPlane) CreateInvitation(ctx context.Context, inv *Invitation) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if _, ok := cp.organizations[inv.OrganizationID]; !ok {
		return ErrOrganizationNotFound
	}

	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	if inv.Token == "" {
		inv.Token = uuid.New().String()
	}
	inv.CreatedAt = time.Now()
	if inv.ExpiresAt.IsZero() {
		inv.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	}

	cp.invitations[inv.ID] = inv
	return nil
}

// AcceptInvitation accepts an invitation and adds the user as a member.
func (cp *ControlPlane) AcceptInvitation(ctx context.Context, token, userID, userName string) (*Member, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var inv *Invitation
	for _, i := range cp.invitations {
		if i.Token == token {
			inv = i
			break
		}
	}

	if inv == nil {
		return nil, ErrInvitationNotFound
	}

	if time.Now().After(inv.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	member := &Member{
		ID:             uuid.New().String(),
		OrganizationID: inv.OrganizationID,
		UserID:         userID,
		Email:          inv.Email,
		Name:           userName,
		Role:           inv.Role,
		Status:         MemberStatusActive,
		InvitedBy:      inv.InvitedBy,
		JoinedAt:       time.Now(),
	}

	cp.members[member.ID] = member
	delete(cp.invitations, inv.ID)

	return member, nil
}

// GetPlanDetails returns details for a specific plan.
func (cp *ControlPlane) GetPlanDetails(plan Plan) *PlanDetails {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.planDetails[plan]
}

// GetAllPlans returns all available plans.
func (cp *ControlPlane) GetAllPlans() map[Plan]*PlanDetails {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	result := make(map[Plan]*PlanDetails)
	for k, v := range cp.planDetails {
		result[k] = v
	}
	return result
}

// HasFeature checks if an organization has access to a feature.
func (cp *ControlPlane) HasFeature(ctx context.Context, orgID string, feature Feature) (bool, error) {
	org, err := cp.GetOrganization(ctx, orgID)
	if err != nil {
		return false, err
	}

	for _, f := range org.PlanDetails.Features {
		if f == feature {
			return true, nil
		}
	}
	return false, nil
}

// CheckWorkspaceQuota checks if a workspace operation is within quota.
func (cp *ControlPlane) CheckWorkspaceQuota(ctx context.Context, wsID, resource string, delta int) error {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	ws, ok := cp.workspaces[wsID]
	if !ok {
		return ErrWorkspaceNotFound
	}

	org, ok := cp.organizations[ws.OrganizationID]
	if !ok {
		return ErrOrganizationNotFound
	}

	limits := org.PlanDetails

	switch resource {
	case "jobs":
		if limits.MaxJobsPerWorkspace > 0 && ws.Usage.Jobs+delta > limits.MaxJobsPerWorkspace {
			return errors.New("job limit exceeded")
		}
	case "executions":
		if limits.MaxExecutionsPerDay > 0 && ws.Usage.ExecutionsToday+delta > limits.MaxExecutionsPerDay {
			return errors.New("daily execution limit exceeded")
		}
	case "dags":
		if limits.MaxDAGs > 0 && ws.Usage.DAGs+delta > limits.MaxDAGs {
			return errors.New("DAG limit exceeded")
		}
	case "concurrent":
		if limits.MaxConcurrentRuns > 0 && ws.Usage.ConcurrentRuns+delta > limits.MaxConcurrentRuns {
			return errors.New("concurrent run limit exceeded")
		}
	}

	return nil
}

// IncrementUsage increments usage counters for a workspace.
func (cp *ControlPlane) IncrementUsage(ctx context.Context, wsID, resource string, delta int) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	ws, ok := cp.workspaces[wsID]
	if !ok {
		return ErrWorkspaceNotFound
	}

	switch resource {
	case "jobs":
		ws.Usage.Jobs += delta
	case "executions":
		ws.Usage.ExecutionsToday += delta
	case "dags":
		ws.Usage.DAGs += delta
	case "concurrent":
		ws.Usage.ConcurrentRuns += delta
	}
	ws.Usage.LastUpdated = time.Now()

	return nil
}
