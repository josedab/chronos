// Package cost provides AWS price provider implementation.
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingtypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

// AWSSdkPriceProvider implements PriceProvider using AWS SDK.
// It uses the AWS Price List API for on-demand prices and EC2 API for spot prices.
type AWSSdkPriceProvider struct {
	mu            sync.RWMutex
	region        string
	httpClient    *http.Client
	cache         *priceCache
	config        *CloudProviderConfig
	ec2Client     *ec2.Client
	pricingClient *pricing.Client
	initialized   bool
}

// NewAWSSdkPriceProvider creates a new AWS SDK-based price provider.
func NewAWSSdkPriceProvider(config *CloudProviderConfig) (*AWSSdkPriceProvider, error) {
	if config == nil {
		config = DefaultCloudProviderConfig()
	}

	region := config.AWSRegion
	if region == "" {
		region = "us-east-1"
	}

	provider := &AWSSdkPriceProvider{
		region:     region,
		httpClient: &http.Client{Timeout: config.HTTPTimeout},
		cache:      newPriceCache(config.CacheTTL),
		config:     config,
	}

	// Initialize AWS SDK clients if credentials are available
	if config.UseLiveAPIs {
		if err := provider.initializeClients(context.Background()); err != nil {
			// Log warning but don't fail - will use fallback pricing
			fmt.Printf("Warning: Failed to initialize AWS clients: %v. Using fallback pricing.\n", err)
		}
	}

	return provider, nil
}

func (p *AWSSdkPriceProvider) initializeClients(ctx context.Context) error {
	var cfg aws.Config
	var err error

	if p.config.AWSAccessKeyID != "" && p.config.AWSSecretAccessKey != "" {
		// Use explicit credentials
		cfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(p.region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				p.config.AWSAccessKeyID,
				p.config.AWSSecretAccessKey,
				"",
			)),
		)
	} else {
		// Use default credential chain (env vars, IAM role, etc.)
		cfg, err = awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(p.region),
		)
	}

	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	p.ec2Client = ec2.NewFromConfig(cfg)
	// Pricing API is only available in us-east-1 and ap-south-1
	pricingCfg := cfg.Copy()
	pricingCfg.Region = "us-east-1"
	p.pricingClient = pricing.NewFromConfig(pricingCfg)
	p.initialized = true

	return nil
}

// GetOnDemandPrice returns the on-demand price for an EC2 instance type.
func (p *AWSSdkPriceProvider) GetOnDemandPrice(ctx context.Context, instanceType, region string) (float64, error) {
	cacheKey := fmt.Sprintf("aws:ondemand:%s:%s", instanceType, region)

	if price, found := p.cache.get(cacheKey); found {
		return price, nil
	}

	var price float64

	if p.initialized && p.pricingClient != nil {
		// Use AWS Pricing API
		apiPrice, err := p.fetchPriceFromAPI(ctx, instanceType, region)
		if err == nil {
			price = apiPrice
		} else {
			price = p.getStaticPrice(instanceType)
		}
	} else {
		price = p.getStaticPrice(instanceType)
	}

	p.cache.set(cacheKey, price)
	return price, nil
}

func (p *AWSSdkPriceProvider) fetchPriceFromAPI(ctx context.Context, instanceType, region string) (float64, error) {
	location := p.regionToLocation(region)

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters: []pricingtypes.Filter{
			{Type: pricingtypes.FilterTypeTermMatch, Field: aws.String("instanceType"), Value: aws.String(instanceType)},
			{Type: pricingtypes.FilterTypeTermMatch, Field: aws.String("location"), Value: aws.String(location)},
			{Type: pricingtypes.FilterTypeTermMatch, Field: aws.String("operatingSystem"), Value: aws.String("Linux")},
			{Type: pricingtypes.FilterTypeTermMatch, Field: aws.String("preInstalledSw"), Value: aws.String("NA")},
			{Type: pricingtypes.FilterTypeTermMatch, Field: aws.String("tenancy"), Value: aws.String("Shared")},
			{Type: pricingtypes.FilterTypeTermMatch, Field: aws.String("capacitystatus"), Value: aws.String("Used")},
		},
		MaxResults: aws.Int32(1),
	}

	result, err := p.pricingClient.GetProducts(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get AWS pricing: %w", err)
	}

	if len(result.PriceList) == 0 {
		return 0, fmt.Errorf("no pricing found for %s in %s", instanceType, region)
	}

	return p.parsePriceList(result.PriceList[0])
}

func (p *AWSSdkPriceProvider) parsePriceList(priceListJSON string) (float64, error) {
	var priceData map[string]interface{}
	if err := json.Unmarshal([]byte(priceListJSON), &priceData); err != nil {
		return 0, err
	}

	terms, ok := priceData["terms"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid price list format")
	}

	onDemand, ok := terms["OnDemand"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("no OnDemand terms found")
	}

	for _, term := range onDemand {
		termData, ok := term.(map[string]interface{})
		if !ok {
			continue
		}
		priceDimensions, ok := termData["priceDimensions"].(map[string]interface{})
		if !ok {
			continue
		}
		for _, dim := range priceDimensions {
			dimData, ok := dim.(map[string]interface{})
			if !ok {
				continue
			}
			pricePerUnit, ok := dimData["pricePerUnit"].(map[string]interface{})
			if !ok {
				continue
			}
			if usd, ok := pricePerUnit["USD"].(string); ok {
				price, err := strconv.ParseFloat(usd, 64)
				if err == nil && price > 0 {
					return price, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("could not parse price from response")
}

func (p *AWSSdkPriceProvider) regionToLocation(region string) string {
	locations := map[string]string{
		"us-east-1":      "US East (N. Virginia)",
		"us-east-2":      "US East (Ohio)",
		"us-west-1":      "US West (N. California)",
		"us-west-2":      "US West (Oregon)",
		"eu-west-1":      "EU (Ireland)",
		"eu-west-2":      "EU (London)",
		"eu-west-3":      "EU (Paris)",
		"eu-central-1":   "EU (Frankfurt)",
		"eu-north-1":     "EU (Stockholm)",
		"ap-southeast-1": "Asia Pacific (Singapore)",
		"ap-southeast-2": "Asia Pacific (Sydney)",
		"ap-northeast-1": "Asia Pacific (Tokyo)",
		"ap-northeast-2": "Asia Pacific (Seoul)",
		"ap-south-1":     "Asia Pacific (Mumbai)",
		"sa-east-1":      "South America (Sao Paulo)",
		"ca-central-1":   "Canada (Central)",
	}
	if loc, ok := locations[region]; ok {
		return loc
	}
	return region
}

// GetSpotPrice returns the current spot price for an EC2 instance type.
func (p *AWSSdkPriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (*SpotPrice, error) {
	cacheKey := fmt.Sprintf("aws:spot:%s:%s", instanceType, region)

	if cached, found := p.cache.getSpot(cacheKey); found {
		return cached, nil
	}

	var spotPrice *SpotPrice

	if p.initialized && p.ec2Client != nil {
		apiPrice, err := p.fetchSpotPriceFromAPI(ctx, instanceType, region)
		if err == nil {
			spotPrice = apiPrice
		}
	}

	if spotPrice == nil {
		// Fallback: estimate from on-demand price
		onDemand, _ := p.GetOnDemandPrice(ctx, instanceType, region)
		spotPrice = &SpotPrice{
			InstanceType:     instanceType,
			Region:           region,
			AvailabilityZone: region + "a",
			Price:            onDemand * 0.3,
			Timestamp:        time.Now(),
			InterruptionRate: p.getInterruptionRate(instanceType),
		}
	}

	p.cache.setSpot(cacheKey, spotPrice)
	return spotPrice, nil
}

func (p *AWSSdkPriceProvider) fetchSpotPriceFromAPI(ctx context.Context, instanceType, region string) (*SpotPrice, error) {
	input := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
		ProductDescriptions: []string{"Linux/UNIX"},
		StartTime:           aws.Time(time.Now().Add(-1 * time.Hour)),
		MaxResults:          aws.Int32(1),
	}

	result, err := p.ec2Client.DescribeSpotPriceHistory(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get spot price: %w", err)
	}

	if len(result.SpotPriceHistory) > 0 {
		sp := result.SpotPriceHistory[0]
		price, _ := strconv.ParseFloat(*sp.SpotPrice, 64)
		return &SpotPrice{
			InstanceType:     instanceType,
			Region:           region,
			AvailabilityZone: *sp.AvailabilityZone,
			Price:            price,
			Timestamp:        *sp.Timestamp,
			InterruptionRate: p.getInterruptionRate(instanceType),
		}, nil
	}

	return nil, fmt.Errorf("no spot price history found")
}

// GetSpotPriceHistory returns historical spot prices.
func (p *AWSSdkPriceProvider) GetSpotPriceHistory(ctx context.Context, instanceType, region string, since time.Time) ([]SpotPrice, error) {
	if p.initialized && p.ec2Client != nil {
		return p.fetchSpotHistoryFromAPI(ctx, instanceType, region, since)
	}

	// Fallback: return simulated history
	current, _ := p.GetSpotPrice(ctx, instanceType, region)
	history := make([]SpotPrice, 0, 24)

	for i := 0; i < 24 && time.Now().Add(-time.Duration(i)*time.Hour).After(since); i++ {
		variance := 1.0 + (float64(i%5)-2.0)*0.05
		history = append(history, SpotPrice{
			InstanceType:     instanceType,
			Region:           region,
			AvailabilityZone: current.AvailabilityZone,
			Price:            current.Price * variance,
			Timestamp:        time.Now().Add(-time.Duration(i) * time.Hour),
			InterruptionRate: current.InterruptionRate,
		})
	}

	return history, nil
}

func (p *AWSSdkPriceProvider) fetchSpotHistoryFromAPI(ctx context.Context, instanceType, region string, since time.Time) ([]SpotPrice, error) {
	input := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
		ProductDescriptions: []string{"Linux/UNIX"},
		StartTime:           aws.Time(since),
	}

	var history []SpotPrice
	paginator := ec2.NewDescribeSpotPriceHistoryPaginator(p.ec2Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, sp := range page.SpotPriceHistory {
			price, _ := strconv.ParseFloat(*sp.SpotPrice, 64)
			history = append(history, SpotPrice{
				InstanceType:     instanceType,
				Region:           region,
				AvailabilityZone: *sp.AvailabilityZone,
				Price:            price,
				Timestamp:        *sp.Timestamp,
				InterruptionRate: p.getInterruptionRate(instanceType),
			})
		}
	}

	return history, nil
}

// GetAvailabilityZones returns available zones for a region.
func (p *AWSSdkPriceProvider) GetAvailabilityZones(ctx context.Context, region string) ([]string, error) {
	if p.initialized && p.ec2Client != nil {
		return p.fetchZonesFromAPI(ctx, region)
	}
	return p.getDefaultZones(region), nil
}

func (p *AWSSdkPriceProvider) fetchZonesFromAPI(ctx context.Context, region string) ([]string, error) {
	input := &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2types.Filter{
			{Name: aws.String("region-name"), Values: []string{region}},
			{Name: aws.String("state"), Values: []string{"available"}},
		},
	}

	result, err := p.ec2Client.DescribeAvailabilityZones(ctx, input)
	if err != nil {
		return nil, err
	}

	zones := make([]string, len(result.AvailabilityZones))
	for i, z := range result.AvailabilityZones {
		zones[i] = *z.ZoneName
	}
	return zones, nil
}

func (p *AWSSdkPriceProvider) getStaticPrice(instanceType string) float64 {
	prices := map[string]float64{
		"t3.nano": 0.0052, "t3.micro": 0.0104, "t3.small": 0.0208, "t3.medium": 0.0416,
		"t3.large": 0.0832, "t3.xlarge": 0.1664, "t3.2xlarge": 0.3328,
		"m5.large": 0.096, "m5.xlarge": 0.192, "m5.2xlarge": 0.384, "m5.4xlarge": 0.768,
		"m6i.large": 0.096, "m6i.xlarge": 0.192,
		"c5.large": 0.085, "c5.xlarge": 0.17, "c5.2xlarge": 0.34,
		"c6i.large": 0.085, "c6i.xlarge": 0.17,
		"r5.large": 0.126, "r5.xlarge": 0.252,
		"r6i.large": 0.126, "r6i.xlarge": 0.252,
		"g4dn.xlarge": 0.526, "p3.2xlarge": 3.06,
	}
	if price, ok := prices[instanceType]; ok {
		return price
	}
	return 0.10
}

func (p *AWSSdkPriceProvider) getInterruptionRate(instanceType string) float64 {
	family := strings.Split(instanceType, ".")[0]
	rates := map[string]float64{
		"t3": 0.05, "t2": 0.05, "m5": 0.08, "m6i": 0.06,
		"c5": 0.10, "c6i": 0.08, "r5": 0.07, "r6i": 0.05,
		"g4dn": 0.15, "p3": 0.20,
	}
	if rate, ok := rates[family]; ok {
		return rate
	}
	return 0.10
}

func (p *AWSSdkPriceProvider) getDefaultZones(region string) []string {
	zoneMap := map[string][]string{
		"us-east-1":      {"us-east-1a", "us-east-1b", "us-east-1c", "us-east-1d", "us-east-1e", "us-east-1f"},
		"us-east-2":      {"us-east-2a", "us-east-2b", "us-east-2c"},
		"us-west-1":      {"us-west-1a", "us-west-1b"},
		"us-west-2":      {"us-west-2a", "us-west-2b", "us-west-2c", "us-west-2d"},
		"eu-west-1":      {"eu-west-1a", "eu-west-1b", "eu-west-1c"},
		"eu-central-1":   {"eu-central-1a", "eu-central-1b", "eu-central-1c"},
		"ap-southeast-1": {"ap-southeast-1a", "ap-southeast-1b", "ap-southeast-1c"},
		"ap-northeast-1": {"ap-northeast-1a", "ap-northeast-1c", "ap-northeast-1d"},
	}
	if zones, ok := zoneMap[region]; ok {
		return zones
	}
	return []string{region + "a", region + "b", region + "c"}
}
