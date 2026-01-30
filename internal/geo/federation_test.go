// Package geo provides cross-region federation tests.
package geo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFederationManager(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.LocalClusterID = "test-cluster"
	
	fm := NewFederationManager(cfg)
	
	if fm == nil {
		t.Fatal("expected non-nil FederationManager")
	}
	if fm.localClusterID != "test-cluster" {
		t.Errorf("expected localClusterID 'test-cluster', got %s", fm.localClusterID)
	}
}

func TestNewFederationManager_GeneratesID(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.LocalClusterID = "" // empty ID should be auto-generated
	
	fm := NewFederationManager(cfg)
	
	if fm.localClusterID == "" {
		t.Error("expected auto-generated cluster ID")
	}
}

func TestFederationManager_AddCluster(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	cluster := &FederatedCluster{
		ID:       "cluster-1",
		Name:     "Test Cluster",
		Region:   "us-east-1",
		Endpoints: []string{"https://cluster1.example.com"},
		Enabled:  true,
	}
	
	err := fm.AddCluster(cluster)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify cluster was added
	retrieved, err := fm.GetCluster("cluster-1")
	if err != nil {
		t.Fatalf("failed to get cluster: %v", err)
	}
	if retrieved.Name != "Test Cluster" {
		t.Errorf("expected name 'Test Cluster', got %s", retrieved.Name)
	}
}

func TestFederationManager_AddCluster_Disabled(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = false
	fm := NewFederationManager(cfg)
	
	cluster := &FederatedCluster{ID: "cluster-1"}
	
	err := fm.AddCluster(cluster)
	if err != ErrFederationDisabled {
		t.Errorf("expected ErrFederationDisabled, got %v", err)
	}
}

func TestFederationManager_RemoveCluster(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	cluster := &FederatedCluster{
		ID:   "cluster-1",
		Name: "Test Cluster",
	}
	
	_ = fm.AddCluster(cluster)
	
	err := fm.RemoveCluster("cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	_, err = fm.GetCluster("cluster-1")
	if err != ErrClusterNotFound {
		t.Errorf("expected ErrClusterNotFound, got %v", err)
	}
}

func TestFederationManager_RemoveCluster_NotFound(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	err := fm.RemoveCluster("nonexistent")
	if err != ErrClusterNotFound {
		t.Errorf("expected ErrClusterNotFound, got %v", err)
	}
}

func TestFederationManager_ListClusters(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	clusters := []*FederatedCluster{
		{ID: "cluster-1", Name: "Cluster 1", Priority: 2},
		{ID: "cluster-2", Name: "Cluster 2", Priority: 1},
		{ID: "cluster-3", Name: "Cluster 3", Priority: 3},
	}
	
	for _, c := range clusters {
		_ = fm.AddCluster(c)
	}
	
	list := fm.ListClusters()
	if len(list) != 3 {
		t.Errorf("expected 3 clusters, got %d", len(list))
	}
	
	// Should be sorted by priority
	if list[0].Priority > list[1].Priority || list[1].Priority > list[2].Priority {
		t.Error("clusters should be sorted by priority")
	}
}

func TestFederationManager_GetClusterStatus(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	cluster := &FederatedCluster{
		ID:     "cluster-1",
		Region: "us-east-1",
	}
	_ = fm.AddCluster(cluster)
	
	status, err := fm.GetClusterStatus("cluster-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.ClusterID != "cluster-1" {
		t.Errorf("expected cluster ID 'cluster-1', got %s", status.ClusterID)
	}
	if !status.Healthy {
		t.Error("expected cluster to be healthy by default")
	}
}

func TestFederationManager_FederateJob(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	// Add target clusters
	_ = fm.AddCluster(&FederatedCluster{ID: "cluster-1"})
	_ = fm.AddCluster(&FederatedCluster{ID: "cluster-2"})
	
	config := &JobFederationConfig{
		Enabled:         true,
		Mode:            FederationModeReplicate,
		TargetClusters:  []string{"cluster-1", "cluster-2"},
		ExecutionPolicy: ExecutionPolicyOwnerOnly,
	}
	
	err := fm.FederateJob("job-1", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify job was federated
	fm.mu.RLock()
	fedJob, exists := fm.jobs["job-1"]
	fm.mu.RUnlock()
	
	if !exists {
		t.Fatal("job should be in federated jobs map")
	}
	if fedJob.OwnerCluster != fm.localClusterID {
		t.Errorf("expected owner cluster %s, got %s", fm.localClusterID, fedJob.OwnerCluster)
	}
}

func TestFederationManager_FederateJob_Disabled(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = false
	fm := NewFederationManager(cfg)
	
	err := fm.FederateJob("job-1", &JobFederationConfig{})
	if err != ErrFederationDisabled {
		t.Errorf("expected ErrFederationDisabled, got %v", err)
	}
}

func TestFederationManager_SyncJob(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	// Add a federated job
	fm.mu.Lock()
	fm.jobs["job-1"] = &FederatedJob{
		JobID:           "job-1",
		OwnerCluster:    fm.localClusterID,
		ReplicaClusters: []string{"cluster-1"},
		Version:         1,
	}
	fm.mu.Unlock()
	
	// Add target cluster
	_ = fm.AddCluster(&FederatedCluster{ID: "cluster-1"})
	
	err := fm.SyncJob("job-1", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify version was incremented
	fm.mu.RLock()
	fedJob := fm.jobs["job-1"]
	fm.mu.RUnlock()
	
	if fedJob.Version != 2 {
		t.Errorf("expected version 2, got %d", fedJob.Version)
	}
}

func TestFederationManager_SyncJob_NotFederated(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	// SyncJob on non-federated job should be a no-op
	err := fm.SyncJob("nonexistent-job", nil)
	if err != nil {
		t.Errorf("expected nil error for non-federated job, got %v", err)
	}
}

func TestFederationManager_DeleteFederatedJob(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	// Add a federated job
	fm.mu.Lock()
	fm.jobs["job-1"] = &FederatedJob{
		JobID:           "job-1",
		ReplicaClusters: []string{},
	}
	fm.mu.Unlock()
	
	err := fm.DeleteFederatedJob("job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify job was removed
	fm.mu.RLock()
	_, exists := fm.jobs["job-1"]
	fm.mu.RUnlock()
	
	if exists {
		t.Error("job should have been removed")
	}
}

func TestFederationManager_SelectExecutionCluster_OwnerOnly(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	fm.mu.Lock()
	fm.jobs["job-1"] = &FederatedJob{
		JobID:        "job-1",
		OwnerCluster: "owner-cluster",
		FederationConfig: &JobFederationConfig{
			ExecutionPolicy: ExecutionPolicyOwnerOnly,
		},
	}
	fm.mu.Unlock()
	
	clusterID, err := fm.SelectExecutionCluster("job-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clusterID != "owner-cluster" {
		t.Errorf("expected owner-cluster, got %s", clusterID)
	}
}

func TestFederationManager_SelectExecutionCluster_NonFederated(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	cfg.LocalClusterID = "local-cluster"
	fm := NewFederationManager(cfg)
	
	clusterID, err := fm.SelectExecutionCluster("non-federated-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clusterID != "local-cluster" {
		t.Errorf("expected local-cluster, got %s", clusterID)
	}
}

func TestFederationManager_GetFederationStats(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	cfg.LocalClusterID = "local-cluster"
	fm := NewFederationManager(cfg)
	
	// Add clusters and jobs
	_ = fm.AddCluster(&FederatedCluster{ID: "cluster-1"})
	_ = fm.AddCluster(&FederatedCluster{ID: "cluster-2"})
	
	fm.mu.Lock()
	fm.jobs["job-1"] = &FederatedJob{JobID: "job-1"}
	fm.mu.Unlock()
	
	stats := fm.GetFederationStats()
	
	if stats.TotalClusters != 2 {
		t.Errorf("expected 2 clusters, got %d", stats.TotalClusters)
	}
	if stats.HealthyClusters != 2 {
		t.Errorf("expected 2 healthy clusters, got %d", stats.HealthyClusters)
	}
	if stats.FederatedJobs != 1 {
		t.Errorf("expected 1 federated job, got %d", stats.FederatedJobs)
	}
	if stats.LocalClusterID != "local-cluster" {
		t.Errorf("expected local-cluster, got %s", stats.LocalClusterID)
	}
}

func TestFederationManager_ReceiveSync_UnauthorizedCluster(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	msg := &SyncMessage{
		SourceCluster: "unknown-cluster",
		Type:          SyncTypeHeartbeat,
	}
	
	err := fm.ReceiveSync(msg)
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestFederationManager_ReceiveSync_ClusterJoin(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	// Add source cluster first
	_ = fm.AddCluster(&FederatedCluster{ID: "source-cluster"})
	
	msg := &SyncMessage{
		SourceCluster: "source-cluster",
		Type:          SyncTypeClusterJoin,
		Payload:       mustJSON(&FederatedCluster{ID: "new-cluster", Region: "us-west-2"}),
	}
	
	err := fm.ReceiveSync(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify new cluster was added
	_, err = fm.GetCluster("new-cluster")
	if err != nil {
		t.Errorf("new cluster should have been added: %v", err)
	}
}

func TestFederationManager_StartStop(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	fm := NewFederationManager(cfg)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	err := fm.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Give goroutines time to start
	time.Sleep(10 * time.Millisecond)
	
	// Stop should not panic
	cancel()
	fm.Stop()
}

func TestFederationManager_Start_Disabled(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = false
	fm := NewFederationManager(cfg)
	
	err := fm.Start(context.Background())
	if err != nil {
		t.Errorf("expected nil error when disabled, got %v", err)
	}
}

func TestDefaultFederationConfig(t *testing.T) {
	cfg := DefaultFederationConfig()
	
	if cfg.Enabled {
		t.Error("federation should be disabled by default")
	}
	if cfg.SyncInterval != 30*time.Second {
		t.Errorf("expected 30s sync interval, got %v", cfg.SyncInterval)
	}
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}
}

func TestConflictResolution_Newest(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.Enabled = true
	cfg.ConflictResolution = ConflictResolutionNewest
	fm := NewFederationManager(cfg)
	
	// Add source cluster
	_ = fm.AddCluster(&FederatedCluster{ID: "source-cluster"})
	
	// Add existing job with older timestamp
	oldTime := time.Now().Add(-1 * time.Hour)
	fm.mu.Lock()
	fm.jobs["job-1"] = &FederatedJob{
		JobID:        "job-1",
		Version:      5,
		LastSyncedAt: oldTime,
	}
	fm.mu.Unlock()
	
	// Receive update with same version but newer timestamp
	msg := &SyncMessage{
		SourceCluster: "source-cluster",
		Type:          SyncTypeJobUpdate,
		Timestamp:     time.Now(),
		Payload: mustJSON(map[string]interface{}{
			"job":  map[string]string{},
			"meta": FederatedJob{JobID: "job-1", Version: 5},
		}),
	}
	
	err := fm.ReceiveSync(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildTLSConfig_Default(t *testing.T) {
	cfg := DefaultFederationConfig()
	
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("expected non-nil TLS config")
	}
	if tlsConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false by default")
	}
	if tlsConfig.MinVersion != 0x0303 { // TLS 1.2
		t.Errorf("expected TLS 1.2 minimum, got %x", tlsConfig.MinVersion)
	}
}

func TestBuildTLSConfig_WithCACert(t *testing.T) {
	// Create a temporary CA certificate file for testing
	tmpDir, err := os.MkdirTemp("", "tls-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// A valid self-signed test certificate
	testCACert := `-----BEGIN CERTIFICATE-----
MIIC/zCCAeegAwIBAgIURYvPq9l46rujODuYrVNg+bqGC10wDQYJKoZIhvcNAQEL
BQAwDzENMAsGA1UEAwwEdGVzdDAeFw0yNjAxMjgyMTI2MjVaFw0yNjAxMjkyMTI2
MjVaMA8xDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQCzOE+Y4ypYfMQMfjdr0NSh8Yl9wdr+SXu72MZ0qrWbeofVCJhzTLU4hYLp
TyyeqsxwUMH6EYs9DD/Ti9fJgfEohOFlsrucQbDFQGW89DZf8k3Zx8Hv9qcwywyh
PHXlh1Yk3i/q/WTCooRoJxIV8BLWOg0zHHd4y7GInrW4OxU1QgkHn4HPn9aUniAO
sNctt7EQjqyKu5yQswTmEmHkj9RFSRDbGfTowp29xmm6bqcXALpxJQL2yZK4hs+U
N2Am9/WN127ihiaVtnV2yLc1AAujItpLlpofWiQXAlRXfxaPdRrvCfHvyty10HYJ
+xFn48xqT+QTN4Qrjkm+z8LBiqEdAgMBAAGjUzBRMB0GA1UdDgQWBBRZHZSOZDfh
h9TbhHG4/3X4Y1VGDjAfBgNVHSMEGDAWgBRZHZSOZDfhh9TbhHG4/3X4Y1VGDjAP
BgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAcjgEeXNRBaXcV7HT7
PuWNcHxQ9bBZVZ59F7g04FgECGYExZbDxsZ1wCyMFNGxOHwogfmv5DFKPduPihmu
y7ImtiTA9+a+WbNzhUqQOLfJHiohB33ykSWtPBgPApby0DxVwO9CskMVPgWFqlmC
jZnB31Q5380jF6LtULZVTZSo+H5tITvS5DRP6jLUCx7bLdO08PUJsJ0oWPlwjS0u
MBk3lJW223l1OSs3RxXn6O1+cdwzaDAO1B140wQOSRuAdoXAvBbFYL8Czuoem0b7
nlLHdVUuLY8Vrbw+bV206ljOygdN5x49hm7jCa2EBPLMepgZ5KkJJ/2WhCDf8hKY
jAD/
-----END CERTIFICATE-----`
	
	certFile := filepath.Join(tmpDir, "ca.crt")
	if err := os.WriteFile(certFile, []byte(testCACert), 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	
	cfg := DefaultFederationConfig()
	cfg.CACertFile = certFile
	
	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tlsConfig.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
}

func TestBuildTLSConfig_InvalidCACertFile(t *testing.T) {
	cfg := DefaultFederationConfig()
	cfg.CACertFile = "/nonexistent/ca.crt"
	
	_, err := buildTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for nonexistent CA cert file")
	}
}

func TestBuildTLSConfig_InvalidCACertContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tls-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	certFile := filepath.Join(tmpDir, "invalid.crt")
	if err := os.WriteFile(certFile, []byte("not a valid certificate"), 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	
	cfg := DefaultFederationConfig()
	cfg.CACertFile = certFile
	
	_, err = buildTLSConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid CA cert content")
	}
}

func TestNewFederationManager_WithCACert(t *testing.T) {
	// Create a temporary CA certificate file for testing
	tmpDir, err := os.MkdirTemp("", "tls-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	testCACert := `-----BEGIN CERTIFICATE-----
MIIC/zCCAeegAwIBAgIURYvPq9l46rujODuYrVNg+bqGC10wDQYJKoZIhvcNAQEL
BQAwDzENMAsGA1UEAwwEdGVzdDAeFw0yNjAxMjgyMTI2MjVaFw0yNjAxMjkyMTI2
MjVaMA8xDTALBgNVBAMMBHRlc3QwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQCzOE+Y4ypYfMQMfjdr0NSh8Yl9wdr+SXu72MZ0qrWbeofVCJhzTLU4hYLp
TyyeqsxwUMH6EYs9DD/Ti9fJgfEohOFlsrucQbDFQGW89DZf8k3Zx8Hv9qcwywyh
PHXlh1Yk3i/q/WTCooRoJxIV8BLWOg0zHHd4y7GInrW4OxU1QgkHn4HPn9aUniAO
sNctt7EQjqyKu5yQswTmEmHkj9RFSRDbGfTowp29xmm6bqcXALpxJQL2yZK4hs+U
N2Am9/WN127ihiaVtnV2yLc1AAujItpLlpofWiQXAlRXfxaPdRrvCfHvyty10HYJ
+xFn48xqT+QTN4Qrjkm+z8LBiqEdAgMBAAGjUzBRMB0GA1UdDgQWBBRZHZSOZDfh
h9TbhHG4/3X4Y1VGDjAfBgNVHSMEGDAWgBRZHZSOZDfhh9TbhHG4/3X4Y1VGDjAP
BgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAcjgEeXNRBaXcV7HT7
PuWNcHxQ9bBZVZ59F7g04FgECGYExZbDxsZ1wCyMFNGxOHwogfmv5DFKPduPihmu
y7ImtiTA9+a+WbNzhUqQOLfJHiohB33ykSWtPBgPApby0DxVwO9CskMVPgWFqlmC
jZnB31Q5380jF6LtULZVTZSo+H5tITvS5DRP6jLUCx7bLdO08PUJsJ0oWPlwjS0u
MBk3lJW223l1OSs3RxXn6O1+cdwzaDAO1B140wQOSRuAdoXAvBbFYL8Czuoem0b7
nlLHdVUuLY8Vrbw+bV206ljOygdN5x49hm7jCa2EBPLMepgZ5KkJJ/2WhCDf8hKY
jAD/
-----END CERTIFICATE-----`
	
	certFile := filepath.Join(tmpDir, "ca.crt")
	if err := os.WriteFile(certFile, []byte(testCACert), 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	
	cfg := DefaultFederationConfig()
	cfg.CACertFile = certFile
	
	fm := NewFederationManager(cfg)
	if fm == nil {
		t.Fatal("expected non-nil FederationManager")
	}
}
