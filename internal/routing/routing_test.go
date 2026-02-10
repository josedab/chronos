package routing

import (
	"context"
	"testing"
	"time"
)

func TestRouterBasic(t *testing.T) {
	router := NewRouter()

	region1 := &Region{
		ID:       "us-east-1",
		Name:     "US East",
		Provider: ProviderAWS,
		Enabled:  true,
		Endpoints: []*Endpoint{
			{ID: "ep-1", URL: "http://localhost:8081", Type: EndpointTypeHTTP, Enabled: true},
		},
		Health: &RegionHealth{
			Status:       HealthStatusHealthy,
			Availability: 1.0,
		},
	}

	err := router.RegisterRegion(region1)
	if err != nil {
		t.Fatalf("failed to register region: %v", err)
	}

	regions := router.ListRegions()
	if len(regions) != 1 {
		t.Errorf("expected 1 region, got %d", len(regions))
	}

	ctx := context.Background()
	decision, err := router.Route(ctx, "job-1", "")
	if err != nil {
		t.Fatalf("routing failed: %v", err)
	}

	if decision.SelectedRegion == nil {
		t.Fatal("expected selected region")
	}
	if decision.SelectedRegion.ID != "us-east-1" {
		t.Errorf("expected us-east-1, got %s", decision.SelectedRegion.ID)
	}
}

func TestRouterHealthBased(t *testing.T) {
	router := NewRouter()

	router.RegisterRegion(&Region{
		ID:       "healthy",
		Name:     "Healthy Region",
		Provider: ProviderAWS,
		Enabled:  true,
		Endpoints: []*Endpoint{
			{ID: "ep-1", URL: "http://localhost:8081", Type: EndpointTypeHTTP, Enabled: true},
		},
		Health: &RegionHealth{
			Status:       HealthStatusHealthy,
			Availability: 1.0,
			SuccessRate:  0.99,
		},
	})

	router.RegisterRegion(&Region{
		ID:       "unhealthy",
		Name:     "Unhealthy Region",
		Provider: ProviderAWS,
		Enabled:  true,
		Endpoints: []*Endpoint{
			{ID: "ep-2", URL: "http://localhost:8082", Type: EndpointTypeHTTP, Enabled: true},
		},
		Health: &RegionHealth{
			Status:       HealthStatusUnhealthy,
			Availability: 0.5,
			SuccessRate:  0.5,
		},
	})

	ctx := context.Background()
	healthyCount := 0
	for i := 0; i < 10; i++ {
		decision, err := router.Route(ctx, "job-1", "")
		if err != nil {
			t.Fatalf("routing failed: %v", err)
		}
		if decision.SelectedRegion.ID == "healthy" {
			healthyCount++
		}
	}

	if healthyCount < 5 {
		t.Errorf("expected healthy region selected more often, got %d/10", healthyCount)
	}
}

func TestRouterLatencyBased(t *testing.T) {
	router := NewRouter()
	policy := &RoutingPolicy{
		ID:       "latency",
		Strategy: StrategyLatencyBased,
	}
	router.SetPolicy(policy)
	router.SetDefaultPolicy(policy)

	router.RegisterRegion(&Region{
		ID:       "low-latency",
		Name:     "Low Latency",
		Provider: ProviderAWS,
		Enabled:  true,
		Endpoints: []*Endpoint{
			{ID: "ep-1", URL: "http://localhost:8081", Type: EndpointTypeHTTP, Enabled: true},
		},
		Health: &RegionHealth{
			Status:       HealthStatusHealthy,
			Availability: 1.0,
			Latency:      10 * time.Millisecond,
		},
	})

	router.RegisterRegion(&Region{
		ID:       "high-latency",
		Name:     "High Latency",
		Provider: ProviderAWS,
		Enabled:  true,
		Endpoints: []*Endpoint{
			{ID: "ep-2", URL: "http://localhost:8082", Type: EndpointTypeHTTP, Enabled: true},
		},
		Health: &RegionHealth{
			Status:       HealthStatusHealthy,
			Availability: 1.0,
			Latency:      500 * time.Millisecond,
		},
	})

	ctx := context.Background()
	decision, err := router.Route(ctx, "job-1", "latency")
	if err != nil {
		t.Fatalf("routing failed: %v", err)
	}

	if decision.SelectedRegion.ID != "low-latency" {
		t.Errorf("expected low-latency, got %s", decision.SelectedRegion.ID)
	}
}

func TestIntelligentRouter(t *testing.T) {
	config := DefaultIntelligentRouterConfig()
	config.EnableML = false

	router := NewIntelligentRouter(config)

	router.baseRouter.RegisterRegion(&Region{
		ID:       "us-east-1",
		Name:     "US East",
		Provider: ProviderAWS,
		Enabled:  true,
		Endpoints: []*Endpoint{
			{ID: "ep-1", URL: "http://localhost:8081", Type: EndpointTypeHTTP, Enabled: true},
		},
		Health: &RegionHealth{
			Status:       HealthStatusHealthy,
			Availability: 1.0,
		},
	})

	ctx := context.Background()
	result, err := router.Route(ctx, "job-1", "")
	if err != nil {
		t.Fatalf("intelligent routing failed: %v", err)
	}

	if result.SelectedTarget == "" {
		t.Fatal("expected selected target")
	}
}

func TestABExperiment(t *testing.T) {
	config := DefaultIntelligentRouterConfig()
	config.EnableABTesting = true
	config.EnableML = false

	router := NewIntelligentRouter(config)

	exp := &ABExperiment{
		ID:          "exp-1",
		Name:        "Test Experiment",
		Description: "Testing A/B",
		Variants: []*ExperimentVariant{
			{ID: "v-a", Name: "A", Strategy: StrategyHealthBased, TrafficWeight: 50},
			{ID: "v-b", Name: "B", Strategy: StrategyLatencyBased, TrafficWeight: 50},
		},
		Status:    ExperimentDraft,
		StartTime: time.Now(),
	}

	err := router.CreateExperiment(exp)
	if err != nil {
		t.Fatalf("failed to create experiment: %v", err)
	}

	experiments := router.ListExperiments()
	if len(experiments) != 1 {
		t.Errorf("expected 1 experiment, got %d", len(experiments))
	}

	retrieved, err := router.GetExperiment("exp-1")
	if err != nil {
		t.Fatalf("failed to get experiment: %v", err)
	}
	if retrieved.Name != "Test Experiment" {
		t.Errorf("expected 'Test Experiment', got %s", retrieved.Name)
	}
}

func TestRegionRemove(t *testing.T) {
	router := NewRouter()

	router.RegisterRegion(&Region{ID: "region-1", Name: "Region 1", Enabled: true})
	router.RegisterRegion(&Region{ID: "region-2", Name: "Region 2", Enabled: true})

	if len(router.ListRegions()) != 2 {
		t.Fatal("expected 2 regions")
	}

	router.UnregisterRegion("region-1")

	regions := router.ListRegions()
	if len(regions) != 1 {
		t.Errorf("expected 1 region after removal, got %d", len(regions))
	}
}

func TestRoutingStrategies(t *testing.T) {
	strategies := []RoutingStrategy{
		StrategyRoundRobin,
		StrategyWeightedRandom,
		StrategyHealthBased,
		StrategyLatencyBased,
		StrategyCostOptimized,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			router := NewRouter()
			policy := &RoutingPolicy{ID: "test", Strategy: strategy}
			router.SetPolicy(policy)
			router.SetDefaultPolicy(policy)

			router.RegisterRegion(&Region{
				ID: "region-1", Name: "Region 1", Provider: ProviderAWS, Enabled: true,
				Endpoints: []*Endpoint{{ID: "ep-1", URL: "http://localhost:8081", Type: EndpointTypeHTTP, Enabled: true}},
				Health:    &RegionHealth{Status: HealthStatusHealthy, Availability: 1.0, Latency: 10 * time.Millisecond},
				Cost:      &RegionCost{ComputePerHour: 0.10},
			})
			router.RegisterRegion(&Region{
				ID: "region-2", Name: "Region 2", Provider: ProviderGCP, Enabled: true,
				Endpoints: []*Endpoint{{ID: "ep-2", URL: "http://localhost:8082", Type: EndpointTypeHTTP, Enabled: true}},
				Health:    &RegionHealth{Status: HealthStatusHealthy, Availability: 1.0, Latency: 20 * time.Millisecond},
				Cost:      &RegionCost{ComputePerHour: 0.12},
			})

			ctx := context.Background()
			decision, err := router.Route(ctx, "job-1", "test")
			if err != nil {
				t.Fatalf("routing with %s failed: %v", strategy, err)
			}
			if decision.SelectedRegion == nil {
				t.Fatalf("expected selected region with %s strategy", strategy)
			}
		})
	}
}

func TestNoRegionsError(t *testing.T) {
	router := NewRouter()
	ctx := context.Background()
	_, err := router.Route(ctx, "job-1", "")
	if err == nil {
		t.Error("expected error for no regions")
	}
}

func TestCloudProviders(t *testing.T) {
	providers := []CloudProvider{ProviderAWS, ProviderGCP, ProviderAzure, ProviderOnPrem, ProviderCustom}
	router := NewRouter()

	for i, provider := range providers {
		router.RegisterRegion(&Region{
			ID: string(provider), Name: string(provider), Provider: provider, Enabled: true,
			Endpoints: []*Endpoint{{ID: "ep", URL: "http://localhost", Type: EndpointTypeHTTP, Enabled: true}},
			Health:    &RegionHealth{Status: HealthStatusHealthy, Availability: 1.0},
			Priority:  i,
		})
	}

	if len(router.ListRegions()) != 5 {
		t.Errorf("expected 5 regions, got %d", len(router.ListRegions()))
	}
}

func TestRoutingPolicies(t *testing.T) {
	router := NewRouter()

	policy1 := &RoutingPolicy{ID: "low-latency", Name: "Low Latency", Strategy: StrategyLatencyBased}
	policy2 := &RoutingPolicy{ID: "cost-opt", Name: "Cost Optimized", Strategy: StrategyCostOptimized}

	router.SetPolicy(policy1)
	router.SetPolicy(policy2)
	
	// Verify policies were set by using them
	router.RegisterRegion(&Region{
		ID: "region-1", Name: "Region 1", Provider: ProviderAWS, Enabled: true,
		Endpoints: []*Endpoint{{ID: "ep-1", URL: "http://localhost:8081", Type: EndpointTypeHTTP, Enabled: true}},
		Health:    &RegionHealth{Status: HealthStatusHealthy, Availability: 1.0, Latency: 10 * time.Millisecond},
		Cost:      &RegionCost{ComputePerHour: 0.10},
	})

	ctx := context.Background()
	
	// Should be able to route with both policies
	_, err := router.Route(ctx, "job-1", "low-latency")
	if err != nil {
		t.Fatalf("failed to route with low-latency policy: %v", err)
	}
	_, err = router.Route(ctx, "job-1", "cost-opt")
	if err != nil {
		t.Fatalf("failed to route with cost-opt policy: %v", err)
	}
}

func TestHealthStatuses(t *testing.T) {
	statuses := []HealthStatus{HealthStatusHealthy, HealthStatusDegraded, HealthStatusUnhealthy, HealthStatusUnknown}

	for _, status := range statuses {
		router := NewRouter()
		router.RegisterRegion(&Region{
			ID: string(status), Enabled: true,
			Endpoints: []*Endpoint{{ID: "ep", URL: "http://localhost", Type: EndpointTypeHTTP, Enabled: true}},
			Health:    &RegionHealth{Status: status, Availability: 0.5},
		})

		regions := router.ListRegions()
		if len(regions) != 1 || regions[0].Health.Status != status {
			t.Errorf("expected status %s", status)
		}
	}
}
