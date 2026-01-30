// Package geo provides multi-region cluster support for Chronos.
package geo

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Common errors.
var (
	ErrRegionNotFound     = errors.New("region not found")
	ErrNoHealthyRegions   = errors.New("no healthy regions available")
	ErrRegionUnavailable  = errors.New("region is unavailable")
)

// Region represents a geographic region.
type Region struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Provider    string            `json:"provider"` // aws, gcp, azure
	Location    Location          `json:"location"`
	Endpoints   []string          `json:"endpoints"`
	Priority    int               `json:"priority"`   // Lower is higher priority
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Location represents geographic coordinates.
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city,omitempty"`
	Country   string  `json:"country,omitempty"`
	Zone      string  `json:"zone,omitempty"` // e.g., us-east-1a
}

// RegionStatus represents the health status of a region.
type RegionStatus struct {
	RegionID    string        `json:"region_id"`
	Healthy     bool          `json:"healthy"`
	Latency     time.Duration `json:"latency"`
	LastCheck   time.Time     `json:"last_check"`
	Error       string        `json:"error,omitempty"`
	ActiveJobs  int           `json:"active_jobs"`
	Capacity    float64       `json:"capacity"` // 0.0-1.0
}

// RoutingPolicy determines how jobs are routed to regions.
type RoutingPolicy string

const (
	// RoutingPolicyNearest routes to the nearest healthy region.
	RoutingPolicyNearest RoutingPolicy = "nearest"
	// RoutingPolicyPriority routes to the highest priority healthy region.
	RoutingPolicyPriority RoutingPolicy = "priority"
	// RoutingPolicyRoundRobin distributes across regions.
	RoutingPolicyRoundRobin RoutingPolicy = "round_robin"
	// RoutingPolicyLeastLoaded routes to the least loaded region.
	RoutingPolicyLeastLoaded RoutingPolicy = "least_loaded"
	// RoutingPolicyFixed routes to a specific region.
	RoutingPolicyFixed RoutingPolicy = "fixed"
)

// JobRegionConfig specifies region preferences for a job.
type JobRegionConfig struct {
	// PreferredRegions is a list of preferred region IDs in order.
	PreferredRegions []string `json:"preferred_regions,omitempty"`
	// ExcludedRegions is a list of excluded region IDs.
	ExcludedRegions []string `json:"excluded_regions,omitempty"`
	// RequiredRegion forces execution in a specific region.
	RequiredRegion string `json:"required_region,omitempty"`
	// RoutingPolicy overrides the default routing policy.
	RoutingPolicy RoutingPolicy `json:"routing_policy,omitempty"`
	// DataResidency enforces data to stay in specific countries.
	DataResidency []string `json:"data_residency,omitempty"`
}

// Config configures the geo-distributed scheduler.
type Config struct {
	// Enabled enables multi-region support.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// LocalRegion is the current region ID.
	LocalRegion string `json:"local_region" yaml:"local_region"`
	// DefaultPolicy is the default routing policy.
	DefaultPolicy RoutingPolicy `json:"default_policy" yaml:"default_policy"`
	// HealthCheckInterval is how often to check region health.
	HealthCheckInterval time.Duration `json:"health_check_interval" yaml:"health_check_interval"`
	// FailoverThreshold is the number of failed checks before failover.
	FailoverThreshold int `json:"failover_threshold" yaml:"failover_threshold"`
}

// DefaultConfig returns default geo configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:             false,
		DefaultPolicy:       RoutingPolicyPriority,
		HealthCheckInterval: 30 * time.Second,
		FailoverThreshold:   3,
	}
}

// Router routes jobs to appropriate regions.
type Router struct {
	mu            sync.RWMutex
	config        Config
	regions       map[string]*Region
	status        map[string]*RegionStatus
	rrIndex       int // Round-robin index
	healthChecker HealthChecker
}

// HealthChecker checks region health.
type HealthChecker interface {
	Check(ctx context.Context, region *Region) (*RegionStatus, error)
}

// NewRouter creates a new geo router.
func NewRouter(cfg Config, checker HealthChecker) *Router {
	return &Router{
		config:        cfg,
		regions:       make(map[string]*Region),
		status:        make(map[string]*RegionStatus),
		healthChecker: checker,
	}
}

// AddRegion adds a region to the router.
func (r *Router) AddRegion(region *Region) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.regions[region.ID] = region
	r.status[region.ID] = &RegionStatus{
		RegionID:  region.ID,
		Healthy:   true,
		LastCheck: time.Now(),
	}

	return nil
}

// RemoveRegion removes a region.
func (r *Router) RemoveRegion(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.regions[id]; !exists {
		return ErrRegionNotFound
	}

	delete(r.regions, id)
	delete(r.status, id)
	return nil
}

// GetRegion returns a region by ID.
func (r *Router) GetRegion(id string) (*Region, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	region, exists := r.regions[id]
	if !exists {
		return nil, ErrRegionNotFound
	}
	return region, nil
}

// ListRegions returns all regions.
func (r *Router) ListRegions() []*Region {
	r.mu.RLock()
	defer r.mu.RUnlock()

	regions := make([]*Region, 0, len(r.regions))
	for _, region := range r.regions {
		regions = append(regions, region)
	}
	return regions
}

// GetStatus returns the status of a region.
func (r *Router) GetStatus(id string) (*RegionStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, exists := r.status[id]
	if !exists {
		return nil, ErrRegionNotFound
	}
	return status, nil
}

// Route selects the best region for a job.
func (r *Router) Route(ctx context.Context, config *JobRegionConfig) (*Region, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// If required region is set, use it
	if config != nil && config.RequiredRegion != "" {
		region, exists := r.regions[config.RequiredRegion]
		if !exists {
			return nil, ErrRegionNotFound
		}
		status := r.status[config.RequiredRegion]
		if !status.Healthy {
			return nil, ErrRegionUnavailable
		}
		return region, nil
	}

	// Get healthy regions
	healthyRegions := r.getHealthyRegions(config)
	if len(healthyRegions) == 0 {
		return nil, ErrNoHealthyRegions
	}

	// Apply routing policy
	policy := r.config.DefaultPolicy
	if config != nil && config.RoutingPolicy != "" {
		policy = config.RoutingPolicy
	}

	switch policy {
	case RoutingPolicyPriority:
		return r.routeByPriority(healthyRegions), nil
	case RoutingPolicyRoundRobin:
		return r.routeRoundRobin(healthyRegions), nil
	case RoutingPolicyLeastLoaded:
		return r.routeLeastLoaded(healthyRegions), nil
	default:
		return r.routeByPriority(healthyRegions), nil
	}
}

// getHealthyRegions returns healthy regions respecting constraints.
func (r *Router) getHealthyRegions(config *JobRegionConfig) []*Region {
	var regions []*Region

	excludeSet := make(map[string]bool)
	if config != nil {
		for _, id := range config.ExcludedRegions {
			excludeSet[id] = true
		}
	}

	residencySet := make(map[string]bool)
	if config != nil && len(config.DataResidency) > 0 {
		for _, country := range config.DataResidency {
			residencySet[country] = true
		}
	}

	for id, region := range r.regions {
		if !region.Enabled {
			continue
		}
		if excludeSet[id] {
			continue
		}
		if status := r.status[id]; !status.Healthy {
			continue
		}
		if len(residencySet) > 0 && !residencySet[region.Location.Country] {
			continue
		}
		regions = append(regions, region)
	}

	return regions
}

func (r *Router) routeByPriority(regions []*Region) *Region {
	var best *Region
	for _, region := range regions {
		if best == nil || region.Priority < best.Priority {
			best = region
		}
	}
	return best
}

func (r *Router) routeRoundRobin(regions []*Region) *Region {
	if len(regions) == 0 {
		return nil
	}
	region := regions[r.rrIndex%len(regions)]
	r.rrIndex++
	return region
}

func (r *Router) routeLeastLoaded(regions []*Region) *Region {
	var best *Region
	var lowestLoad float64 = 2.0 // Higher than max

	for _, region := range regions {
		status := r.status[region.ID]
		if status.Capacity < lowestLoad {
			lowestLoad = status.Capacity
			best = region
		}
	}
	return best
}

// StartHealthChecks starts periodic health checking.
func (r *Router) StartHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.checkAllRegions(ctx)
		}
	}
}

func (r *Router) checkAllRegions(ctx context.Context) {
	r.mu.RLock()
	regions := make([]*Region, 0, len(r.regions))
	for _, region := range r.regions {
		regions = append(regions, region)
	}
	r.mu.RUnlock()

	for _, region := range regions {
		if r.healthChecker != nil {
			status, err := r.healthChecker.Check(ctx, region)
			if err != nil {
				r.mu.Lock()
				r.status[region.ID].Healthy = false
				r.status[region.ID].Error = err.Error()
				r.status[region.ID].LastCheck = time.Now()
				r.mu.Unlock()
			} else {
				r.mu.Lock()
				r.status[region.ID] = status
				r.mu.Unlock()
			}
		}
	}
}

// Stats returns routing statistics.
type Stats struct {
	TotalRegions   int            `json:"total_regions"`
	HealthyRegions int            `json:"healthy_regions"`
	LocalRegion    string         `json:"local_region"`
	RegionStats    []RegionStatus `json:"region_stats"`
}

// GetStats returns current router statistics.
func (r *Router) GetStats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := Stats{
		TotalRegions: len(r.regions),
		LocalRegion:  r.config.LocalRegion,
		RegionStats:  make([]RegionStatus, 0, len(r.status)),
	}

	for _, status := range r.status {
		if status.Healthy {
			stats.HealthyRegions++
		}
		stats.RegionStats = append(stats.RegionStats, *status)
	}

	return stats
}
