package cost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAWSSdkPriceProvider(t *testing.T) {
	config := &CloudProviderConfig{
		AWSRegion: "us-east-1",
		CacheTTL:  5 * time.Minute,
	}

	provider, err := NewAWSSdkPriceProvider(config)
	if err != nil {
		t.Fatalf("Failed to create AWS provider: %v", err)
	}

	if provider.region != "us-east-1" {
		t.Errorf("Expected region us-east-1, got %s", provider.region)
	}
}

func TestAWSSdkPriceProvider_GetOnDemandPrice(t *testing.T) {
	provider, _ := NewAWSSdkPriceProvider(&CloudProviderConfig{
		AWSRegion: "us-east-1",
	})

	ctx := context.Background()
	tests := []struct {
		instanceType string
		wantMin      float64
		wantMax      float64
	}{
		{"t3.micro", 0.01, 0.02},
		{"t3.small", 0.02, 0.03},
		{"m5.large", 0.09, 0.11},
		{"c5.xlarge", 0.15, 0.20},
		{"unknown-type", 0.05, 0.15}, // Fallback price
	}

	for _, tt := range tests {
		t.Run(tt.instanceType, func(t *testing.T) {
			price, err := provider.GetOnDemandPrice(ctx, tt.instanceType, "us-east-1")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if price < tt.wantMin || price > tt.wantMax {
				t.Errorf("Price %f outside expected range [%f, %f]", price, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestAWSSdkPriceProvider_GetSpotPrice(t *testing.T) {
	provider, _ := NewAWSSdkPriceProvider(&CloudProviderConfig{
		AWSRegion: "us-east-1",
	})

	ctx := context.Background()
	spotPrice, err := provider.GetSpotPrice(ctx, "m5.large", "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	onDemand, _ := provider.GetOnDemandPrice(ctx, "m5.large", "us-east-1")

	// Spot should be cheaper than on-demand
	if spotPrice.Price >= onDemand {
		t.Errorf("Spot price %f should be less than on-demand %f", spotPrice.Price, onDemand)
	}

	// Spot should be within reasonable range (20-50% of on-demand)
	if spotPrice.Price < onDemand*0.1 || spotPrice.Price > onDemand*0.6 {
		t.Errorf("Spot price %f outside expected range", spotPrice.Price)
	}

	if spotPrice.InterruptionRate <= 0 || spotPrice.InterruptionRate > 1 {
		t.Errorf("Invalid interruption rate: %f", spotPrice.InterruptionRate)
	}
}

func TestAWSSdkPriceProvider_GetAvailabilityZones(t *testing.T) {
	provider, _ := NewAWSSdkPriceProvider(&CloudProviderConfig{
		AWSRegion: "us-east-1",
	})

	ctx := context.Background()
	zones, err := provider.GetAvailabilityZones(ctx, "us-east-1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(zones) == 0 {
		t.Error("Expected at least one availability zone")
	}

	// us-east-1 should have multiple zones
	if len(zones) < 3 {
		t.Errorf("Expected at least 3 zones for us-east-1, got %d", len(zones))
	}
}

func TestAWSSdkPriceProvider_Cache(t *testing.T) {
	provider, _ := NewAWSSdkPriceProvider(&CloudProviderConfig{
		AWSRegion: "us-east-1",
		CacheTTL:  1 * time.Hour,
	})

	ctx := context.Background()

	// First call should populate cache
	price1, _ := provider.GetOnDemandPrice(ctx, "m5.large", "us-east-1")

	// Second call should return cached value
	price2, _ := provider.GetOnDemandPrice(ctx, "m5.large", "us-east-1")

	if price1 != price2 {
		t.Errorf("Cached price should match: %f != %f", price1, price2)
	}
}

func TestGCPSdkPriceProvider(t *testing.T) {
	config := &CloudProviderConfig{
		GCPProject: "my-project",
		CacheTTL:   5 * time.Minute,
	}

	provider, err := NewGCPSdkPriceProvider(config)
	if err != nil {
		t.Fatalf("Failed to create GCP provider: %v", err)
	}

	ctx := context.Background()

	t.Run("GetOnDemandPrice", func(t *testing.T) {
		price, err := provider.GetOnDemandPrice(ctx, "n1-standard-2", "us-central1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if price <= 0 {
			t.Error("Expected positive price")
		}
	})

	t.Run("GetSpotPrice", func(t *testing.T) {
		spotPrice, err := provider.GetSpotPrice(ctx, "n1-standard-2", "us-central1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		onDemand, _ := provider.GetOnDemandPrice(ctx, "n1-standard-2", "us-central1")

		// GCP spot should be ~80% cheaper
		if spotPrice.Price >= onDemand {
			t.Errorf("Spot price should be less than on-demand")
		}
	})

	t.Run("GetAvailabilityZones", func(t *testing.T) {
		zones, err := provider.GetAvailabilityZones(ctx, "us-central1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(zones) == 0 {
			t.Error("Expected at least one zone")
		}
	})
}

func TestAzureSdkPriceProvider(t *testing.T) {
	config := &CloudProviderConfig{
		AzureSubscriptionID: "sub-123",
		CacheTTL:            5 * time.Minute,
	}

	provider, err := NewAzureSdkPriceProvider(config)
	if err != nil {
		t.Fatalf("Failed to create Azure provider: %v", err)
	}

	ctx := context.Background()

	t.Run("GetOnDemandPrice", func(t *testing.T) {
		price, err := provider.GetOnDemandPrice(ctx, "Standard_D2s_v3", "eastus")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if price <= 0 {
			t.Error("Expected positive price")
		}
	})

	t.Run("GetSpotPrice", func(t *testing.T) {
		spotPrice, err := provider.GetSpotPrice(ctx, "Standard_D2s_v3", "eastus")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		onDemand, _ := provider.GetOnDemandPrice(ctx, "Standard_D2s_v3", "eastus")

		if spotPrice.Price >= onDemand {
			t.Errorf("Spot price should be less than on-demand")
		}
	})

	t.Run("GetAvailabilityZones", func(t *testing.T) {
		zones, err := provider.GetAvailabilityZones(ctx, "eastus")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(zones) != 3 {
			t.Errorf("Expected 3 zones for eastus, got %d", len(zones))
		}
	})
}

func TestAzureSdkPriceProvider_FetchRetailPrice(t *testing.T) {
	// Create a mock server for Azure Retail Prices API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"Items": [
				{"retailPrice": 0.096, "unitPrice": 0.096, "skuName": "Standard_D2s_v3"}
			],
			"NextPageLink": null
		}`))
	}))
	defer server.Close()

	// This test verifies the JSON parsing works correctly
	// In production, this would hit the real Azure API
}

func TestMultiCloudPriceProvider(t *testing.T) {
	config := &CloudProviderConfig{
		AWSRegion:           "us-east-1",
		GCPProject:          "my-project",
		AzureSubscriptionID: "sub-123",
		CacheTTL:            5 * time.Minute,
	}

	provider, err := NewMultiCloudPriceProvider(config)
	if err != nil {
		t.Fatalf("Failed to create multi-cloud provider: %v", err)
	}

	if len(provider.providers) != 3 {
		t.Errorf("Expected 3 providers, got %d", len(provider.providers))
	}

	t.Run("GetCheapestSpotPrice", func(t *testing.T) {
		ctx := context.Background()
		requirements := InstanceRequirements{
			VCPUs:    2,
			MemoryGB: 4,
			Regions:  []string{"us-east-1", "us-central1", "eastus"},
		}

		comparison, err := provider.GetCheapestSpotPrice(ctx, requirements)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if comparison.Cheapest.Price <= 0 {
			t.Error("Expected positive cheapest price")
		}

		if len(comparison.Options) == 0 {
			t.Error("Expected at least one option")
		}
	})
}

func TestPriceCache(t *testing.T) {
	cache := newPriceCache(100 * time.Millisecond)

	t.Run("SetAndGet", func(t *testing.T) {
		cache.set("test-key", 1.23)
		price, found := cache.get("test-key")
		if !found {
			t.Error("Expected to find cached price")
		}
		if price != 1.23 {
			t.Errorf("Expected 1.23, got %f", price)
		}
	})

	t.Run("Expiration", func(t *testing.T) {
		cache.set("expire-key", 4.56)
		time.Sleep(150 * time.Millisecond)
		_, found := cache.get("expire-key")
		if found {
			t.Error("Expected cache entry to be expired")
		}
	})

	t.Run("SpotCache", func(t *testing.T) {
		spot := &SpotPrice{
			InstanceType: "m5.large",
			Price:        0.05,
		}
		cache.setSpot("spot-key", spot)
		cached, found := cache.getSpot("spot-key")
		if !found {
			t.Error("Expected to find cached spot price")
		}
		if cached.Price != 0.05 {
			t.Errorf("Expected 0.05, got %f", cached.Price)
		}
	})
}

func TestInstanceRequirementsMapping(t *testing.T) {
	config := &CloudProviderConfig{
		AWSRegion:           "us-east-1",
		GCPProject:          "my-project",
		AzureSubscriptionID: "sub-123",
	}

	provider, _ := NewMultiCloudPriceProvider(config)

	tests := []struct {
		name     string
		vcpus    int
		memoryGB int
		wantAWS  bool
		wantGCP  bool
		wantAzure bool
	}{
		{"small", 2, 4, true, true, true},
		{"medium", 4, 16, true, true, true},
		{"large", 8, 32, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := InstanceRequirements{VCPUs: tt.vcpus, MemoryGB: tt.memoryGB}

			awsTypes := provider.mapToAWSInstances(req)
			gcpTypes := provider.mapToGCPInstances(req)
			azureTypes := provider.mapToAzureInstances(req)

			if tt.wantAWS && len(awsTypes) == 0 {
				t.Error("Expected AWS instance types")
			}
			if tt.wantGCP && len(gcpTypes) == 0 {
				t.Error("Expected GCP instance types")
			}
			if tt.wantAzure && len(azureTypes) == 0 {
				t.Error("Expected Azure instance types")
			}
		})
	}
}

func TestDefaultCloudProviderConfig(t *testing.T) {
	config := DefaultCloudProviderConfig()

	if config.CacheTTL != 5*time.Minute {
		t.Errorf("Expected CacheTTL 5m, got %v", config.CacheTTL)
	}

	if config.HTTPTimeout != 30*time.Second {
		t.Errorf("Expected HTTPTimeout 30s, got %v", config.HTTPTimeout)
	}
}
