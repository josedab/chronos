// Package cost provides simple mock price providers for testing and demonstration.
package cost

import (
	"context"
	"time"
)

// SimplePriceProvider provides mock pricing information for testing.
type SimplePriceProvider struct {
	prices map[string]float64
	region string
}

// NewSimplePriceProvider creates a new simple price provider.
func NewSimplePriceProvider(region string) *SimplePriceProvider {
	return &SimplePriceProvider{
		region: region,
		prices: map[string]float64{
			"t3.micro": 0.0104, "t3.small": 0.0208, "t3.medium": 0.0416, "t3.large": 0.0832,
			"m5.large": 0.096, "m5.xlarge": 0.192, "m5.2xlarge": 0.384,
			"c5.large": 0.085, "c5.xlarge": 0.17, "r5.large": 0.126,
			"e2-micro": 0.0084, "e2-small": 0.0168, "e2-medium": 0.0335,
			"n1-standard-1": 0.0475, "n1-standard-2": 0.095, "n1-standard-4": 0.19,
			"Standard_B1s": 0.0124, "Standard_B2s": 0.0496, "Standard_D2s_v3": 0.096,
		},
	}
}

// GetOnDemandPrice returns the on-demand price for an instance type.
func (p *SimplePriceProvider) GetOnDemandPrice(ctx context.Context, instanceType, region string) (float64, error) {
	if price, ok := p.prices[instanceType]; ok {
		return price, nil
	}
	return 0.1, nil
}

// GetSpotPrice returns the current spot price.
func (p *SimplePriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (*SpotPrice, error) {
	onDemand, _ := p.GetOnDemandPrice(ctx, instanceType, region)
	return &SpotPrice{
		InstanceType:     instanceType,
		Region:           region,
		AvailabilityZone: region + "a",
		Price:            onDemand * 0.3,
		Timestamp:        time.Now(),
		InterruptionRate: 0.05,
	}, nil
}

// GetSpotPriceHistory returns historical spot prices.
func (p *SimplePriceProvider) GetSpotPriceHistory(ctx context.Context, instanceType, region string, since time.Time) ([]SpotPrice, error) {
	current, _ := p.GetSpotPrice(ctx, instanceType, region)
	return []SpotPrice{*current}, nil
}

// GetAvailabilityZones returns available zones.
func (p *SimplePriceProvider) GetAvailabilityZones(ctx context.Context, region string) ([]string, error) {
	return []string{region + "a", region + "b", region + "c"}, nil
}

// NewAWSPriceProvider creates a simple AWS price provider for testing.
func NewAWSPriceProvider(region string) *SimplePriceProvider {
	return NewSimplePriceProvider(region)
}

// NewGCPPriceProvider creates a simple GCP price provider for testing.
func NewGCPPriceProvider(project string) *SimplePriceProvider {
	return NewSimplePriceProvider("us-central1")
}

// NewAzurePriceProvider creates a simple Azure price provider for testing.
func NewAzurePriceProvider(subscriptionID string) *SimplePriceProvider {
	return NewSimplePriceProvider("eastus")
}
