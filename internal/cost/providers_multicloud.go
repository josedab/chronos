// Package cost provides multi-cloud price comparison functionality.
package cost

import (
	"context"
	"fmt"
	"sort"
)

// MultiCloudPriceProvider aggregates pricing from multiple cloud providers.
type MultiCloudPriceProvider struct {
	providers map[string]PriceProvider
}

// NewMultiCloudPriceProvider creates a provider that can query multiple clouds.
func NewMultiCloudPriceProvider(config *CloudProviderConfig) (*MultiCloudPriceProvider, error) {
	providers := make(map[string]PriceProvider)

	if config.AWSRegion != "" || config.AWSAccessKeyID != "" {
		aws, err := NewAWSSdkPriceProvider(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create AWS provider: %w", err)
		}
		providers["aws"] = aws
	}

	if config.GCPProject != "" || config.GCPCredentialsJSON != "" {
		gcp, err := NewGCPSdkPriceProvider(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCP provider: %w", err)
		}
		providers["gcp"] = gcp
	}

	if config.AzureSubscriptionID != "" || config.AzureClientID != "" {
		azure, err := NewAzureSdkPriceProvider(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure provider: %w", err)
		}
		providers["azure"] = azure
	}

	return &MultiCloudPriceProvider{providers: providers}, nil
}

// GetCheapestSpotPrice finds the cheapest spot price across all configured providers.
func (m *MultiCloudPriceProvider) GetCheapestSpotPrice(ctx context.Context, requirements InstanceRequirements) (*SpotPriceComparison, error) {
	var results []SpotPriceOption

	for cloud, provider := range m.providers {
		instanceTypes := m.mapRequirementsToInstanceTypes(cloud, requirements)
		for _, instanceType := range instanceTypes {
			for _, region := range requirements.Regions {
				price, err := provider.GetSpotPrice(ctx, instanceType, region)
				if err != nil {
					continue
				}
				results = append(results, SpotPriceOption{
					Cloud: cloud, InstanceType: instanceType, Region: region,
					Price: price.Price, SpotPrice: price,
				})
			}
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no spot prices found for requirements")
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Price < results[j].Price })

	return &SpotPriceComparison{
		Cheapest:   results[0],
		Options:    results,
		Comparison: m.buildComparison(results),
	}, nil
}

func (m *MultiCloudPriceProvider) mapRequirementsToInstanceTypes(cloud string, req InstanceRequirements) []string {
	switch cloud {
	case "aws":
		return m.mapToAWSInstances(req)
	case "gcp":
		return m.mapToGCPInstances(req)
	case "azure":
		return m.mapToAzureInstances(req)
	default:
		return nil
	}
}

func (m *MultiCloudPriceProvider) mapToAWSInstances(req InstanceRequirements) []string {
	if req.VCPUs <= 2 && req.MemoryGB <= 4 {
		return []string{"t3.small", "t3.medium"}
	} else if req.VCPUs <= 4 && req.MemoryGB <= 16 {
		return []string{"m5.large", "m5.xlarge", "c5.large", "c5.xlarge"}
	} else if req.VCPUs <= 8 && req.MemoryGB <= 32 {
		return []string{"m5.2xlarge", "c5.2xlarge", "r5.xlarge"}
	}
	return []string{"m5.xlarge"}
}

func (m *MultiCloudPriceProvider) mapToGCPInstances(req InstanceRequirements) []string {
	if req.VCPUs <= 2 && req.MemoryGB <= 4 {
		return []string{"e2-small", "e2-medium"}
	} else if req.VCPUs <= 4 && req.MemoryGB <= 16 {
		return []string{"n1-standard-2", "n1-standard-4", "e2-standard-4"}
	} else if req.VCPUs <= 8 && req.MemoryGB <= 32 {
		return []string{"n1-standard-8", "c2-standard-8", "e2-standard-8"}
	}
	return []string{"n1-standard-4"}
}

func (m *MultiCloudPriceProvider) mapToAzureInstances(req InstanceRequirements) []string {
	if req.VCPUs <= 2 && req.MemoryGB <= 4 {
		return []string{"Standard_B2s", "Standard_D2s_v3"}
	} else if req.VCPUs <= 4 && req.MemoryGB <= 16 {
		return []string{"Standard_D4s_v3", "Standard_F4s_v2"}
	} else if req.VCPUs <= 8 && req.MemoryGB <= 32 {
		return []string{"Standard_D8s_v3", "Standard_E8s_v3", "Standard_F8s_v2"}
	}
	return []string{"Standard_D4s_v3"}
}

func (m *MultiCloudPriceProvider) buildComparison(options []SpotPriceOption) map[string]float64 {
	comparison := make(map[string]float64)
	for _, opt := range options {
		key := fmt.Sprintf("%s:%s:%s", opt.Cloud, opt.Region, opt.InstanceType)
		comparison[key] = opt.Price
	}
	return comparison
}
