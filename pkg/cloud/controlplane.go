// Package cloud provides control plane functionality for managed Chronos cloud.
package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTenantExists       = errors.New("tenant already exists")
	ErrClusterNotFound    = errors.New("cluster not found")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrBillingRequired    = errors.New("billing information required")
	ErrSubscriptionExpired = errors.New("subscription expired")
	ErrInvalidPlan        = errors.New("invalid plan")
)

// Tenant represents a customer tenant in the managed cloud.
type Tenant struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Email         string            `json:"email"`
	Organization  string            `json:"organization,omitempty"`
	Plan          PlanType          `json:"plan"`
	Status        TenantStatus      `json:"status"`
	Clusters      []string          `json:"clusters"`
	BillingID     string            `json:"billing_id,omitempty"`
	Quotas        *TenantQuotas     `json:"quotas"`
	Settings      *TenantSettings   `json:"settings,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	TrialEndsAt   *time.Time        `json:"trial_ends_at,omitempty"`
}

// TenantStatus represents the status of a tenant.
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusTrial     TenantStatus = "trial"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusDeleted   TenantStatus = "deleted"
)

// PlanType represents subscription plan types.
type PlanType string

const (
	PlanTypeFree       PlanType = "free"
	PlanTypeStarter    PlanType = "starter"
	PlanTypePro        PlanType = "pro"
	PlanTypeEnterprise PlanType = "enterprise"
)

// TenantQuotas defines resource quotas for a tenant.
type TenantQuotas struct {
	MaxClusters       int   `json:"max_clusters"`
	MaxJobsPerCluster int   `json:"max_jobs_per_cluster"`
	MaxExecutionsPerMonth int64 `json:"max_executions_per_month"`
	MaxStorageGB      int   `json:"max_storage_gb"`
	MaxRetentionDays  int   `json:"max_retention_days"`
	MaxConcurrentJobs int   `json:"max_concurrent_jobs"`
}

// TenantSettings holds tenant-specific settings.
type TenantSettings struct {
	DefaultRegion     string            `json:"default_region,omitempty"`
	AllowedRegions    []string          `json:"allowed_regions,omitempty"`
	NotificationEmail string            `json:"notification_email,omitempty"`
	WebhookURL        string            `json:"webhook_url,omitempty"`
	EnableAuditLog    bool              `json:"enable_audit_log"`
	CustomDomain      string            `json:"custom_domain,omitempty"`
	SSOConfig         *SSOConfig        `json:"sso_config,omitempty"`
}

// SSOConfig holds SSO configuration.
type SSOConfig struct {
	Provider     string `json:"provider"` // okta, auth0, google, azure
	Domain       string `json:"domain"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// ManagedCluster represents a managed Chronos cluster.
type ManagedCluster struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	Name          string            `json:"name"`
	Region        string            `json:"region"`
	Status        ClusterStatus     `json:"status"`
	Version       string            `json:"version"`
	Nodes         int               `json:"nodes"`
	Tier          ClusterTier       `json:"tier"`
	Endpoint      string            `json:"endpoint,omitempty"`
	APIKey        string            `json:"api_key,omitempty"`
	Config        *ClusterConfig    `json:"config,omitempty"`
	Usage         *ClusterUsage     `json:"usage,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	ProvisionedAt *time.Time        `json:"provisioned_at,omitempty"`
}

// ClusterStatus represents cluster provisioning status.
type ClusterStatus string

const (
	ClusterStatusPending      ClusterStatus = "pending"
	ClusterStatusProvisioning ClusterStatus = "provisioning"
	ClusterStatusRunning      ClusterStatus = "running"
	ClusterStatusUpdating     ClusterStatus = "updating"
	ClusterStatusStopped      ClusterStatus = "stopped"
	ClusterStatusFailed       ClusterStatus = "failed"
	ClusterStatusDeleting     ClusterStatus = "deleting"
)

// ClusterTier represents cluster size/capability tier.
type ClusterTier string

const (
	ClusterTierDev        ClusterTier = "dev"        // 1 node, limited
	ClusterTierStarter    ClusterTier = "starter"    // 3 nodes
	ClusterTierProduction ClusterTier = "production" // 5 nodes, HA
	ClusterTierEnterprise ClusterTier = "enterprise" // Custom
)

// ClusterConfig holds cluster configuration.
type ClusterConfig struct {
	AutoScaling     bool          `json:"auto_scaling"`
	MinNodes        int           `json:"min_nodes"`
	MaxNodes        int           `json:"max_nodes"`
	BackupEnabled   bool          `json:"backup_enabled"`
	BackupSchedule  string        `json:"backup_schedule,omitempty"`
	MaintenanceWindow string      `json:"maintenance_window,omitempty"`
	CustomSettings  map[string]interface{} `json:"custom_settings,omitempty"`
}

// ClusterUsage tracks cluster resource usage.
type ClusterUsage struct {
	JobCount          int       `json:"job_count"`
	ExecutionsToday   int64     `json:"executions_today"`
	ExecutionsMonth   int64     `json:"executions_month"`
	StorageUsedGB     float64   `json:"storage_used_gb"`
	CPUUtilization    float64   `json:"cpu_utilization"`
	MemoryUtilization float64   `json:"memory_utilization"`
	LastUpdated       time.Time `json:"last_updated"`
}

// UsageMetrics tracks billable usage.
type UsageMetrics struct {
	TenantID        string    `json:"tenant_id"`
	ClusterID       string    `json:"cluster_id"`
	Period          string    `json:"period"` // YYYY-MM
	Executions      int64     `json:"executions"`
	ExecutionTimeMs int64     `json:"execution_time_ms"`
	StorageGB       float64   `json:"storage_gb"`
	DataTransferGB  float64   `json:"data_transfer_gb"`
	APIRequests     int64     `json:"api_requests"`
	ComputedAt      time.Time `json:"computed_at"`
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID          string        `json:"id"`
	TenantID    string        `json:"tenant_id"`
	Period      string        `json:"period"`
	Status      InvoiceStatus `json:"status"`
	AmountCents int64         `json:"amount_cents"`
	Currency    string        `json:"currency"`
	LineItems   []LineItem    `json:"line_items"`
	DueDate     time.Time     `json:"due_date"`
	PaidAt      *time.Time    `json:"paid_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

// InvoiceStatus represents invoice status.
type InvoiceStatus string

const (
	InvoiceStatusDraft   InvoiceStatus = "draft"
	InvoiceStatusPending InvoiceStatus = "pending"
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusOverdue InvoiceStatus = "overdue"
	InvoiceStatusVoid    InvoiceStatus = "void"
)

// LineItem represents an invoice line item.
type LineItem struct {
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	UnitPrice   int64  `json:"unit_price_cents"`
	TotalCents  int64  `json:"total_cents"`
}

// Region represents an available cloud region.
type Region struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"` // aws, gcp, azure
	Location    string   `json:"location"`
	Available   bool     `json:"available"`
	Features    []string `json:"features"`
}

// ControlPlane manages the managed cloud infrastructure.
type ControlPlane struct {
	tenants   map[string]*Tenant
	clusters  map[string]*ManagedCluster
	usage     map[string]*UsageMetrics
	regions   map[string]*Region
	mu        sync.RWMutex
	config    ControlPlaneConfig
}

// ControlPlaneConfig configures the control plane.
type ControlPlaneConfig struct {
	DefaultPlan        PlanType
	TrialDays          int
	EnableAutoScaling  bool
	EnableBackups      bool
	SupportedRegions   []string
	DefaultRegion      string
}

// DefaultControlPlaneConfig returns default control plane configuration.
func DefaultControlPlaneConfig() ControlPlaneConfig {
	return ControlPlaneConfig{
		DefaultPlan:       PlanTypeFree,
		TrialDays:         14,
		EnableAutoScaling: true,
		EnableBackups:     true,
		DefaultRegion:     "us-east-1",
		SupportedRegions: []string{
			"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1",
		},
	}
}

// NewControlPlane creates a new control plane.
func NewControlPlane(cfg ControlPlaneConfig) *ControlPlane {
	cp := &ControlPlane{
		tenants:  make(map[string]*Tenant),
		clusters: make(map[string]*ManagedCluster),
		usage:    make(map[string]*UsageMetrics),
		regions:  make(map[string]*Region),
		config:   cfg,
	}

	// Initialize regions
	cp.initializeRegions()

	return cp
}

func (cp *ControlPlane) initializeRegions() {
	regions := []*Region{
		{ID: "us-east-1", Name: "US East (N. Virginia)", Provider: "aws", Location: "Virginia, USA", Available: true, Features: []string{"standard", "ha"}},
		{ID: "us-west-2", Name: "US West (Oregon)", Provider: "aws", Location: "Oregon, USA", Available: true, Features: []string{"standard", "ha"}},
		{ID: "eu-west-1", Name: "EU (Ireland)", Provider: "aws", Location: "Dublin, Ireland", Available: true, Features: []string{"standard", "ha", "gdpr"}},
		{ID: "ap-southeast-1", Name: "Asia Pacific (Singapore)", Provider: "aws", Location: "Singapore", Available: true, Features: []string{"standard", "ha"}},
		{ID: "eu-central-1", Name: "EU (Frankfurt)", Provider: "aws", Location: "Frankfurt, Germany", Available: true, Features: []string{"standard", "ha", "gdpr"}},
	}

	for _, r := range regions {
		cp.regions[r.ID] = r
	}
}

// CreateTenant creates a new tenant.
func (cp *ControlPlane) CreateTenant(ctx context.Context, name, email, organization string) (*Tenant, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Check if email already exists
	for _, t := range cp.tenants {
		if t.Email == email {
			return nil, ErrTenantExists
		}
	}

	tenant := &Tenant{
		ID:           uuid.New().String(),
		Name:         name,
		Email:        email,
		Organization: organization,
		Plan:         cp.config.DefaultPlan,
		Status:       TenantStatusTrial,
		Clusters:     []string{},
		Quotas:       cp.getQuotasForPlan(cp.config.DefaultPlan),
		Settings:     &TenantSettings{DefaultRegion: cp.config.DefaultRegion},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Set trial end date
	trialEnd := time.Now().AddDate(0, 0, cp.config.TrialDays)
	tenant.TrialEndsAt = &trialEnd

	cp.tenants[tenant.ID] = tenant
	return tenant, nil
}

// GetTenant retrieves a tenant by ID.
func (cp *ControlPlane) GetTenant(tenantID string) (*Tenant, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}
	return tenant, nil
}

// UpdateTenantPlan updates a tenant's subscription plan.
func (cp *ControlPlane) UpdateTenantPlan(tenantID string, plan PlanType) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return ErrTenantNotFound
	}

	tenant.Plan = plan
	tenant.Quotas = cp.getQuotasForPlan(plan)
	tenant.Status = TenantStatusActive
	tenant.UpdatedAt = time.Now()

	return nil
}

// CreateCluster creates a new managed cluster for a tenant.
func (cp *ControlPlane) CreateCluster(ctx context.Context, tenantID, name, region string, tier ClusterTier) (*ManagedCluster, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	// Check quota
	if len(tenant.Clusters) >= tenant.Quotas.MaxClusters {
		return nil, ErrQuotaExceeded
	}

	// Validate region
	if _, ok := cp.regions[region]; !ok {
		return nil, fmt.Errorf("invalid region: %s", region)
	}

	cluster := &ManagedCluster{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		Name:      name,
		Region:    region,
		Status:    ClusterStatusPending,
		Version:   "latest",
		Nodes:     cp.getNodesForTier(tier),
		Tier:      tier,
		APIKey:    uuid.New().String(),
		Config: &ClusterConfig{
			AutoScaling:   tier != ClusterTierDev,
			MinNodes:      1,
			MaxNodes:      cp.getMaxNodesForTier(tier),
			BackupEnabled: tier != ClusterTierDev,
		},
		Usage: &ClusterUsage{
			LastUpdated: time.Now(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cp.clusters[cluster.ID] = cluster
	tenant.Clusters = append(tenant.Clusters, cluster.ID)

	// Simulate async provisioning
	go cp.provisionCluster(cluster)

	return cluster, nil
}

// GetCluster retrieves a cluster by ID.
func (cp *ControlPlane) GetCluster(clusterID string) (*ManagedCluster, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	cluster, exists := cp.clusters[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}
	return cluster, nil
}

// ListTenantClusters lists all clusters for a tenant.
func (cp *ControlPlane) ListTenantClusters(tenantID string) ([]*ManagedCluster, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	clusters := make([]*ManagedCluster, 0, len(tenant.Clusters))
	for _, id := range tenant.Clusters {
		if c, ok := cp.clusters[id]; ok {
			clusters = append(clusters, c)
		}
	}

	return clusters, nil
}

// DeleteCluster deletes a managed cluster.
func (cp *ControlPlane) DeleteCluster(ctx context.Context, clusterID string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cluster, exists := cp.clusters[clusterID]
	if !exists {
		return ErrClusterNotFound
	}

	cluster.Status = ClusterStatusDeleting
	cluster.UpdatedAt = time.Now()

	// Remove from tenant
	if tenant, ok := cp.tenants[cluster.TenantID]; ok {
		newClusters := make([]string, 0, len(tenant.Clusters)-1)
		for _, id := range tenant.Clusters {
			if id != clusterID {
				newClusters = append(newClusters, id)
			}
		}
		tenant.Clusters = newClusters
	}

	// Simulate async deletion
	go func() {
		time.Sleep(5 * time.Second)
		cp.mu.Lock()
		delete(cp.clusters, clusterID)
		cp.mu.Unlock()
	}()

	return nil
}

// RecordUsage records usage metrics for billing.
func (cp *ControlPlane) RecordUsage(tenantID, clusterID string, executions int64, executionTimeMs int64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	period := time.Now().Format("2006-01")
	key := fmt.Sprintf("%s:%s:%s", tenantID, clusterID, period)

	usage, exists := cp.usage[key]
	if !exists {
		usage = &UsageMetrics{
			TenantID:  tenantID,
			ClusterID: clusterID,
			Period:    period,
		}
		cp.usage[key] = usage
	}

	usage.Executions += executions
	usage.ExecutionTimeMs += executionTimeMs
	usage.ComputedAt = time.Now()

	// Update cluster usage
	if cluster, ok := cp.clusters[clusterID]; ok {
		cluster.Usage.ExecutionsToday += executions
		cluster.Usage.ExecutionsMonth += executions
		cluster.Usage.LastUpdated = time.Now()
	}
}

// GetUsage retrieves usage metrics for a tenant.
func (cp *ControlPlane) GetUsage(tenantID, period string) ([]*UsageMetrics, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var results []*UsageMetrics
	prefix := tenantID + ":"

	for key, usage := range cp.usage {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			if period == "" || usage.Period == period {
				results = append(results, usage)
			}
		}
	}

	return results, nil
}

// CheckQuota checks if a tenant has quota for an operation.
func (cp *ControlPlane) CheckQuota(tenantID string, operation string) error {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return ErrTenantNotFound
	}

	// Check subscription status
	if tenant.Status == TenantStatusSuspended {
		return ErrSubscriptionExpired
	}

	if tenant.Status == TenantStatusTrial && tenant.TrialEndsAt != nil {
		if time.Now().After(*tenant.TrialEndsAt) {
			return ErrSubscriptionExpired
		}
	}

	// Check monthly execution quota
	period := time.Now().Format("2006-01")
	var totalExecutions int64
	for _, clusterID := range tenant.Clusters {
		key := fmt.Sprintf("%s:%s:%s", tenantID, clusterID, period)
		if usage, ok := cp.usage[key]; ok {
			totalExecutions += usage.Executions
		}
	}

	if totalExecutions >= tenant.Quotas.MaxExecutionsPerMonth {
		return ErrQuotaExceeded
	}

	return nil
}

// ListRegions returns available regions.
func (cp *ControlPlane) ListRegions() []*Region {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	regions := make([]*Region, 0, len(cp.regions))
	for _, r := range cp.regions {
		if r.Available {
			regions = append(regions, r)
		}
	}
	return regions
}

// GetPlanDetails returns details for a plan.
func (cp *ControlPlane) GetPlanDetails(plan PlanType) (*PlanDetails, error) {
	details := cp.getPlanDetails(plan)
	if details == nil {
		return nil, ErrInvalidPlan
	}
	return details, nil
}

// PlanDetails contains plan information.
type PlanDetails struct {
	Type         PlanType      `json:"type"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	PriceMonthly int64         `json:"price_monthly_cents"`
	Quotas       *TenantQuotas `json:"quotas"`
	Features     []string      `json:"features"`
}

func (cp *ControlPlane) getPlanDetails(plan PlanType) *PlanDetails {
	plans := map[PlanType]*PlanDetails{
		PlanTypeFree: {
			Type:         PlanTypeFree,
			Name:         "Free",
			Description:  "Perfect for trying out Chronos",
			PriceMonthly: 0,
			Quotas:       cp.getQuotasForPlan(PlanTypeFree),
			Features:     []string{"1 cluster", "100 jobs", "10K executions/month", "7 day retention"},
		},
		PlanTypeStarter: {
			Type:         PlanTypeStarter,
			Name:         "Starter",
			Description:  "For small teams",
			PriceMonthly: 2900, // $29
			Quotas:       cp.getQuotasForPlan(PlanTypeStarter),
			Features:     []string{"3 clusters", "500 jobs", "100K executions/month", "30 day retention", "Email support"},
		},
		PlanTypePro: {
			Type:         PlanTypePro,
			Name:         "Pro",
			Description:  "For growing teams",
			PriceMonthly: 9900, // $99
			Quotas:       cp.getQuotasForPlan(PlanTypePro),
			Features:     []string{"10 clusters", "Unlimited jobs", "1M executions/month", "90 day retention", "Priority support", "SSO"},
		},
		PlanTypeEnterprise: {
			Type:         PlanTypeEnterprise,
			Name:         "Enterprise",
			Description:  "For large organizations",
			PriceMonthly: 0, // Custom pricing
			Quotas:       cp.getQuotasForPlan(PlanTypeEnterprise),
			Features:     []string{"Unlimited clusters", "Unlimited jobs", "Unlimited executions", "Custom retention", "Dedicated support", "SSO", "SLA", "Custom regions"},
		},
	}

	return plans[plan]
}

func (cp *ControlPlane) getQuotasForPlan(plan PlanType) *TenantQuotas {
	quotas := map[PlanType]*TenantQuotas{
		PlanTypeFree: {
			MaxClusters:          1,
			MaxJobsPerCluster:    100,
			MaxExecutionsPerMonth: 10000,
			MaxStorageGB:         1,
			MaxRetentionDays:     7,
			MaxConcurrentJobs:    5,
		},
		PlanTypeStarter: {
			MaxClusters:          3,
			MaxJobsPerCluster:    500,
			MaxExecutionsPerMonth: 100000,
			MaxStorageGB:         10,
			MaxRetentionDays:     30,
			MaxConcurrentJobs:    25,
		},
		PlanTypePro: {
			MaxClusters:          10,
			MaxJobsPerCluster:    5000,
			MaxExecutionsPerMonth: 1000000,
			MaxStorageGB:         100,
			MaxRetentionDays:     90,
			MaxConcurrentJobs:    100,
		},
		PlanTypeEnterprise: {
			MaxClusters:          -1, // Unlimited
			MaxJobsPerCluster:    -1,
			MaxExecutionsPerMonth: -1,
			MaxStorageGB:         -1,
			MaxRetentionDays:     365,
			MaxConcurrentJobs:    -1,
		},
	}

	return quotas[plan]
}

func (cp *ControlPlane) getNodesForTier(tier ClusterTier) int {
	nodes := map[ClusterTier]int{
		ClusterTierDev:        1,
		ClusterTierStarter:    3,
		ClusterTierProduction: 5,
		ClusterTierEnterprise: 7,
	}
	return nodes[tier]
}

func (cp *ControlPlane) getMaxNodesForTier(tier ClusterTier) int {
	maxNodes := map[ClusterTier]int{
		ClusterTierDev:        1,
		ClusterTierStarter:    5,
		ClusterTierProduction: 10,
		ClusterTierEnterprise: 50,
	}
	return maxNodes[tier]
}

func (cp *ControlPlane) provisionCluster(cluster *ManagedCluster) {
	// Simulate provisioning delay
	cp.mu.Lock()
	cluster.Status = ClusterStatusProvisioning
	cp.mu.Unlock()

	time.Sleep(10 * time.Second)

	cp.mu.Lock()
	cluster.Status = ClusterStatusRunning
	cluster.Endpoint = fmt.Sprintf("https://%s.chronos.cloud", cluster.ID[:8])
	now := time.Now()
	cluster.ProvisionedAt = &now
	cluster.UpdatedAt = now
	cp.mu.Unlock()
}

// GenerateAPIKey generates a new API key for a cluster.
func (cp *ControlPlane) GenerateAPIKey(clusterID string) (string, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cluster, exists := cp.clusters[clusterID]
	if !exists {
		return "", ErrClusterNotFound
	}

	newKey := uuid.New().String()
	cluster.APIKey = newKey
	cluster.UpdatedAt = time.Now()

	return newKey, nil
}

// ExportTenantData exports all tenant data (for GDPR compliance).
func (cp *ControlPlane) ExportTenantData(tenantID string) ([]byte, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	tenant, exists := cp.tenants[tenantID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	export := map[string]interface{}{
		"tenant":   tenant,
		"clusters": make([]*ManagedCluster, 0),
		"usage":    make([]*UsageMetrics, 0),
	}

	for _, clusterID := range tenant.Clusters {
		if c, ok := cp.clusters[clusterID]; ok {
			export["clusters"] = append(export["clusters"].([]*ManagedCluster), c)
		}
	}

	for key, usage := range cp.usage {
		if len(key) >= len(tenantID) && key[:len(tenantID)] == tenantID {
			export["usage"] = append(export["usage"].([]*UsageMetrics), usage)
		}
	}

	return json.MarshalIndent(export, "", "  ")
}
