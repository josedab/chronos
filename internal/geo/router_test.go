// Package geo provides multi-region router tests.
package geo

import (
	"context"
	"testing"
	"time"
)

// mockHealthChecker implements HealthChecker for testing.
type mockHealthChecker struct {
	results map[string]*RegionStatus
	err     error
}

func (m *mockHealthChecker) Check(ctx context.Context, region *Region) (*RegionStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	if status, ok := m.results[region.ID]; ok {
		return status, nil
	}
	return &RegionStatus{
		RegionID:  region.ID,
		Healthy:   true,
		LastCheck: time.Now(),
	}, nil
}

func TestNewRouter(t *testing.T) {
	cfg := DefaultConfig()
	router := NewRouter(cfg, nil)
	
	if router == nil {
		t.Fatal("expected non-nil Router")
	}
	if router.config.DefaultPolicy != RoutingPolicyPriority {
		t.Errorf("expected priority policy, got %s", router.config.DefaultPolicy)
	}
}

func TestRouter_AddRegion(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	region := &Region{
		ID:       "us-east-1",
		Name:     "US East",
		Provider: "aws",
		Enabled:  true,
		Priority: 1,
	}
	
	err := router.AddRegion(region)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Verify region was added
	retrieved, err := router.GetRegion("us-east-1")
	if err != nil {
		t.Fatalf("failed to get region: %v", err)
	}
	if retrieved.Name != "US East" {
		t.Errorf("expected name 'US East', got %s", retrieved.Name)
	}
}

func TestRouter_RemoveRegion(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	
	err := router.RemoveRegion("us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	_, err = router.GetRegion("us-east-1")
	if err != ErrRegionNotFound {
		t.Errorf("expected ErrRegionNotFound, got %v", err)
	}
}

func TestRouter_RemoveRegion_NotFound(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	err := router.RemoveRegion("nonexistent")
	if err != ErrRegionNotFound {
		t.Errorf("expected ErrRegionNotFound, got %v", err)
	}
}

func TestRouter_ListRegions(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	regions := []*Region{
		{ID: "us-east-1", Name: "US East", Enabled: true},
		{ID: "us-west-2", Name: "US West", Enabled: true},
		{ID: "eu-west-1", Name: "EU West", Enabled: true},
	}
	
	for _, r := range regions {
		_ = router.AddRegion(r)
	}
	
	list := router.ListRegions()
	if len(list) != 3 {
		t.Errorf("expected 3 regions, got %d", len(list))
	}
}

func TestRouter_GetStatus(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	
	status, err := router.GetStatus("us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.RegionID != "us-east-1" {
		t.Errorf("expected region ID 'us-east-1', got %s", status.RegionID)
	}
	if !status.Healthy {
		t.Error("expected region to be healthy by default")
	}
}

func TestRouter_GetStatus_NotFound(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_, err := router.GetStatus("nonexistent")
	if err != ErrRegionNotFound {
		t.Errorf("expected ErrRegionNotFound, got %v", err)
	}
}

func TestRouter_Route_RequiredRegion(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true, Priority: 2})
	_ = router.AddRegion(&Region{ID: "us-west-2", Enabled: true, Priority: 1})
	
	config := &JobRegionConfig{
		RequiredRegion: "us-east-1",
	}
	
	region, err := router.Route(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.ID != "us-east-1" {
		t.Errorf("expected us-east-1, got %s", region.ID)
	}
}

func TestRouter_Route_RequiredRegion_NotFound(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	
	config := &JobRegionConfig{
		RequiredRegion: "nonexistent",
	}
	
	_, err := router.Route(context.Background(), config)
	if err != ErrRegionNotFound {
		t.Errorf("expected ErrRegionNotFound, got %v", err)
	}
}

func TestRouter_Route_RequiredRegion_Unhealthy(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	
	// Mark region as unhealthy
	router.mu.Lock()
	router.status["us-east-1"].Healthy = false
	router.mu.Unlock()
	
	config := &JobRegionConfig{
		RequiredRegion: "us-east-1",
	}
	
	_, err := router.Route(context.Background(), config)
	if err != ErrRegionUnavailable {
		t.Errorf("expected ErrRegionUnavailable, got %v", err)
	}
}

func TestRouter_Route_Priority(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPolicy = RoutingPolicyPriority
	router := NewRouter(cfg, nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true, Priority: 3})
	_ = router.AddRegion(&Region{ID: "us-west-2", Enabled: true, Priority: 1}) // Highest priority
	_ = router.AddRegion(&Region{ID: "eu-west-1", Enabled: true, Priority: 2})
	
	region, err := router.Route(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.ID != "us-west-2" {
		t.Errorf("expected us-west-2 (lowest priority number), got %s", region.ID)
	}
}

func TestRouter_Route_RoundRobin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPolicy = RoutingPolicyRoundRobin
	router := NewRouter(cfg, nil)
	
	_ = router.AddRegion(&Region{ID: "region-1", Enabled: true})
	_ = router.AddRegion(&Region{ID: "region-2", Enabled: true})
	_ = router.AddRegion(&Region{ID: "region-3", Enabled: true})
	
	config := &JobRegionConfig{
		RoutingPolicy: RoutingPolicyRoundRobin,
	}
	
	// Route multiple times and check distribution
	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		region, err := router.Route(context.Background(), config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen[region.ID]++
	}
	
	// Each region should be hit at least once with round robin
	if len(seen) < 2 {
		t.Error("round robin should distribute across regions")
	}
}

func TestRouter_Route_LeastLoaded(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPolicy = RoutingPolicyLeastLoaded
	router := NewRouter(cfg, nil)
	
	_ = router.AddRegion(&Region{ID: "region-1", Enabled: true})
	_ = router.AddRegion(&Region{ID: "region-2", Enabled: true})
	_ = router.AddRegion(&Region{ID: "region-3", Enabled: true})
	
	// Set different load levels
	router.mu.Lock()
	router.status["region-1"].Capacity = 0.8
	router.status["region-2"].Capacity = 0.2 // Least loaded
	router.status["region-3"].Capacity = 0.5
	router.mu.Unlock()
	
	region, err := router.Route(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.ID != "region-2" {
		t.Errorf("expected region-2 (least loaded), got %s", region.ID)
	}
}

func TestRouter_Route_ExcludedRegions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultPolicy = RoutingPolicyPriority
	router := NewRouter(cfg, nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true, Priority: 1})
	_ = router.AddRegion(&Region{ID: "us-west-2", Enabled: true, Priority: 2})
	
	config := &JobRegionConfig{
		ExcludedRegions: []string{"us-east-1"},
	}
	
	region, err := router.Route(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.ID != "us-west-2" {
		t.Errorf("expected us-west-2 (us-east-1 excluded), got %s", region.ID)
	}
}

func TestRouter_Route_DataResidency(t *testing.T) {
	cfg := DefaultConfig()
	router := NewRouter(cfg, nil)
	
	_ = router.AddRegion(&Region{
		ID:       "us-east-1",
		Enabled:  true,
		Priority: 1,
		Location: Location{Country: "US"},
	})
	_ = router.AddRegion(&Region{
		ID:       "eu-west-1",
		Enabled:  true,
		Priority: 2,
		Location: Location{Country: "IE"},
	})
	
	config := &JobRegionConfig{
		DataResidency: []string{"IE"}, // Only Ireland
	}
	
	region, err := router.Route(context.Background(), config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.ID != "eu-west-1" {
		t.Errorf("expected eu-west-1 (Ireland), got %s", region.ID)
	}
}

func TestRouter_Route_NoHealthyRegions(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	
	// Mark all regions as unhealthy
	router.mu.Lock()
	router.status["us-east-1"].Healthy = false
	router.mu.Unlock()
	
	_, err := router.Route(context.Background(), nil)
	if err != ErrNoHealthyRegions {
		t.Errorf("expected ErrNoHealthyRegions, got %v", err)
	}
}

func TestRouter_Route_DisabledRegion(t *testing.T) {
	router := NewRouter(DefaultConfig(), nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: false}) // Disabled
	_ = router.AddRegion(&Region{ID: "us-west-2", Enabled: true})
	
	region, err := router.Route(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if region.ID != "us-west-2" {
		t.Errorf("expected us-west-2 (us-east-1 disabled), got %s", region.ID)
	}
}

func TestRouter_GetStats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LocalRegion = "us-east-1"
	router := NewRouter(cfg, nil)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	_ = router.AddRegion(&Region{ID: "us-west-2", Enabled: true})
	
	// Mark one as unhealthy
	router.mu.Lock()
	router.status["us-west-2"].Healthy = false
	router.mu.Unlock()
	
	stats := router.GetStats()
	
	if stats.TotalRegions != 2 {
		t.Errorf("expected 2 total regions, got %d", stats.TotalRegions)
	}
	if stats.HealthyRegions != 1 {
		t.Errorf("expected 1 healthy region, got %d", stats.HealthyRegions)
	}
	if stats.LocalRegion != "us-east-1" {
		t.Errorf("expected local region us-east-1, got %s", stats.LocalRegion)
	}
}

func TestRouter_HealthCheck(t *testing.T) {
	checker := &mockHealthChecker{
		results: map[string]*RegionStatus{
			"us-east-1": {
				RegionID:  "us-east-1",
				Healthy:   true,
				Latency:   10 * time.Millisecond,
				LastCheck: time.Now(),
			},
		},
	}
	
	cfg := DefaultConfig()
	cfg.HealthCheckInterval = 10 * time.Millisecond
	router := NewRouter(cfg, checker)
	
	_ = router.AddRegion(&Region{ID: "us-east-1", Enabled: true})
	
	// Manually trigger health check
	router.checkAllRegions(context.Background())
	
	status, _ := router.GetStatus("us-east-1")
	if status.Latency != 10*time.Millisecond {
		t.Errorf("expected 10ms latency, got %v", status.Latency)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.Enabled {
		t.Error("geo routing should be disabled by default")
	}
	if cfg.DefaultPolicy != RoutingPolicyPriority {
		t.Errorf("expected priority policy, got %s", cfg.DefaultPolicy)
	}
	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("expected 30s health check interval, got %v", cfg.HealthCheckInterval)
	}
	if cfg.FailoverThreshold != 3 {
		t.Errorf("expected failover threshold 3, got %d", cfg.FailoverThreshold)
	}
}

func TestRoutingPolicy_Constants(t *testing.T) {
	policies := []RoutingPolicy{
		RoutingPolicyNearest,
		RoutingPolicyPriority,
		RoutingPolicyRoundRobin,
		RoutingPolicyLeastLoaded,
		RoutingPolicyFixed,
	}
	
	for _, p := range policies {
		if p == "" {
			t.Error("routing policy should not be empty")
		}
	}
}
