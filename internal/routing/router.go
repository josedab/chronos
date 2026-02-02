// Package routing provides global job routing based on geography, cost, and health.
package routing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common errors.
var (
	ErrNoAvailableRegions = errors.New("no available regions for routing")
	ErrRegionNotFound     = errors.New("region not found")
	ErrEndpointNotFound   = errors.New("endpoint not found")
	ErrRoutingFailed      = errors.New("routing failed")
)

// Region represents a geographic region.
type Region struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    CloudProvider     `json:"provider"`
	Location    GeoLocation       `json:"location"`
	Endpoints   []*Endpoint       `json:"endpoints"`
	Health      *RegionHealth     `json:"health"`
	Cost        *RegionCost       `json:"cost"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Priority    int               `json:"priority"` // Lower is higher priority
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// CloudProvider represents a cloud provider.
type CloudProvider string

const (
	ProviderAWS     CloudProvider = "aws"
	ProviderGCP     CloudProvider = "gcp"
	ProviderAzure   CloudProvider = "azure"
	ProviderOnPrem  CloudProvider = "on_prem"
	ProviderCustom  CloudProvider = "custom"
)

// GeoLocation represents a geographic location.
type GeoLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	City      string  `json:"city,omitempty"`
}

// Endpoint represents a service endpoint in a region.
type Endpoint struct {
	ID        string         `json:"id"`
	URL       string         `json:"url"`
	Type      EndpointType   `json:"type"`
	Weight    int            `json:"weight"` // For weighted routing
	Health    *EndpointHealth `json:"health"`
	Enabled   bool           `json:"enabled"`
}

// EndpointType represents the type of endpoint.
type EndpointType string

const (
	EndpointTypeHTTP    EndpointType = "http"
	EndpointTypeGRPC    EndpointType = "grpc"
	EndpointTypeKafka   EndpointType = "kafka"
	EndpointTypeNATS    EndpointType = "nats"
)

// RegionHealth contains health metrics for a region.
type RegionHealth struct {
	Status       HealthStatus  `json:"status"`
	Latency      time.Duration `json:"latency"`
	SuccessRate  float64       `json:"success_rate"`
	LastCheck    time.Time     `json:"last_check"`
	FailCount    int           `json:"fail_count"`
	Availability float64       `json:"availability"` // 0.0 - 1.0
}

// EndpointHealth contains health metrics for an endpoint.
type EndpointHealth struct {
	Status       HealthStatus  `json:"status"`
	Latency      time.Duration `json:"latency"`
	SuccessRate  float64       `json:"success_rate"`
	LastCheck    time.Time     `json:"last_check"`
	Consecutive  int           `json:"consecutive_failures"`
}

// HealthStatus represents the health status.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// RegionCost contains cost information for a region.
type RegionCost struct {
	ComputePerHour  float64       `json:"compute_per_hour"`  // USD
	NetworkPerGB    float64       `json:"network_per_gb"`    // USD
	SpotDiscount    float64       `json:"spot_discount"`     // 0.0 - 1.0
	Currency        string        `json:"currency"`
	LastUpdated     time.Time     `json:"last_updated"`
}

// RoutingPolicy defines how jobs are routed.
type RoutingPolicy struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Strategy    RoutingStrategy  `json:"strategy"`
	Preferences *RoutePreferences `json:"preferences,omitempty"`
	Constraints *RouteConstraints `json:"constraints,omitempty"`
	Failover    *FailoverConfig  `json:"failover,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}

// RoutingStrategy defines the routing strategy.
type RoutingStrategy string

const (
	StrategyLatencyBased    RoutingStrategy = "latency"       // Route to lowest latency
	StrategyCostOptimized   RoutingStrategy = "cost"          // Route to cheapest
	StrategyGeoAffinity     RoutingStrategy = "geo_affinity"  // Route to closest
	StrategyWeightedRandom  RoutingStrategy = "weighted"      // Weighted random
	StrategyRoundRobin      RoutingStrategy = "round_robin"   // Sequential
	StrategyFailover        RoutingStrategy = "failover"      // Primary with failover
	StrategyHealthBased     RoutingStrategy = "health"        // Route to healthiest
)

// RoutePreferences contains routing preferences.
type RoutePreferences struct {
	PreferredRegions  []string `json:"preferred_regions,omitempty"`
	PreferredProviders []CloudProvider `json:"preferred_providers,omitempty"`
	PreferSpot        bool     `json:"prefer_spot"`
	MaxLatency        time.Duration `json:"max_latency,omitempty"`
	MaxCost           float64  `json:"max_cost,omitempty"`
}

// RouteConstraints contains routing constraints.
type RouteConstraints struct {
	ExcludedRegions   []string `json:"excluded_regions,omitempty"`
	ExcludedProviders []CloudProvider `json:"excluded_providers,omitempty"`
	RequiredCountries []string `json:"required_countries,omitempty"` // Data residency
	MinAvailability   float64  `json:"min_availability,omitempty"`
}

// FailoverConfig contains failover configuration.
type FailoverConfig struct {
	Enabled           bool          `json:"enabled"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	CircuitBreaker    *CircuitBreaker `json:"circuit_breaker,omitempty"`
}

// CircuitBreaker configuration.
type CircuitBreaker struct {
	FailureThreshold int           `json:"failure_threshold"`
	ResetTimeout     time.Duration `json:"reset_timeout"`
	HalfOpenRequests int           `json:"half_open_requests"`
}

// RouteDecision contains the routing decision.
type RouteDecision struct {
	ID              string        `json:"id"`
	JobID           string        `json:"job_id"`
	SelectedRegion  *Region       `json:"selected_region"`
	SelectedEndpoint *Endpoint    `json:"selected_endpoint"`
	Reason          string        `json:"reason"`
	Alternatives    []*Region     `json:"alternatives,omitempty"`
	Score           float64       `json:"score"`
	DecidedAt       time.Time     `json:"decided_at"`
}

// Router handles global job routing.
type Router struct {
	mu            sync.RWMutex
	regions       map[string]*Region
	policies      map[string]*RoutingPolicy
	defaultPolicy *RoutingPolicy
	clientLoc     *GeoLocation // Client/source location for geo calculations
	roundRobinIdx int
}

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{
		regions:  make(map[string]*Region),
		policies: make(map[string]*RoutingPolicy),
		defaultPolicy: &RoutingPolicy{
			ID:       "default",
			Name:     "Default Policy",
			Strategy: StrategyHealthBased,
			Failover: &FailoverConfig{
				Enabled:    true,
				MaxRetries: 2,
				RetryDelay: time.Second * 5,
			},
		},
	}
}

// RegisterRegion registers a new region.
func (r *Router) RegisterRegion(region *Region) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if region.ID == "" {
		region.ID = uuid.New().String()
	}
	region.CreatedAt = time.Now().UTC()
	region.UpdatedAt = region.CreatedAt

	if region.Health == nil {
		region.Health = &RegionHealth{
			Status:       HealthStatusUnknown,
			Availability: 1.0,
		}
	}

	r.regions[region.ID] = region
	return nil
}

// UnregisterRegion removes a region.
func (r *Router) UnregisterRegion(regionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.regions[regionID]; !exists {
		return ErrRegionNotFound
	}

	delete(r.regions, regionID)
	return nil
}

// GetRegion retrieves a region by ID.
func (r *Router) GetRegion(regionID string) (*Region, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	region, exists := r.regions[regionID]
	if !exists {
		return nil, ErrRegionNotFound
	}
	return region, nil
}

// ListRegions returns all registered regions.
func (r *Router) ListRegions() []*Region {
	r.mu.RLock()
	defer r.mu.RUnlock()

	regions := make([]*Region, 0, len(r.regions))
	for _, region := range r.regions {
		regions = append(regions, region)
	}
	return regions
}

// SetPolicy sets a routing policy.
func (r *Router) SetPolicy(policy *RoutingPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}
	policy.CreatedAt = time.Now().UTC()
	r.policies[policy.ID] = policy
}

// SetDefaultPolicy sets the default routing policy.
func (r *Router) SetDefaultPolicy(policy *RoutingPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultPolicy = policy
}

// SetClientLocation sets the client location for geo-based routing.
func (r *Router) SetClientLocation(loc *GeoLocation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientLoc = loc
}

// Route determines the best region for a job execution.
func (r *Router) Route(ctx context.Context, jobID string, policyID string) (*RouteDecision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get policy
	policy := r.defaultPolicy
	if policyID != "" {
		if p, exists := r.policies[policyID]; exists {
			policy = p
		}
	}

	// Get available regions
	available := r.getAvailableRegions(policy)
	if len(available) == 0 {
		return nil, ErrNoAvailableRegions
	}

	// Score and sort regions based on strategy
	scored := r.scoreRegions(available, policy)

	decision := &RouteDecision{
		ID:              uuid.New().String(),
		JobID:           jobID,
		SelectedRegion:  scored[0].region,
		Score:           scored[0].score,
		Reason:          r.generateReason(scored[0], policy),
		Alternatives:    make([]*Region, 0),
		DecidedAt:       time.Now().UTC(),
	}

	// Add alternatives
	for i := 1; i < len(scored) && i < 3; i++ {
		decision.Alternatives = append(decision.Alternatives, scored[i].region)
	}

	// Select endpoint within region
	if len(scored[0].region.Endpoints) > 0 {
		decision.SelectedEndpoint = r.selectEndpoint(scored[0].region, policy)
	}

	return decision, nil
}

type scoredRegion struct {
	region *Region
	score  float64
}

func (r *Router) getAvailableRegions(policy *RoutingPolicy) []*Region {
	available := make([]*Region, 0)

	for _, region := range r.regions {
		if !region.Enabled {
			continue
		}

		// Check health
		if region.Health != nil && region.Health.Status == HealthStatusUnhealthy {
			continue
		}

		// Apply constraints
		if policy.Constraints != nil {
			if contains(policy.Constraints.ExcludedRegions, region.ID) {
				continue
			}
			if containsProvider(policy.Constraints.ExcludedProviders, region.Provider) {
				continue
			}
			if len(policy.Constraints.RequiredCountries) > 0 {
				if !contains(policy.Constraints.RequiredCountries, region.Location.Country) {
					continue
				}
			}
			if policy.Constraints.MinAvailability > 0 && region.Health != nil {
				if region.Health.Availability < policy.Constraints.MinAvailability {
					continue
				}
			}
		}

		available = append(available, region)
	}

	return available
}

func (r *Router) scoreRegions(regions []*Region, policy *RoutingPolicy) []scoredRegion {
	scored := make([]scoredRegion, len(regions))

	for i, region := range regions {
		score := r.calculateScore(region, policy)
		scored[i] = scoredRegion{region: region, score: score}
	}

	// Sort by score descending (higher is better)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored
}

func (r *Router) calculateScore(region *Region, policy *RoutingPolicy) float64 {
	var score float64 = 100.0

	switch policy.Strategy {
	case StrategyLatencyBased:
		if region.Health != nil {
			// Lower latency = higher score
			latencyMs := float64(region.Health.Latency.Milliseconds())
			score = 100.0 - (latencyMs / 10.0) // Lose 0.1 point per ms
		}

	case StrategyCostOptimized:
		if region.Cost != nil {
			// Lower cost = higher score
			score = 100.0 - (region.Cost.ComputePerHour * 10.0)
			if policy.Preferences != nil && policy.Preferences.PreferSpot && region.Cost.SpotDiscount > 0 {
				score += region.Cost.SpotDiscount * 20.0
			}
		}

	case StrategyGeoAffinity:
		if r.clientLoc != nil {
			distance := haversineDistance(
				r.clientLoc.Latitude, r.clientLoc.Longitude,
				region.Location.Latitude, region.Location.Longitude,
			)
			// Closer = higher score (max distance ~20000 km)
			score = 100.0 - (distance / 200.0)
		}

	case StrategyHealthBased:
		if region.Health != nil {
			score = region.Health.Availability * 50.0
			score += region.Health.SuccessRate * 30.0
			if region.Health.Status == HealthStatusHealthy {
				score += 20.0
			} else if region.Health.Status == HealthStatusDegraded {
				score += 10.0
			}
		}

	case StrategyWeightedRandom:
		// Use priority as weight
		score = float64(100 - region.Priority)

	case StrategyRoundRobin:
		// All equal, will be rotated
		score = 100.0

	case StrategyFailover:
		// Primary regions get higher score
		score = float64(100 - region.Priority*10)
	}

	// Apply preference boosts
	if policy.Preferences != nil {
		if contains(policy.Preferences.PreferredRegions, region.ID) {
			score += 20.0
		}
		if containsProvider(policy.Preferences.PreferredProviders, region.Provider) {
			score += 10.0
		}
	}

	// Penalize for degraded health
	if region.Health != nil {
		if region.Health.Status == HealthStatusDegraded {
			score *= 0.8
		}
		if region.Health.FailCount > 0 {
			score *= (1.0 - float64(region.Health.FailCount)*0.05)
		}
	}

	return score
}

func (r *Router) selectEndpoint(region *Region, policy *RoutingPolicy) *Endpoint {
	var available []*Endpoint
	for _, ep := range region.Endpoints {
		if !ep.Enabled {
			continue
		}
		if ep.Health != nil && ep.Health.Status == HealthStatusUnhealthy {
			continue
		}
		available = append(available, ep)
	}

	if len(available) == 0 {
		return nil
	}

	// Weighted selection based on endpoint weight
	totalWeight := 0
	for _, ep := range available {
		weight := ep.Weight
		if weight == 0 {
			weight = 1
		}
		totalWeight += weight
	}

	// For now, return the healthiest endpoint
	sort.Slice(available, func(i, j int) bool {
		if available[i].Health == nil {
			return false
		}
		if available[j].Health == nil {
			return true
		}
		return available[i].Health.SuccessRate > available[j].Health.SuccessRate
	})

	return available[0]
}

func (r *Router) generateReason(scored scoredRegion, policy *RoutingPolicy) string {
	switch policy.Strategy {
	case StrategyLatencyBased:
		if scored.region.Health != nil {
			return "Selected for lowest latency (" + scored.region.Health.Latency.String() + ")"
		}
		return "Selected for lowest latency"
	case StrategyCostOptimized:
		if scored.region.Cost != nil {
			return "Selected for lowest cost ($" + formatFloat(scored.region.Cost.ComputePerHour, 4) + "/hr)"
		}
		return "Selected for lowest cost"
	case StrategyGeoAffinity:
		return "Selected for closest geographic location (" + scored.region.Location.Country + ")"
	case StrategyHealthBased:
		return "Selected for best health metrics (availability: " + formatFloat(scored.region.Health.Availability*100, 1) + "%)"
	case StrategyFailover:
		return "Selected as primary region (priority: " + string(rune('0'+scored.region.Priority)) + ")"
	default:
		return "Selected based on routing policy"
	}
}

// UpdateRegionHealth updates health metrics for a region.
func (r *Router) UpdateRegionHealth(regionID string, health *RegionHealth) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	region, exists := r.regions[regionID]
	if !exists {
		return ErrRegionNotFound
	}

	region.Health = health
	region.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateEndpointHealth updates health metrics for an endpoint.
func (r *Router) UpdateEndpointHealth(regionID, endpointID string, health *EndpointHealth) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	region, exists := r.regions[regionID]
	if !exists {
		return ErrRegionNotFound
	}

	for _, ep := range region.Endpoints {
		if ep.ID == endpointID {
			ep.Health = health
			region.UpdatedAt = time.Now().UTC()
			return nil
		}
	}

	return ErrEndpointNotFound
}

// RecordExecution records an execution result for health tracking.
func (r *Router) RecordExecution(regionID string, success bool, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	region, exists := r.regions[regionID]
	if !exists {
		return
	}

	if region.Health == nil {
		region.Health = &RegionHealth{
			Status:       HealthStatusUnknown,
			Availability: 1.0,
			SuccessRate:  1.0,
		}
	}

	// Update rolling averages
	health := region.Health
	health.LastCheck = time.Now().UTC()

	// Exponential moving average for latency
	if health.Latency == 0 {
		health.Latency = latency
	} else {
		alpha := 0.2
		health.Latency = time.Duration(alpha*float64(latency) + (1-alpha)*float64(health.Latency))
	}

	// Update success rate
	if success {
		health.SuccessRate = 0.95*health.SuccessRate + 0.05*1.0
		health.FailCount = 0
	} else {
		health.SuccessRate = 0.95*health.SuccessRate + 0.05*0.0
		health.FailCount++
	}

	// Update status based on metrics
	if health.SuccessRate >= 0.99 {
		health.Status = HealthStatusHealthy
	} else if health.SuccessRate >= 0.90 {
		health.Status = HealthStatusDegraded
	} else {
		health.Status = HealthStatusUnhealthy
	}

	region.UpdatedAt = time.Now().UTC()
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsProvider(slice []CloudProvider, item CloudProvider) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth's radius in km

	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0

	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func formatFloat(f float64, decimals int) string {
	return fmt.Sprintf("%.*f", decimals, f)
}
