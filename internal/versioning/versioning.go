// Package versioning provides job version control and canary deployment capabilities.
package versioning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chronos/chronos/internal/models"
	"github.com/google/uuid"
)

// Common errors.
var (
	ErrVersionNotFound      = errors.New("version not found")
	ErrNoVersionsExist      = errors.New("no versions exist for this job")
	ErrCanaryAlreadyActive  = errors.New("canary deployment already active")
	ErrNoCanaryActive       = errors.New("no canary deployment active")
	ErrInvalidPercentage    = errors.New("invalid percentage (must be 0-100)")
	ErrCannotRollback       = errors.New("cannot rollback - no previous version")
)

// Version represents a single version of a job configuration.
type Version struct {
	ID          string          `json:"id"`
	JobID       string          `json:"job_id"`
	Number      int             `json:"number"`
	Config      json.RawMessage `json:"config"`
	Description string          `json:"description,omitempty"`
	Author      string          `json:"author,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	Diff        *VersionDiff    `json:"diff,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

// VersionDiff shows what changed between versions.
type VersionDiff struct {
	PreviousVersion int                    `json:"previous_version"`
	Changes         []Change               `json:"changes"`
	FieldsChanged   []string               `json:"fields_changed"`
	Summary         string                 `json:"summary"`
}

// Change represents a single field change.
type Change struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
	Type     ChangeType  `json:"type"`
}

// ChangeType represents the type of change.
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeRemoved  ChangeType = "removed"
	ChangeTypeModified ChangeType = "modified"
)

// CanaryDeployment represents an active canary deployment.
type CanaryDeployment struct {
	ID              string        `json:"id"`
	JobID           string        `json:"job_id"`
	BaseVersion     int           `json:"base_version"`
	CanaryVersion   int           `json:"canary_version"`
	TrafficPercent  int           `json:"traffic_percent"`
	Status          CanaryStatus  `json:"status"`
	StartedAt       time.Time     `json:"started_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
	Metrics         *CanaryMetrics `json:"metrics,omitempty"`
	RolloutStrategy *RolloutStrategy `json:"rollout_strategy,omitempty"`
	AutoPromote     bool          `json:"auto_promote"`
	AutoRollback    bool          `json:"auto_rollback"`
}

// CanaryStatus represents the status of a canary deployment.
type CanaryStatus string

const (
	CanaryStatusPending    CanaryStatus = "pending"
	CanaryStatusRunning    CanaryStatus = "running"
	CanaryStatusPromoting  CanaryStatus = "promoting"
	CanaryStatusRollingBack CanaryStatus = "rolling_back"
	CanaryStatusPromoted   CanaryStatus = "promoted"
	CanaryStatusRolledBack CanaryStatus = "rolled_back"
	CanaryStatusFailed     CanaryStatus = "failed"
)

// CanaryMetrics tracks performance during canary deployment.
type CanaryMetrics struct {
	BaseExecutions     int           `json:"base_executions"`
	CanaryExecutions   int           `json:"canary_executions"`
	BaseSuccessRate    float64       `json:"base_success_rate"`
	CanarySuccessRate  float64       `json:"canary_success_rate"`
	BaseAvgDuration    time.Duration `json:"base_avg_duration"`
	CanaryAvgDuration  time.Duration `json:"canary_avg_duration"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// RolloutStrategy defines how canary traffic increases.
type RolloutStrategy struct {
	Type            RolloutType `json:"type"`
	Increments      []int       `json:"increments"` // e.g., [10, 25, 50, 75, 100]
	IncrementDelay  string      `json:"increment_delay"` // e.g., "1h"
	SuccessThreshold float64    `json:"success_threshold"` // min success rate to continue
	MaxFailures     int         `json:"max_failures"` // max failures before rollback
}

// RolloutType represents the type of rollout strategy.
type RolloutType string

const (
	RolloutTypeManual     RolloutType = "manual"
	RolloutTypeAutomatic  RolloutType = "automatic"
	RolloutTypeTimeBased  RolloutType = "time_based"
)

// Store defines the interface for version storage.
type Store interface {
	// Version operations
	CreateVersion(ctx context.Context, version *Version) error
	GetVersion(ctx context.Context, jobID string, versionNumber int) (*Version, error)
	GetLatestVersion(ctx context.Context, jobID string) (*Version, error)
	ListVersions(ctx context.Context, jobID string, limit int) ([]*Version, error)
	DeleteVersion(ctx context.Context, jobID string, versionNumber int) error
	
	// Canary operations
	CreateCanary(ctx context.Context, canary *CanaryDeployment) error
	GetCanary(ctx context.Context, jobID string) (*CanaryDeployment, error)
	UpdateCanary(ctx context.Context, canary *CanaryDeployment) error
	DeleteCanary(ctx context.Context, jobID string) error
	ListActiveCanaries(ctx context.Context) ([]*CanaryDeployment, error)
}

// Manager handles job versioning and canary deployments.
type Manager struct {
	mu     sync.RWMutex
	store  Store
}

// NewManager creates a new version manager.
func NewManager(store Store) *Manager {
	return &Manager{
		store: store,
	}
}

// CreateVersion creates a new version of a job.
func (m *Manager) CreateVersion(ctx context.Context, job *models.Job, author, description string) (*Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get current latest version
	var versionNumber int
	latestVersion, err := m.store.GetLatestVersion(ctx, job.ID)
	if err != nil && !errors.Is(err, ErrNoVersionsExist) {
		return nil, err
	}
	if latestVersion != nil {
		versionNumber = latestVersion.Number + 1
	} else {
		versionNumber = 1
	}

	// Serialize job config
	configJSON, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize job config: %w", err)
	}

	version := &Version{
		ID:          uuid.New().String(),
		JobID:       job.ID,
		Number:      versionNumber,
		Config:      configJSON,
		Description: description,
		Author:      author,
		CreatedAt:   time.Now().UTC(),
	}

	// Calculate diff if previous version exists
	if latestVersion != nil {
		diff, err := m.calculateDiff(latestVersion.Config, configJSON)
		if err == nil {
			diff.PreviousVersion = latestVersion.Number
			version.Diff = diff
		}
	}

	if err := m.store.CreateVersion(ctx, version); err != nil {
		return nil, err
	}

	return version, nil
}

// GetVersion retrieves a specific version of a job.
func (m *Manager) GetVersion(ctx context.Context, jobID string, versionNumber int) (*Version, error) {
	return m.store.GetVersion(ctx, jobID, versionNumber)
}

// GetLatestVersion retrieves the latest version of a job.
func (m *Manager) GetLatestVersion(ctx context.Context, jobID string) (*Version, error) {
	return m.store.GetLatestVersion(ctx, jobID)
}

// ListVersions lists all versions of a job.
func (m *Manager) ListVersions(ctx context.Context, jobID string, limit int) ([]*Version, error) {
	if limit <= 0 {
		limit = 50
	}
	return m.store.ListVersions(ctx, jobID, limit)
}

// GetJobAtVersion retrieves the job configuration at a specific version.
func (m *Manager) GetJobAtVersion(ctx context.Context, jobID string, versionNumber int) (*models.Job, error) {
	version, err := m.store.GetVersion(ctx, jobID, versionNumber)
	if err != nil {
		return nil, err
	}

	var job models.Job
	if err := json.Unmarshal(version.Config, &job); err != nil {
		return nil, fmt.Errorf("failed to deserialize job config: %w", err)
	}

	return &job, nil
}

// CompareVersions compares two versions of a job.
func (m *Manager) CompareVersions(ctx context.Context, jobID string, v1, v2 int) (*VersionComparison, error) {
	version1, err := m.store.GetVersion(ctx, jobID, v1)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v1, err)
	}

	version2, err := m.store.GetVersion(ctx, jobID, v2)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", v2, err)
	}

	diff, err := m.calculateDiff(version1.Config, version2.Config)
	if err != nil {
		return nil, err
	}

	return &VersionComparison{
		JobID:    jobID,
		Version1: v1,
		Version2: v2,
		Diff:     diff,
	}, nil
}

// VersionComparison contains the comparison between two versions.
type VersionComparison struct {
	JobID    string       `json:"job_id"`
	Version1 int          `json:"version1"`
	Version2 int          `json:"version2"`
	Diff     *VersionDiff `json:"diff"`
}

// Rollback rolls back a job to a previous version.
func (m *Manager) Rollback(ctx context.Context, jobID string, targetVersion int, author string) (*Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get the target version
	target, err := m.store.GetVersion(ctx, jobID, targetVersion)
	if err != nil {
		return nil, err
	}

	// Get current latest version
	latest, err := m.store.GetLatestVersion(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if targetVersion >= latest.Number {
		return nil, ErrCannotRollback
	}

	// Create a new version with the old config
	newVersion := &Version{
		ID:          uuid.New().String(),
		JobID:       jobID,
		Number:      latest.Number + 1,
		Config:      target.Config,
		Description: fmt.Sprintf("Rollback to version %d", targetVersion),
		Author:      author,
		CreatedAt:   time.Now().UTC(),
		Tags:        []string{"rollback", fmt.Sprintf("from_v%d", latest.Number)},
	}

	// Calculate diff
	diff, err := m.calculateDiff(latest.Config, target.Config)
	if err == nil {
		diff.PreviousVersion = latest.Number
		newVersion.Diff = diff
	}

	if err := m.store.CreateVersion(ctx, newVersion); err != nil {
		return nil, err
	}

	return newVersion, nil
}

// StartCanary starts a canary deployment for a job.
func (m *Manager) StartCanary(ctx context.Context, jobID string, canaryVersion, initialPercent int, strategy *RolloutStrategy) (*CanaryDeployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if canary already exists
	existing, _ := m.store.GetCanary(ctx, jobID)
	if existing != nil && (existing.Status == CanaryStatusRunning || existing.Status == CanaryStatusPending) {
		return nil, ErrCanaryAlreadyActive
	}

	// Validate percent
	if initialPercent < 0 || initialPercent > 100 {
		return nil, ErrInvalidPercentage
	}

	// Get latest version as base
	latest, err := m.store.GetLatestVersion(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Validate canary version exists
	_, err = m.store.GetVersion(ctx, jobID, canaryVersion)
	if err != nil {
		return nil, err
	}

	canary := &CanaryDeployment{
		ID:             uuid.New().String(),
		JobID:          jobID,
		BaseVersion:    latest.Number,
		CanaryVersion:  canaryVersion,
		TrafficPercent: initialPercent,
		Status:         CanaryStatusRunning,
		StartedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		RolloutStrategy: strategy,
		Metrics: &CanaryMetrics{
			LastUpdated: time.Now().UTC(),
		},
	}

	if strategy != nil && strategy.Type == RolloutTypeAutomatic {
		canary.AutoPromote = true
		canary.AutoRollback = true
	}

	if err := m.store.CreateCanary(ctx, canary); err != nil {
		return nil, err
	}

	return canary, nil
}

// UpdateCanaryTraffic updates the traffic percentage for a canary.
func (m *Manager) UpdateCanaryTraffic(ctx context.Context, jobID string, percent int) (*CanaryDeployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if percent < 0 || percent > 100 {
		return nil, ErrInvalidPercentage
	}

	canary, err := m.store.GetCanary(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if canary.Status != CanaryStatusRunning {
		return nil, fmt.Errorf("canary is not running (status: %s)", canary.Status)
	}

	canary.TrafficPercent = percent
	canary.UpdatedAt = time.Now().UTC()

	if err := m.store.UpdateCanary(ctx, canary); err != nil {
		return nil, err
	}

	return canary, nil
}

// PromoteCanary promotes the canary version to production.
func (m *Manager) PromoteCanary(ctx context.Context, jobID, author string) (*Version, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	canary, err := m.store.GetCanary(ctx, jobID)
	if err != nil {
		return nil, ErrNoCanaryActive
	}

	if canary.Status != CanaryStatusRunning {
		return nil, fmt.Errorf("canary is not running (status: %s)", canary.Status)
	}

	// Get canary version config
	canaryVersion, err := m.store.GetVersion(ctx, jobID, canary.CanaryVersion)
	if err != nil {
		return nil, err
	}

	// Get current latest
	latest, err := m.store.GetLatestVersion(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Create new version promoting canary
	newVersion := &Version{
		ID:          uuid.New().String(),
		JobID:       jobID,
		Number:      latest.Number + 1,
		Config:      canaryVersion.Config,
		Description: fmt.Sprintf("Promoted from canary (version %d)", canary.CanaryVersion),
		Author:      author,
		CreatedAt:   time.Now().UTC(),
		Tags:        []string{"promoted", "canary"},
	}

	if err := m.store.CreateVersion(ctx, newVersion); err != nil {
		return nil, err
	}

	// Update canary status
	canary.Status = CanaryStatusPromoted
	now := time.Now().UTC()
	canary.CompletedAt = &now
	canary.UpdatedAt = now
	m.store.UpdateCanary(ctx, canary)

	return newVersion, nil
}

// RollbackCanary rolls back a canary deployment to the base version.
func (m *Manager) RollbackCanary(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	canary, err := m.store.GetCanary(ctx, jobID)
	if err != nil {
		return ErrNoCanaryActive
	}

	if canary.Status != CanaryStatusRunning {
		return fmt.Errorf("canary is not running (status: %s)", canary.Status)
	}

	canary.Status = CanaryStatusRolledBack
	now := time.Now().UTC()
	canary.CompletedAt = &now
	canary.UpdatedAt = now

	return m.store.UpdateCanary(ctx, canary)
}

// GetCanary returns the active canary for a job.
func (m *Manager) GetCanary(ctx context.Context, jobID string) (*CanaryDeployment, error) {
	return m.store.GetCanary(ctx, jobID)
}

// UpdateCanaryMetrics updates metrics for a canary deployment.
func (m *Manager) UpdateCanaryMetrics(ctx context.Context, jobID string, isCanary bool, success bool, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	canary, err := m.store.GetCanary(ctx, jobID)
	if err != nil {
		return nil // No active canary, ignore
	}

	if canary.Status != CanaryStatusRunning {
		return nil
	}

	if canary.Metrics == nil {
		canary.Metrics = &CanaryMetrics{}
	}

	if isCanary {
		canary.Metrics.CanaryExecutions++
		total := float64(canary.Metrics.CanaryExecutions)
		if success {
			canary.Metrics.CanarySuccessRate = ((canary.Metrics.CanarySuccessRate * (total - 1)) + 1) / total
		} else {
			canary.Metrics.CanarySuccessRate = (canary.Metrics.CanarySuccessRate * (total - 1)) / total
		}
		canary.Metrics.CanaryAvgDuration = time.Duration(
			(int64(canary.Metrics.CanaryAvgDuration)*(int64(total)-1) + int64(duration)) / int64(total),
		)
	} else {
		canary.Metrics.BaseExecutions++
		total := float64(canary.Metrics.BaseExecutions)
		if success {
			canary.Metrics.BaseSuccessRate = ((canary.Metrics.BaseSuccessRate * (total - 1)) + 1) / total
		} else {
			canary.Metrics.BaseSuccessRate = (canary.Metrics.BaseSuccessRate * (total - 1)) / total
		}
		canary.Metrics.BaseAvgDuration = time.Duration(
			(int64(canary.Metrics.BaseAvgDuration)*(int64(total)-1) + int64(duration)) / int64(total),
		)
	}

	canary.Metrics.LastUpdated = time.Now().UTC()
	canary.UpdatedAt = time.Now().UTC()

	// Check auto-rollback conditions
	if canary.AutoRollback && canary.RolloutStrategy != nil {
		if canary.Metrics.CanaryExecutions >= 10 { // Min samples
			if canary.Metrics.CanarySuccessRate < canary.RolloutStrategy.SuccessThreshold {
				canary.Status = CanaryStatusRollingBack
			}
		}
	}

	return m.store.UpdateCanary(ctx, canary)
}

// ShouldUseCanary determines if a specific execution should use the canary version.
func (m *Manager) ShouldUseCanary(ctx context.Context, jobID string, executionID string) (bool, int, error) {
	canary, err := m.store.GetCanary(ctx, jobID)
	if err != nil {
		return false, 0, nil // No canary, use base version
	}

	if canary.Status != CanaryStatusRunning {
		return false, 0, nil
	}

	// Simple hash-based routing for consistent behavior
	hash := 0
	for _, c := range executionID {
		hash = (hash*31 + int(c)) % 100
	}

	useCanary := hash < canary.TrafficPercent
	if useCanary {
		return true, canary.CanaryVersion, nil
	}
	return false, canary.BaseVersion, nil
}

// calculateDiff calculates the difference between two JSON configs.
func (m *Manager) calculateDiff(oldConfig, newConfig json.RawMessage) (*VersionDiff, error) {
	var oldMap, newMap map[string]interface{}
	
	if err := json.Unmarshal(oldConfig, &oldMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(newConfig, &newMap); err != nil {
		return nil, err
	}

	diff := &VersionDiff{
		Changes:       make([]Change, 0),
		FieldsChanged: make([]string, 0),
	}

	// Find changes
	allKeys := make(map[string]bool)
	for k := range oldMap {
		allKeys[k] = true
	}
	for k := range newMap {
		allKeys[k] = true
	}

	for key := range allKeys {
		oldVal, oldExists := oldMap[key]
		newVal, newExists := newMap[key]

		if !oldExists {
			diff.Changes = append(diff.Changes, Change{
				Field:    key,
				NewValue: newVal,
				Type:     ChangeTypeAdded,
			})
			diff.FieldsChanged = append(diff.FieldsChanged, key)
		} else if !newExists {
			diff.Changes = append(diff.Changes, Change{
				Field:    key,
				OldValue: oldVal,
				Type:     ChangeTypeRemoved,
			})
			diff.FieldsChanged = append(diff.FieldsChanged, key)
		} else if !jsonEqual(oldVal, newVal) {
			diff.Changes = append(diff.Changes, Change{
				Field:    key,
				OldValue: oldVal,
				NewValue: newVal,
				Type:     ChangeTypeModified,
			})
			diff.FieldsChanged = append(diff.FieldsChanged, key)
		}
	}

	// Generate summary
	if len(diff.FieldsChanged) == 0 {
		diff.Summary = "No changes"
	} else if len(diff.FieldsChanged) == 1 {
		diff.Summary = fmt.Sprintf("Changed %s", diff.FieldsChanged[0])
	} else {
		diff.Summary = fmt.Sprintf("Changed %d fields: %v", len(diff.FieldsChanged), diff.FieldsChanged)
	}

	return diff, nil
}

func jsonEqual(a, b interface{}) bool {
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}

// MemoryStore provides an in-memory implementation of Store.
type MemoryStore struct {
	mu       sync.RWMutex
	versions map[string][]*Version // jobID -> versions
	canaries map[string]*CanaryDeployment // jobID -> canary
}

// NewMemoryStore creates a new in-memory version store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		versions: make(map[string][]*Version),
		canaries: make(map[string]*CanaryDeployment),
	}
}

func (s *MemoryStore) CreateVersion(ctx context.Context, version *Version) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versions[version.JobID] = append(s.versions[version.JobID], version)
	return nil
}

func (s *MemoryStore) GetVersion(ctx context.Context, jobID string, versionNumber int) (*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	versions := s.versions[jobID]
	for _, v := range versions {
		if v.Number == versionNumber {
			return v, nil
		}
	}
	return nil, ErrVersionNotFound
}

func (s *MemoryStore) GetLatestVersion(ctx context.Context, jobID string) (*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	versions := s.versions[jobID]
	if len(versions) == 0 {
		return nil, ErrNoVersionsExist
	}
	return versions[len(versions)-1], nil
}

func (s *MemoryStore) ListVersions(ctx context.Context, jobID string, limit int) ([]*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	versions := s.versions[jobID]
	if len(versions) <= limit {
		return versions, nil
	}
	// Return most recent versions
	return versions[len(versions)-limit:], nil
}

func (s *MemoryStore) DeleteVersion(ctx context.Context, jobID string, versionNumber int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	versions := s.versions[jobID]
	for i, v := range versions {
		if v.Number == versionNumber {
			s.versions[jobID] = append(versions[:i], versions[i+1:]...)
			return nil
		}
	}
	return ErrVersionNotFound
}

func (s *MemoryStore) CreateCanary(ctx context.Context, canary *CanaryDeployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canaries[canary.JobID] = canary
	return nil
}

func (s *MemoryStore) GetCanary(ctx context.Context, jobID string) (*CanaryDeployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	canary, ok := s.canaries[jobID]
	if !ok {
		return nil, ErrNoCanaryActive
	}
	return canary, nil
}

func (s *MemoryStore) UpdateCanary(ctx context.Context, canary *CanaryDeployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canaries[canary.JobID] = canary
	return nil
}

func (s *MemoryStore) DeleteCanary(ctx context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.canaries, jobID)
	return nil
}

func (s *MemoryStore) ListActiveCanaries(ctx context.Context) ([]*CanaryDeployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]*CanaryDeployment, 0)
	for _, c := range s.canaries {
		if c.Status == CanaryStatusRunning || c.Status == CanaryStatusPending {
			result = append(result, c)
		}
	}
	return result, nil
}
