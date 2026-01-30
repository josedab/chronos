// Package cost provides Azure price provider implementation.
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
)

// AzureSdkPriceProvider implements PriceProvider using Azure SDK.
type AzureSdkPriceProvider struct {
	mu             sync.RWMutex
	subscriptionID string
	httpClient     *http.Client
	cache          *priceCache
	config         *CloudProviderConfig
	computeClient  *armcompute.VirtualMachineSizesClient
	initialized    bool
}

// NewAzureSdkPriceProvider creates a new Azure SDK-based price provider.
func NewAzureSdkPriceProvider(config *CloudProviderConfig) (*AzureSdkPriceProvider, error) {
	if config == nil {
		config = DefaultCloudProviderConfig()
	}

	provider := &AzureSdkPriceProvider{
		subscriptionID: config.AzureSubscriptionID,
		httpClient:     &http.Client{Timeout: config.HTTPTimeout},
		cache:          newPriceCache(config.CacheTTL),
		config:         config,
	}

	if config.UseLiveAPIs && config.AzureSubscriptionID != "" {
		if err := provider.initializeClients(context.Background()); err != nil {
			fmt.Printf("Warning: Failed to initialize Azure clients: %v. Using fallback pricing.\n", err)
		}
	}

	return provider, nil
}

func (p *AzureSdkPriceProvider) initializeClients(ctx context.Context) error {
	var cred *azidentity.DefaultAzureCredential
	var err error

	if p.config.AzureClientID != "" && p.config.AzureClientSecret != "" && p.config.AzureTenantID != "" {
		// Use client secret credential
		clientCred, err := azidentity.NewClientSecretCredential(
			p.config.AzureTenantID,
			p.config.AzureClientID,
			p.config.AzureClientSecret,
			nil,
		)
		if err != nil {
			return fmt.Errorf("failed to create client secret credential: %w", err)
		}

		client, err := armcompute.NewVirtualMachineSizesClient(p.subscriptionID, clientCred, nil)
		if err != nil {
			return fmt.Errorf("failed to create compute client: %w", err)
		}
		p.computeClient = client
		p.initialized = true
		return nil
	}

	// Use default credential chain
	cred, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to create default credential: %w", err)
	}

	client, err := armcompute.NewVirtualMachineSizesClient(p.subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	p.computeClient = client
	p.initialized = true

	return nil
}

// GetOnDemandPrice returns the on-demand price for an Azure VM size.
func (p *AzureSdkPriceProvider) GetOnDemandPrice(ctx context.Context, vmSize, region string) (float64, error) {
	cacheKey := fmt.Sprintf("azure:ondemand:%s:%s", vmSize, region)

	if price, found := p.cache.get(cacheKey); found {
		return price, nil
	}

	// Azure Retail Prices API is public and doesn't require SDK authentication
	price, err := p.fetchAzureRetailPrice(ctx, vmSize, region, false)
	if err != nil {
		price = p.getStaticPrice(vmSize)
	}

	p.cache.set(cacheKey, price)
	return price, nil
}

// GetSpotPrice returns the current spot price for an Azure VM size.
func (p *AzureSdkPriceProvider) GetSpotPrice(ctx context.Context, vmSize, region string) (*SpotPrice, error) {
	cacheKey := fmt.Sprintf("azure:spot:%s:%s", vmSize, region)

	if cached, found := p.cache.getSpot(cacheKey); found {
		return cached, nil
	}

	spotPriceVal, err := p.fetchAzureRetailPrice(ctx, vmSize, region, true)
	if err != nil {
		onDemand, _ := p.GetOnDemandPrice(ctx, vmSize, region)
		spotPriceVal = onDemand * 0.25
	}

	spotPrice := &SpotPrice{
		InstanceType:     vmSize,
		Region:           region,
		AvailabilityZone: "1",
		Price:            spotPriceVal,
		Timestamp:        time.Now(),
		InterruptionRate: p.getInterruptionRate(vmSize),
	}

	p.cache.setSpot(cacheKey, spotPrice)
	return spotPrice, nil
}

// GetSpotPriceHistory returns historical spot prices.
func (p *AzureSdkPriceProvider) GetSpotPriceHistory(ctx context.Context, vmSize, region string, since time.Time) ([]SpotPrice, error) {
	current, _ := p.GetSpotPrice(ctx, vmSize, region)
	return []SpotPrice{*current}, nil
}

// GetAvailabilityZones returns available zones for an Azure region.
func (p *AzureSdkPriceProvider) GetAvailabilityZones(ctx context.Context, region string) ([]string, error) {
	return p.getDefaultZones(region), nil
}

func (p *AzureSdkPriceProvider) fetchAzureRetailPrice(ctx context.Context, vmSize, region string, spot bool) (float64, error) {
	armRegion := p.convertToArmRegion(region)
	priceType := "Consumption"
	if spot {
		priceType = "Spot"
	}

	filter := fmt.Sprintf(
		"armRegionName eq '%s' and armSkuName eq '%s' and priceType eq '%s' and serviceName eq 'Virtual Machines'",
		armRegion, vmSize, priceType,
	)

	url := fmt.Sprintf("https://prices.azure.com/api/retail/prices?$filter=%s", filter)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch Azure pricing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Azure pricing API returned status %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			RetailPrice float64 `json:"retailPrice"`
			UnitPrice   float64 `json:"unitPrice"`
			SkuName     string  `json:"skuName"`
		} `json:"Items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode Azure pricing response: %w", err)
	}

	if len(result.Items) > 0 {
		return result.Items[0].RetailPrice, nil
	}

	return 0, fmt.Errorf("no pricing found for %s in %s", vmSize, region)
}

func (p *AzureSdkPriceProvider) convertToArmRegion(region string) string {
	regionMap := map[string]string{
		"eastus": "eastus", "eastus2": "eastus2", "westus": "westus", "westus2": "westus2",
		"westeurope": "westeurope", "northeurope": "northeurope",
		"southeastasia": "southeastasia", "eastasia": "eastasia",
		"japaneast": "japaneast", "australiaeast": "australiaeast",
		"centralus": "centralus", "southcentralus": "southcentralus",
	}
	if armRegion, ok := regionMap[strings.ToLower(region)]; ok {
		return armRegion
	}
	return region
}

func (p *AzureSdkPriceProvider) getStaticPrice(vmSize string) float64 {
	prices := map[string]float64{
		"Standard_B1s": 0.0104, "Standard_B1ms": 0.0207, "Standard_B2s": 0.0416,
		"Standard_B2ms": 0.0832, "Standard_B4ms": 0.166,
		"Standard_D2s_v3": 0.096, "Standard_D4s_v3": 0.192, "Standard_D8s_v3": 0.384,
		"Standard_D2s_v5": 0.096, "Standard_D4s_v5": 0.192,
		"Standard_E2s_v3": 0.126, "Standard_E4s_v3": 0.252, "Standard_E8s_v3": 0.504,
		"Standard_F2s_v2": 0.085, "Standard_F4s_v2": 0.169, "Standard_F8s_v2": 0.338,
		"Standard_NC6": 0.90, "Standard_NC12": 1.80, "Standard_NV6": 1.14,
	}
	if price, ok := prices[vmSize]; ok {
		return price
	}
	return 0.10
}

func (p *AzureSdkPriceProvider) getInterruptionRate(vmSize string) float64 {
	series := strings.Split(vmSize, "_")[1]
	if len(series) > 0 {
		series = series[:1]
	}
	rates := map[string]float64{
		"B": 0.06, "D": 0.08, "E": 0.07, "F": 0.10, "N": 0.15,
	}
	if rate, ok := rates[series]; ok {
		return rate
	}
	return 0.08
}

func (p *AzureSdkPriceProvider) getDefaultZones(region string) []string {
	zoneMap := map[string][]string{
		"eastus": {"1", "2", "3"}, "eastus2": {"1", "2", "3"},
		"westus2": {"1", "2", "3"}, "westeurope": {"1", "2", "3"},
		"northeurope": {"1", "2", "3"}, "southeastasia": {"1", "2", "3"},
		"centralus": {"1", "2", "3"}, "westus": {},
	}
	if zones, ok := zoneMap[strings.ToLower(region)]; ok {
		return zones
	}
	return []string{"1", "2", "3"}
}
