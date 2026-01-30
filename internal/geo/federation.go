// Package geo provides cross-region federation for Chronos clusters.
package geo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Federation errors.
var (
	ErrFederationDisabled = errors.New("federation is disabled")
	ErrClusterNotFound    = errors.New("cluster not found")
	ErrSyncConflict       = errors.New("sync conflict detected")
	ErrUnauthorized       = errors.New("unauthorized cluster")
)

// FederatedCluster represents a remote Chronos cluster.
type FederatedCluster struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Region      string            `json:"region"`
	Endpoints   []string          `json:"endpoints"`
	APIKey      string            `json:"api_key,omitempty"`
	TLSConfig   *TLSConfig        `json:"tls,omitempty"`
	Priority    int               `json:"priority"`
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	JoinedAt    time.Time         `json:"joined_at"`
}

// TLSConfig holds TLS configuration for cluster communication.
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	CACertFile         string `json:"ca_cert_file,omitempty"`
	CertFile           string `json:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty"`
}

// ClusterStatus represents the status of a federated cluster.
type ClusterStatus struct {
	ClusterID   string        `json:"cluster_id"`
	Region      string        `json:"region"`
	Healthy     bool          `json:"healthy"`
	IsLeader    bool          `json:"is_leader"`
	NodeCount   int           `json:"node_count"`
	JobCount    int           `json:"job_count"`
	Latency     time.Duration `json:"latency"`
	LastSync    time.Time     `json:"last_sync"`
	LastCheck   time.Time     `json:"last_check"`
	Version     string        `json:"version"`
	Error       string        `json:"error,omitempty"`
}

// SyncMessage represents a synchronization message between clusters.
type SyncMessage struct {
	ID          string          `json:"id"`
	Type        SyncMessageType `json:"type"`
	SourceCluster string        `json:"source_cluster"`
	TargetCluster string        `json:"target_cluster,omitempty"` // Empty for broadcast
	Payload     json.RawMessage `json:"payload"`
	Version     int64           `json:"version"`
	Timestamp   time.Time       `json:"timestamp"`
	Signature   string          `json:"signature,omitempty"`
}

// SyncMessageType represents the type of sync message.
type SyncMessageType string

const (
	SyncTypeJobCreate   SyncMessageType = "job_create"
	SyncTypeJobUpdate   SyncMessageType = "job_update"
	SyncTypeJobDelete   SyncMessageType = "job_delete"
	SyncTypeJobSync     SyncMessageType = "job_sync"
	SyncTypeExecution   SyncMessageType = "execution"
	SyncTypeHeartbeat   SyncMessageType = "heartbeat"
	SyncTypeClusterJoin SyncMessageType = "cluster_join"
	SyncTypeClusterLeave SyncMessageType = "cluster_leave"
	SyncTypeFullSync    SyncMessageType = "full_sync"
)

// FederatedJob represents a job that is federated across clusters.
type FederatedJob struct {
	JobID           string           `json:"job_id"`
	Name            string           `json:"name"`
	OwnerCluster    string           `json:"owner_cluster"`
	ReplicaClusters []string         `json:"replica_clusters"`
	Version         int64            `json:"version"`
	FederationConfig *JobFederationConfig `json:"federation_config"`
	LastSyncedAt    time.Time        `json:"last_synced_at"`
}

// JobFederationConfig configures how a job is federated.
type JobFederationConfig struct {
	// Enabled enables federation for this job.
	Enabled bool `json:"enabled"`
	// Mode is the federation mode.
	Mode FederationMode `json:"mode"`
	// TargetClusters is the list of clusters to sync to.
	TargetClusters []string `json:"target_clusters,omitempty"`
	// ExcludedClusters is the list of clusters to exclude.
	ExcludedClusters []string `json:"excluded_clusters,omitempty"`
	// ExecutionPolicy determines where the job can execute.
	ExecutionPolicy ExecutionPolicy `json:"execution_policy"`
	// ConflictResolution determines how conflicts are resolved.
	ConflictResolution ConflictResolution `json:"conflict_resolution"`
}

// FederationMode represents how jobs are federated.
type FederationMode string

const (
	// FederationModeReplicate copies jobs to all target clusters.
	FederationModeReplicate FederationMode = "replicate"
	// FederationModeFailover syncs jobs for failover purposes only.
	FederationModeFailover FederationMode = "failover"
	// FederationModePartition partitions jobs across clusters.
	FederationModePartition FederationMode = "partition"
)

// ExecutionPolicy determines where federated jobs can execute.
type ExecutionPolicy string

const (
	// ExecutionPolicyOwnerOnly allows execution only on owner cluster.
	ExecutionPolicyOwnerOnly ExecutionPolicy = "owner_only"
	// ExecutionPolicyAnyCluster allows execution on any replica.
	ExecutionPolicyAnyCluster ExecutionPolicy = "any_cluster"
	// ExecutionPolicyAllClusters executes on all clusters.
	ExecutionPolicyAllClusters ExecutionPolicy = "all_clusters"
	// ExecutionPolicyPreferred prefers owner but fails over.
	ExecutionPolicyPreferred ExecutionPolicy = "preferred"
)

// ConflictResolution determines how sync conflicts are resolved.
type ConflictResolution string

const (
	ConflictResolutionNewest     ConflictResolution = "newest"
	ConflictResolutionOwnerWins  ConflictResolution = "owner_wins"
	ConflictResolutionMerge      ConflictResolution = "merge"
	ConflictResolutionManual     ConflictResolution = "manual"
)

// FederationManager manages cluster federation.
type FederationManager struct {
	localClusterID string
	clusters       map[string]*FederatedCluster
	status         map[string]*ClusterStatus
	jobs           map[string]*FederatedJob
	syncQueue      chan *SyncMessage
	httpClient     *http.Client
	config         FederationConfig
	mu             sync.RWMutex
	stopCh         chan struct{}
}

// FederationConfig configures the federation manager.
type FederationConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	LocalClusterID    string        `json:"local_cluster_id" yaml:"local_cluster_id"`
	LocalClusterName  string        `json:"local_cluster_name" yaml:"local_cluster_name"`
	LocalRegion       string        `json:"local_region" yaml:"local_region"`
	SyncInterval      time.Duration `json:"sync_interval" yaml:"sync_interval"`
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	SyncTimeout       time.Duration `json:"sync_timeout" yaml:"sync_timeout"`
	MaxSyncRetries    int           `json:"max_sync_retries" yaml:"max_sync_retries"`
	ConflictResolution ConflictResolution `json:"conflict_resolution" yaml:"conflict_resolution"`
	// TLS configuration for secure cluster communication
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	CACertFile         string `json:"ca_cert_file" yaml:"ca_cert_file"`
}

// DefaultFederationConfig returns default federation configuration.
func DefaultFederationConfig() FederationConfig {
	return FederationConfig{
		Enabled:            false,
		SyncInterval:       30 * time.Second,
		HealthCheckInterval: 15 * time.Second,
		SyncTimeout:        10 * time.Second,
		MaxSyncRetries:     3,
		ConflictResolution: ConflictResolutionNewest,
		InsecureSkipVerify: false, // Secure by default
	}
}

// NewFederationManager creates a new federation manager.
func NewFederationManager(cfg FederationConfig) *FederationManager {
	if cfg.LocalClusterID == "" {
		cfg.LocalClusterID = uuid.New().String()
	}

	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		// Fall back to basic TLS config if CA loading fails
		tlsConfig = &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			MinVersion:         tls.VersionTLS12,
		}
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
	}

	return &FederationManager{
		localClusterID: cfg.LocalClusterID,
		clusters:       make(map[string]*FederatedCluster),
		status:         make(map[string]*ClusterStatus),
		jobs:           make(map[string]*FederatedJob),
		syncQueue:      make(chan *SyncMessage, 1000),
		httpClient:     &http.Client{Transport: transport, Timeout: cfg.SyncTimeout},
		config:         cfg,
		stopCh:         make(chan struct{}),
	}
}

// buildTLSConfig creates a TLS configuration with optional CA certificate loading.
func buildTLSConfig(cfg FederationConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// Load CA certificate if specified
	if cfg.CACertFile != "" {
		caCert, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	return tlsConfig, nil
}

// Start starts the federation manager.
func (f *FederationManager) Start(ctx context.Context) error {
	if !f.config.Enabled {
		return nil
	}

	go f.syncLoop(ctx)
	go f.healthCheckLoop(ctx)

	return nil
}

// Stop stops the federation manager.
func (f *FederationManager) Stop() {
	close(f.stopCh)
}

// AddCluster adds a remote cluster to the federation.
func (f *FederationManager) AddCluster(cluster *FederatedCluster) error {
	if !f.config.Enabled {
		return ErrFederationDisabled
	}

	f.mu.Lock()
	if cluster.ID == "" {
		cluster.ID = uuid.New().String()
	}
	cluster.JoinedAt = time.Now()

	f.clusters[cluster.ID] = cluster
	f.status[cluster.ID] = &ClusterStatus{
		ClusterID: cluster.ID,
		Region:    cluster.Region,
		Healthy:   true,
		LastCheck: time.Now(),
	}
	f.mu.Unlock()

	// Broadcast join to other clusters (outside lock to avoid deadlock)
	f.broadcastAsync(&SyncMessage{
		ID:            uuid.New().String(),
		Type:          SyncTypeClusterJoin,
		SourceCluster: f.localClusterID,
		Payload:       mustJSON(cluster),
		Timestamp:     time.Now(),
	})

	return nil
}

// RemoveCluster removes a cluster from the federation.
func (f *FederationManager) RemoveCluster(clusterID string) error {
	f.mu.Lock()
	if _, exists := f.clusters[clusterID]; !exists {
		f.mu.Unlock()
		return ErrClusterNotFound
	}

	delete(f.clusters, clusterID)
	delete(f.status, clusterID)
	f.mu.Unlock()

	// Broadcast leave (outside lock to avoid deadlock)
	f.broadcastAsync(&SyncMessage{
		ID:            uuid.New().String(),
		Type:          SyncTypeClusterLeave,
		SourceCluster: f.localClusterID,
		Payload:       mustJSON(map[string]string{"cluster_id": clusterID}),
		Timestamp:     time.Now(),
	})

	return nil
}

// GetCluster retrieves a federated cluster.
func (f *FederationManager) GetCluster(clusterID string) (*FederatedCluster, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	cluster, exists := f.clusters[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}
	return cluster, nil
}

// ListClusters returns all federated clusters.
func (f *FederationManager) ListClusters() []*FederatedCluster {
	f.mu.RLock()
	defer f.mu.RUnlock()

	clusters := make([]*FederatedCluster, 0, len(f.clusters))
	for _, c := range f.clusters {
		clusters = append(clusters, c)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Priority < clusters[j].Priority
	})

	return clusters
}

// GetClusterStatus returns the status of a cluster.
func (f *FederationManager) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	status, exists := f.status[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}
	return status, nil
}

// GetAllClusterStatus returns status of all clusters.
func (f *FederationManager) GetAllClusterStatus() []*ClusterStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	statuses := make([]*ClusterStatus, 0, len(f.status))
	for _, s := range f.status {
		statuses = append(statuses, s)
	}
	return statuses
}

// FederateJob adds a job to federation.
func (f *FederationManager) FederateJob(jobID string, config *JobFederationConfig) error {
	if !f.config.Enabled {
		return ErrFederationDisabled
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	fedJob := &FederatedJob{
		JobID:            jobID,
		OwnerCluster:     f.localClusterID,
		Version:          1,
		FederationConfig: config,
		LastSyncedAt:     time.Now(),
	}

	f.jobs[jobID] = fedJob

	// Sync to target clusters
	targets := f.getTargetClusters(config)
	for _, clusterID := range targets {
		f.syncQueue <- &SyncMessage{
			ID:            uuid.New().String(),
			Type:          SyncTypeJobCreate,
			SourceCluster: f.localClusterID,
			TargetCluster: clusterID,
			Payload:       mustJSON(fedJob),
			Version:       fedJob.Version,
			Timestamp:     time.Now(),
		}
	}

	return nil
}

// SyncJob synchronizes a job across federation.
func (f *FederationManager) SyncJob(jobID string, jobData interface{}) error {
	f.mu.Lock()
	fedJob, exists := f.jobs[jobID]
	if !exists {
		f.mu.Unlock()
		return nil // Not a federated job
	}

	fedJob.Version++
	fedJob.LastSyncedAt = time.Now()
	version := fedJob.Version

	// Copy the replica list to avoid data race after releasing lock
	replicaClusters := make([]string, len(fedJob.ReplicaClusters))
	copy(replicaClusters, fedJob.ReplicaClusters)
	f.mu.Unlock()

	// Send sync to replicas (outside lock)
	for _, clusterID := range replicaClusters {
		f.syncQueue <- &SyncMessage{
			ID:            uuid.New().String(),
			Type:          SyncTypeJobUpdate,
			SourceCluster: f.localClusterID,
			TargetCluster: clusterID,
			Payload:       mustJSON(map[string]interface{}{"job": jobData, "meta": fedJob}),
			Version:       version,
			Timestamp:     time.Now(),
		}
	}

	return nil
}

// DeleteFederatedJob removes a job from federation.
func (f *FederationManager) DeleteFederatedJob(jobID string) error {
	f.mu.Lock()
	fedJob, exists := f.jobs[jobID]
	if !exists {
		f.mu.Unlock()
		return nil
	}
	// Copy the replica list before deleting
	replicaClusters := make([]string, len(fedJob.ReplicaClusters))
	copy(replicaClusters, fedJob.ReplicaClusters)
	delete(f.jobs, jobID)
	f.mu.Unlock()

	// Broadcast delete (outside lock)
	for _, clusterID := range replicaClusters {
		f.syncQueue <- &SyncMessage{
			ID:            uuid.New().String(),
			Type:          SyncTypeJobDelete,
			SourceCluster: f.localClusterID,
			TargetCluster: clusterID,
			Payload:       mustJSON(map[string]string{"job_id": jobID}),
			Timestamp:     time.Now(),
		}
	}

	return nil
}

// ReceiveSync processes an incoming sync message.
func (f *FederationManager) ReceiveSync(msg *SyncMessage) error {
	// Verify source cluster is known
	f.mu.RLock()
	_, exists := f.clusters[msg.SourceCluster]
	f.mu.RUnlock()

	if !exists {
		return ErrUnauthorized
	}

	switch msg.Type {
	case SyncTypeJobCreate:
		return f.handleJobCreate(msg)
	case SyncTypeJobUpdate:
		return f.handleJobUpdate(msg)
	case SyncTypeJobDelete:
		return f.handleJobDelete(msg)
	case SyncTypeExecution:
		return f.handleExecution(msg)
	case SyncTypeHeartbeat:
		return f.handleHeartbeat(msg)
	case SyncTypeClusterJoin:
		return f.handleClusterJoin(msg)
	case SyncTypeClusterLeave:
		return f.handleClusterLeave(msg)
	}

	return nil
}

func (f *FederationManager) handleJobCreate(msg *SyncMessage) error {
	var fedJob FederatedJob
	if err := json.Unmarshal(msg.Payload, &fedJob); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Check for conflicts
	existing, exists := f.jobs[fedJob.JobID]
	if exists && existing.Version >= fedJob.Version {
		return ErrSyncConflict
	}

	// Add as replica
	fedJob.ReplicaClusters = append(fedJob.ReplicaClusters, f.localClusterID)
	f.jobs[fedJob.JobID] = &fedJob

	return nil
}

func (f *FederationManager) handleJobUpdate(msg *SyncMessage) error {
	var payload struct {
		Job  interface{}    `json:"job"`
		Meta FederatedJob   `json:"meta"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	existing, exists := f.jobs[payload.Meta.JobID]
	if !exists {
		// Create if doesn't exist
		f.jobs[payload.Meta.JobID] = &payload.Meta
		return nil
	}

	// Check version for conflict
	if existing.Version >= payload.Meta.Version {
		switch f.config.ConflictResolution {
		case ConflictResolutionNewest:
			// Accept newer timestamp
			if msg.Timestamp.After(existing.LastSyncedAt) {
				f.jobs[payload.Meta.JobID] = &payload.Meta
			}
		case ConflictResolutionOwnerWins:
			// Only accept if from owner
			if msg.SourceCluster == existing.OwnerCluster {
				f.jobs[payload.Meta.JobID] = &payload.Meta
			}
		default:
			return ErrSyncConflict
		}
		return nil
	}

	f.jobs[payload.Meta.JobID] = &payload.Meta
	return nil
}

func (f *FederationManager) handleJobDelete(msg *SyncMessage) error {
	var payload struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.jobs, payload.JobID)
	return nil
}

func (f *FederationManager) handleExecution(msg *SyncMessage) error {
	// Handle execution notifications from other clusters
	// This allows tracking distributed executions
	return nil
}

func (f *FederationManager) handleHeartbeat(msg *SyncMessage) error {
	var status ClusterStatus
	if err := json.Unmarshal(msg.Payload, &status); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.status[msg.SourceCluster] = &status
	return nil
}

func (f *FederationManager) handleClusterJoin(msg *SyncMessage) error {
	var cluster FederatedCluster
	if err := json.Unmarshal(msg.Payload, &cluster); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.clusters[cluster.ID] = &cluster
	f.status[cluster.ID] = &ClusterStatus{
		ClusterID: cluster.ID,
		Region:    cluster.Region,
		Healthy:   true,
		LastCheck: time.Now(),
	}

	return nil
}

func (f *FederationManager) handleClusterLeave(msg *SyncMessage) error {
	var payload struct {
		ClusterID string `json:"cluster_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.clusters, payload.ClusterID)
	delete(f.status, payload.ClusterID)

	return nil
}

// syncLoop processes the sync queue.
func (f *FederationManager) syncLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		case msg := <-f.syncQueue:
			f.sendSync(msg)
		}
	}
}

// sendSync sends a sync message to a target cluster.
func (f *FederationManager) sendSync(msg *SyncMessage) error {
	f.mu.RLock()
	cluster, exists := f.clusters[msg.TargetCluster]
	f.mu.RUnlock()

	if !exists {
		return ErrClusterNotFound
	}

	if len(cluster.Endpoints) == 0 {
		return errors.New("no endpoints for cluster")
	}

	// Try each endpoint
	for _, endpoint := range cluster.Endpoints {
		if err := f.sendToEndpoint(endpoint, cluster.APIKey, msg); err == nil {
			return nil
		}
	}

	return errors.New("all endpoints failed")
}

func (f *FederationManager) sendToEndpoint(endpoint, apiKey string, msg *SyncMessage) error {
	// Placeholder for actual HTTP/gRPC sync
	// In production, this would POST to the cluster's sync endpoint
	return nil
}

// broadcastAsync broadcasts a message to all clusters asynchronously.
func (f *FederationManager) broadcastAsync(msg *SyncMessage) {
	f.mu.RLock()
	clusters := make([]string, 0, len(f.clusters))
	for id := range f.clusters {
		clusters = append(clusters, id)
	}
	f.mu.RUnlock()

	for _, clusterID := range clusters {
		msgCopy := *msg
		msgCopy.TargetCluster = clusterID
		f.syncQueue <- &msgCopy
	}
}

// healthCheckLoop periodically checks cluster health.
func (f *FederationManager) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(f.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.checkAllClusters()
		}
	}
}

func (f *FederationManager) checkAllClusters() {
	f.mu.RLock()
	clusters := make([]*FederatedCluster, 0, len(f.clusters))
	for _, c := range f.clusters {
		clusters = append(clusters, c)
	}
	f.mu.RUnlock()

	for _, cluster := range clusters {
		go f.checkCluster(cluster)
	}
}

func (f *FederationManager) checkCluster(cluster *FederatedCluster) {
	start := time.Now()
	status := &ClusterStatus{
		ClusterID: cluster.ID,
		Region:    cluster.Region,
		LastCheck: start,
	}

	// Placeholder for actual health check
	// In production, this would call the cluster's health endpoint
	status.Healthy = true
	status.Latency = time.Since(start)

	f.mu.Lock()
	f.status[cluster.ID] = status
	f.mu.Unlock()
}

// getTargetClusters returns clusters to sync to based on config.
func (f *FederationManager) getTargetClusters(config *JobFederationConfig) []string {
	if len(config.TargetClusters) > 0 {
		return config.TargetClusters
	}

	// Return all clusters except excluded
	excludeMap := make(map[string]bool)
	for _, id := range config.ExcludedClusters {
		excludeMap[id] = true
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	targets := make([]string, 0)
	for id := range f.clusters {
		if !excludeMap[id] {
			targets = append(targets, id)
		}
	}
	return targets
}

// SelectExecutionCluster selects the best cluster for job execution.
func (f *FederationManager) SelectExecutionCluster(jobID string) (string, error) {
	f.mu.RLock()
	fedJob, exists := f.jobs[jobID]
	if !exists {
		f.mu.RUnlock()
		return f.localClusterID, nil // Local execution for non-federated jobs
	}

	config := fedJob.FederationConfig
	f.mu.RUnlock()

	switch config.ExecutionPolicy {
	case ExecutionPolicyOwnerOnly:
		return fedJob.OwnerCluster, nil

	case ExecutionPolicyAnyCluster:
		// Return healthiest cluster
		return f.selectHealthiestCluster(append(fedJob.ReplicaClusters, fedJob.OwnerCluster))

	case ExecutionPolicyPreferred:
		// Try owner first, then replicas
		status, _ := f.GetClusterStatus(fedJob.OwnerCluster)
		if status != nil && status.Healthy {
			return fedJob.OwnerCluster, nil
		}
		return f.selectHealthiestCluster(fedJob.ReplicaClusters)

	default:
		return f.localClusterID, nil
	}
}

func (f *FederationManager) selectHealthiestCluster(clusterIDs []string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var best string
	var bestLatency time.Duration = time.Hour

	for _, id := range clusterIDs {
		status, exists := f.status[id]
		if !exists || !status.Healthy {
			continue
		}
		if status.Latency < bestLatency {
			best = id
			bestLatency = status.Latency
		}
	}

	if best == "" {
		return "", ErrNoHealthyRegions
	}
	return best, nil
}

// GetFederationStats returns federation statistics.
func (f *FederationManager) GetFederationStats() *FederationStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	healthyClusters := 0
	for _, s := range f.status {
		if s.Healthy {
			healthyClusters++
		}
	}

	return &FederationStats{
		LocalClusterID:   f.localClusterID,
		TotalClusters:    len(f.clusters),
		HealthyClusters:  healthyClusters,
		FederatedJobs:    len(f.jobs),
		PendingSyncs:     len(f.syncQueue),
		Enabled:          f.config.Enabled,
	}
}

// FederationStats contains federation statistics.
type FederationStats struct {
	LocalClusterID  string `json:"local_cluster_id"`
	TotalClusters   int    `json:"total_clusters"`
	HealthyClusters int    `json:"healthy_clusters"`
	FederatedJobs   int    `json:"federated_jobs"`
	PendingSyncs    int    `json:"pending_syncs"`
	Enabled         bool   `json:"enabled"`
}

func mustJSON(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
