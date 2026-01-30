// Package cost provides cloud provider SDK integrations for real pricing data.
package cost

import (
	"time"
)

// CloudProviderConfig holds configuration for cloud provider SDK clients.
type CloudProviderConfig struct {
	// AWS configuration
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string

	// GCP configuration
	GCPProject         string
	GCPCredentialsJSON string

	// Azure configuration
	AzureSubscriptionID string
	AzureTenantID       string
	AzureClientID       string
	AzureClientSecret   string

	// Common settings
	CacheTTL    time.Duration
	HTTPTimeout time.Duration

	// UseLiveAPIs enables real API calls (requires valid credentials)
	// When false, uses static pricing data as fallback
	UseLiveAPIs bool
}

// DefaultCloudProviderConfig returns sensible defaults.
func DefaultCloudProviderConfig() *CloudProviderConfig {
	return &CloudProviderConfig{
		CacheTTL:    5 * time.Minute,
		HTTPTimeout: 30 * time.Second,
		UseLiveAPIs: false, // Default to static pricing for safety
	}
}

// InstanceRequirements specifies compute requirements for instance selection.
type InstanceRequirements struct {
	VCPUs           int
	MemoryGB        int
	GPUs            int
	Regions         []string
	MaxPrice        float64
	MaxInterruption float64
}

// SpotPriceOption represents a single spot price option.
type SpotPriceOption struct {
	Cloud        string
	InstanceType string
	Region       string
	Price        float64
	SpotPrice    *SpotPrice
}

// SpotPriceComparison holds comparison results across providers.
type SpotPriceComparison struct {
	Cheapest   SpotPriceOption
	Options    []SpotPriceOption
	Comparison map[string]float64
}
