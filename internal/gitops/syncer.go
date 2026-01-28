// Package gitops provides Git-based job synchronization for Chronos.
package gitops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Common errors.
var (
	ErrRepoNotFound     = errors.New("repository not found")
	ErrSyncInProgress   = errors.New("sync already in progress")
	ErrInvalidJobSpec   = errors.New("invalid job specification")
	ErrNoChanges        = errors.New("no changes detected")
)

// Config configures the GitOps syncer.
type Config struct {
	// Enabled enables GitOps synchronization.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// RepoURL is the Git repository URL.
	RepoURL string `json:"repo_url" yaml:"repo_url"`
	// Branch is the branch to sync from.
	Branch string `json:"branch" yaml:"branch"`
	// Path is the directory path within the repo containing job definitions.
	Path string `json:"path" yaml:"path"`
	// SyncInterval is how often to poll for changes.
	SyncInterval time.Duration `json:"sync_interval" yaml:"sync_interval"`
	// SSHKeyPath is the path to SSH private key for authentication.
	SSHKeyPath string `json:"ssh_key_path" yaml:"ssh_key_path"`
	// Token is the access token for HTTPS authentication.
	Token string `json:"token" yaml:"token"`
	// AutoApply automatically applies changes without approval.
	AutoApply bool `json:"auto_apply" yaml:"auto_apply"`
}

// DefaultConfig returns default GitOps configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:      false,
		Branch:       "main",
		Path:         "jobs",
		SyncInterval: 5 * time.Minute,
		AutoApply:    false,
	}
}

// JobSpec represents a job definition in YAML.
type JobSpec struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       JobData  `yaml:"spec"`
}

// Metadata contains job metadata.
type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// JobData contains the job specification.
type JobData struct {
	Schedule    string                 `yaml:"schedule"`
	Timezone    string                 `yaml:"timezone,omitempty"`
	Description string                 `yaml:"description,omitempty"`
	Webhook     WebhookSpec            `yaml:"webhook"`
	Retry       *RetrySpec             `yaml:"retry,omitempty"`
	Timeout     string                 `yaml:"timeout,omitempty"`
	Concurrency string                 `yaml:"concurrency,omitempty"`
	Enabled     *bool                  `yaml:"enabled,omitempty"`
	Tags        map[string]string      `yaml:"tags,omitempty"`
	DependsOn   []string               `yaml:"dependsOn,omitempty"`
	Secrets     map[string]string      `yaml:"secrets,omitempty"`
}

// WebhookSpec defines webhook configuration.
type WebhookSpec struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Body    string            `yaml:"body,omitempty"`
	Auth    *AuthSpec         `yaml:"auth,omitempty"`
}

// AuthSpec defines authentication.
type AuthSpec struct {
	Type   string `yaml:"type"` // basic, bearer, api_key
	Secret string `yaml:"secret"`
}

// RetrySpec defines retry policy.
type RetrySpec struct {
	MaxAttempts     int    `yaml:"maxAttempts"`
	InitialInterval string `yaml:"initialInterval"`
	MaxInterval     string `yaml:"maxInterval"`
	Multiplier      float64 `yaml:"multiplier"`
}

// SyncResult represents the result of a sync operation.
type SyncResult struct {
	Timestamp   time.Time     `json:"timestamp"`
	CommitSHA   string        `json:"commit_sha"`
	Branch      string        `json:"branch"`
	Created     []string      `json:"created"`
	Updated     []string      `json:"updated"`
	Deleted     []string      `json:"deleted"`
	Unchanged   []string      `json:"unchanged"`
	Errors      []SyncError   `json:"errors,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// SyncError represents an error during sync.
type SyncError struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// JobHandler handles job operations.
type JobHandler interface {
	CreateJob(ctx context.Context, spec *JobSpec) error
	UpdateJob(ctx context.Context, spec *JobSpec) error
	DeleteJob(ctx context.Context, name, namespace string) error
	GetJob(ctx context.Context, name, namespace string) (*JobSpec, error)
	ListJobs(ctx context.Context, namespace string) ([]*JobSpec, error)
}

// Syncer synchronizes jobs from Git repositories.
type Syncer struct {
	mu          sync.Mutex
	config      Config
	handler     JobHandler
	lastSync    *SyncResult
	lastHashes  map[string]string
	stopCh      chan struct{}
	running     bool
}

// NewSyncer creates a new GitOps syncer.
func NewSyncer(cfg Config, handler JobHandler) *Syncer {
	return &Syncer{
		config:     cfg,
		handler:    handler,
		lastHashes: make(map[string]string),
	}
}

// Start begins the sync loop.
func (s *Syncer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrSyncInProgress
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	// Initial sync
	if _, err := s.Sync(ctx); err != nil {
		// Log error but continue
		_ = err
	}

	// Start sync loop
	go s.syncLoop(ctx)

	return nil
}

// Stop stops the sync loop.
func (s *Syncer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// syncLoop periodically syncs from Git.
func (s *Syncer) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if _, err := s.Sync(ctx); err != nil {
				// Log error but continue
				_ = err
			}
		}
	}
}

// Sync performs a synchronization from Git.
func (s *Syncer) Sync(ctx context.Context) (*SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	result := &SyncResult{
		Timestamp: start,
		Branch:    s.config.Branch,
		Created:   []string{},
		Updated:   []string{},
		Deleted:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}

	// Read job files from the configured path
	// In production, this would clone/pull from Git
	specs, hashes, err := s.readJobSpecs(s.config.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read job specs: %w", err)
	}

	// Determine changes
	currentNames := make(map[string]bool)
	for name, spec := range specs {
		currentNames[name] = true

		oldHash, exists := s.lastHashes[name]
		newHash := hashes[name]

		if !exists {
			// New job
			if s.config.AutoApply {
				if err := s.handler.CreateJob(ctx, spec); err != nil {
					result.Errors = append(result.Errors, SyncError{
						File:    name,
						Message: err.Error(),
					})
				} else {
					result.Created = append(result.Created, name)
				}
			} else {
				result.Created = append(result.Created, name+" (pending)")
			}
		} else if oldHash != newHash {
			// Updated job
			if s.config.AutoApply {
				if err := s.handler.UpdateJob(ctx, spec); err != nil {
					result.Errors = append(result.Errors, SyncError{
						File:    name,
						Message: err.Error(),
					})
				} else {
					result.Updated = append(result.Updated, name)
				}
			} else {
				result.Updated = append(result.Updated, name+" (pending)")
			}
		} else {
			result.Unchanged = append(result.Unchanged, name)
		}
	}

	// Check for deleted jobs
	for name := range s.lastHashes {
		if !currentNames[name] {
			if s.config.AutoApply {
				parts := parseJobName(name)
				if err := s.handler.DeleteJob(ctx, parts.name, parts.namespace); err != nil {
					result.Errors = append(result.Errors, SyncError{
						File:    name,
						Message: err.Error(),
					})
				} else {
					result.Deleted = append(result.Deleted, name)
				}
			} else {
				result.Deleted = append(result.Deleted, name+" (pending)")
			}
		}
	}

	// Update state
	s.lastHashes = hashes
	s.lastSync = result
	result.Duration = time.Since(start)

	return result, nil
}

// readJobSpecs reads job specifications from a directory.
func (s *Syncer) readJobSpecs(path string) (map[string]*JobSpec, map[string]string, error) {
	specs := make(map[string]*JobSpec)
	hashes := make(map[string]string)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(filePath)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		var spec JobSpec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return fmt.Errorf("failed to parse %s: %w", filePath, err)
		}

		if spec.Kind != "ChronosJob" {
			return nil
		}

		name := spec.Metadata.Name
		if spec.Metadata.Namespace != "" {
			name = spec.Metadata.Namespace + "/" + name
		}

		specs[name] = &spec
		hashes[name] = hashContent(data)

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}

	return specs, hashes, nil
}

// GetLastSync returns the last sync result.
func (s *Syncer) GetLastSync() *SyncResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSync
}

// DryRun performs a sync without applying changes.
func (s *Syncer) DryRun(ctx context.Context) (*SyncResult, error) {
	oldAutoApply := s.config.AutoApply
	s.config.AutoApply = false
	defer func() { s.config.AutoApply = oldAutoApply }()

	return s.Sync(ctx)
}

// hashContent returns SHA256 hash of content.
func hashContent(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// jobNameParts holds parsed job name components.
type jobNameParts struct {
	namespace string
	name      string
}

// parseJobName parses "namespace/name" or "name".
func parseJobName(s string) jobNameParts {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return jobNameParts{
				namespace: s[:i],
				name:      s[i+1:],
			}
		}
	}
	return jobNameParts{name: s}
}

// ValidateSpec validates a job specification.
func ValidateSpec(spec *JobSpec) error {
	if spec.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if spec.Kind != "ChronosJob" {
		return fmt.Errorf("kind must be 'ChronosJob'")
	}
	if spec.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if spec.Spec.Schedule == "" {
		return fmt.Errorf("spec.schedule is required")
	}
	if spec.Spec.Webhook.URL == "" {
		return fmt.Errorf("spec.webhook.url is required")
	}
	return nil
}
