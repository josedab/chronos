// Package cost provides GCP price provider implementation.
package cost

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	billing "cloud.google.com/go/billing/apiv1"
	billingpb "cloud.google.com/go/billing/apiv1/billingpb"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCPSdkPriceProvider implements PriceProvider using GCP SDK.
type GCPSdkPriceProvider struct {
	mu            sync.RWMutex
	project       string
	httpClient    *http.Client
	cache         *priceCache
	config        *CloudProviderConfig
	billingClient *billing.CloudCatalogClient
	computeClient *compute.Service
	initialized   bool
}

// NewGCPSdkPriceProvider creates a new GCP SDK-based price provider.
func NewGCPSdkPriceProvider(config *CloudProviderConfig) (*GCPSdkPriceProvider, error) {
	if config == nil {
		config = DefaultCloudProviderConfig()
	}

	provider := &GCPSdkPriceProvider{
		project:    config.GCPProject,
		httpClient: &http.Client{Timeout: config.HTTPTimeout},
		cache:      newPriceCache(config.CacheTTL),
		config:     config,
	}

	if config.UseLiveAPIs {
		if err := provider.initializeClients(context.Background()); err != nil {
			fmt.Printf("Warning: Failed to initialize GCP clients: %v. Using fallback pricing.\n", err)
		}
	}

	return provider, nil
}

func (p *GCPSdkPriceProvider) initializeClients(ctx context.Context) error {
	var opts []option.ClientOption

	if p.config.GCPCredentialsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(p.config.GCPCredentialsJSON)))
	}

	billingClient, err := billing.NewCloudCatalogClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create billing client: %w", err)
	}
	p.billingClient = billingClient

	computeClient, err := compute.NewService(ctx, opts...)
	if err != nil {
		billingClient.Close()
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	p.computeClient = computeClient
	p.initialized = true

	return nil
}

// GetOnDemandPrice returns the on-demand price for a GCP machine type.
func (p *GCPSdkPriceProvider) GetOnDemandPrice(ctx context.Context, machineType, region string) (float64, error) {
	cacheKey := fmt.Sprintf("gcp:ondemand:%s:%s", machineType, region)

	if price, found := p.cache.get(cacheKey); found {
		return price, nil
	}

	var price float64

	if p.initialized && p.billingClient != nil {
		apiPrice, err := p.fetchPriceFromAPI(ctx, machineType, region)
		if err == nil {
			price = apiPrice
		} else {
			price = p.getStaticPrice(machineType)
		}
	} else {
		price = p.getStaticPrice(machineType)
	}

	p.cache.set(cacheKey, price)
	return price, nil
}

func (p *GCPSdkPriceProvider) fetchPriceFromAPI(ctx context.Context, machineType, region string) (float64, error) {
	// Compute Engine service ID
	req := &billingpb.ListSkusRequest{
		Parent: "services/6F81-5844-456A",
	}

	it := p.billingClient.ListSkus(ctx, req)
	for {
		sku, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		// Match SKU to machine type and region
		if p.matchesSku(sku, machineType, region) {
			return p.extractPrice(sku), nil
		}
	}

	return 0, fmt.Errorf("no pricing found for %s in %s", machineType, region)
}

func (p *GCPSdkPriceProvider) matchesSku(sku *billingpb.Sku, machineType, region string) bool {
	desc := strings.ToLower(sku.Description)
	machineFamily := strings.Split(machineType, "-")[0]

	if !strings.Contains(desc, strings.ToLower(machineFamily)) {
		return false
	}

	for _, sr := range sku.ServiceRegions {
		if strings.Contains(strings.ToLower(sr), strings.ToLower(region)) {
			return true
		}
	}
	return false
}

func (p *GCPSdkPriceProvider) extractPrice(sku *billingpb.Sku) float64 {
	for _, tier := range sku.PricingInfo {
		expr := tier.PricingExpression
		if expr == nil {
			continue
		}
		for _, rate := range expr.TieredRates {
			if rate.UnitPrice != nil {
				// Convert from micro-units
				return float64(rate.UnitPrice.Units) + float64(rate.UnitPrice.Nanos)/1e9
			}
		}
	}
	return 0
}

// GetSpotPrice returns the current preemptible/spot price.
func (p *GCPSdkPriceProvider) GetSpotPrice(ctx context.Context, machineType, region string) (*SpotPrice, error) {
	cacheKey := fmt.Sprintf("gcp:spot:%s:%s", machineType, region)

	if cached, found := p.cache.getSpot(cacheKey); found {
		return cached, nil
	}

	// GCP Spot VMs are typically 60-91% cheaper than on-demand
	onDemand, _ := p.GetOnDemandPrice(ctx, machineType, region)
	spotPrice := &SpotPrice{
		InstanceType:     machineType,
		Region:           region,
		AvailabilityZone: region + "-a",
		Price:            onDemand * 0.20,
		Timestamp:        time.Now(),
		InterruptionRate: p.getInterruptionRate(machineType),
	}

	p.cache.setSpot(cacheKey, spotPrice)
	return spotPrice, nil
}

// GetSpotPriceHistory returns historical spot prices.
func (p *GCPSdkPriceProvider) GetSpotPriceHistory(ctx context.Context, machineType, region string, since time.Time) ([]SpotPrice, error) {
	current, _ := p.GetSpotPrice(ctx, machineType, region)
	return []SpotPrice{*current}, nil
}

// GetAvailabilityZones returns available zones for a GCP region.
func (p *GCPSdkPriceProvider) GetAvailabilityZones(ctx context.Context, region string) ([]string, error) {
	if p.initialized && p.computeClient != nil && p.project != "" {
		return p.fetchZonesFromAPI(ctx, region)
	}
	return p.getDefaultZones(region), nil
}

func (p *GCPSdkPriceProvider) fetchZonesFromAPI(ctx context.Context, region string) ([]string, error) {
	resp, err := p.computeClient.Zones.List(p.project).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	var zones []string
	for _, zone := range resp.Items {
		if strings.HasPrefix(zone.Name, region) {
			zones = append(zones, zone.Name)
		}
	}
	return zones, nil
}

func (p *GCPSdkPriceProvider) getStaticPrice(machineType string) float64 {
	prices := map[string]float64{
		"e2-micro": 0.0084, "e2-small": 0.0168, "e2-medium": 0.0335,
		"e2-standard-2": 0.067, "e2-standard-4": 0.134, "e2-standard-8": 0.268,
		"n1-standard-1": 0.0475, "n1-standard-2": 0.095, "n1-standard-4": 0.19, "n1-standard-8": 0.38,
		"n2-standard-2": 0.0971, "n2-standard-4": 0.1942, "n2-standard-8": 0.3885,
		"c2-standard-4": 0.2088, "c2-standard-8": 0.4176, "c2-standard-16": 0.8352,
		"m1-megamem-96": 10.6740, "a2-highgpu-1g": 3.6731,
	}
	if price, ok := prices[machineType]; ok {
		return price
	}
	return 0.10
}

func (p *GCPSdkPriceProvider) getInterruptionRate(machineType string) float64 {
	family := strings.Split(machineType, "-")[0]
	rates := map[string]float64{
		"e2": 0.08, "n1": 0.10, "n2": 0.09, "c2": 0.12, "m1": 0.07, "a2": 0.15,
	}
	if rate, ok := rates[family]; ok {
		return rate
	}
	return 0.10
}

func (p *GCPSdkPriceProvider) getDefaultZones(region string) []string {
	zoneMap := map[string][]string{
		"us-central1":     {"us-central1-a", "us-central1-b", "us-central1-c", "us-central1-f"},
		"us-east1":        {"us-east1-b", "us-east1-c", "us-east1-d"},
		"us-west1":        {"us-west1-a", "us-west1-b", "us-west1-c"},
		"europe-west1":    {"europe-west1-b", "europe-west1-c", "europe-west1-d"},
		"asia-east1":      {"asia-east1-a", "asia-east1-b", "asia-east1-c"},
		"asia-southeast1": {"asia-southeast1-a", "asia-southeast1-b", "asia-southeast1-c"},
	}
	if zones, ok := zoneMap[region]; ok {
		return zones
	}
	return []string{region + "-a", region + "-b", region + "-c"}
}

// Close releases resources.
func (p *GCPSdkPriceProvider) Close() error {
	if p.billingClient != nil {
		return p.billingClient.Close()
	}
	return nil
}
