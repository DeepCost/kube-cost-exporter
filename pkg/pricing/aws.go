package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	pricingTypes "github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/sirupsen/logrus"
)

// AWSProvider implements the Provider interface for AWS
type AWSProvider struct {
	ec2Client     *ec2.Client
	pricingClient *pricing.Client
	region        string
	logger        *logrus.Logger
}

// NewAWSProvider creates a new AWS pricing provider
func NewAWSProvider(region string) (*AWSProvider, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Pricing API is only available in us-east-1
	pricingCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS pricing config: %w", err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &AWSProvider{
		ec2Client:     ec2.NewFromConfig(cfg),
		pricingClient: pricing.NewFromConfig(pricingCfg),
		region:        region,
		logger:        logger,
	}, nil
}

// GetInstancePrice returns the on-demand hourly price for an EC2 instance
func (a *AWSProvider) GetInstancePrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	// Use the pricing API to get on-demand pricing
	filters := []pricingTypes.Filter{
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("ServiceCode"),
			Value: aws.String("AmazonEC2"),
		},
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("instanceType"),
			Value: aws.String(instanceType),
		},
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("location"),
			Value: aws.String(a.regionToLocation(region)),
		},
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("tenancy"),
			Value: aws.String("Shared"),
		},
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("operatingSystem"),
			Value: aws.String("Linux"),
		},
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("preInstalledSw"),
			Value: aws.String("NA"),
		},
		{
			Type:  pricingTypes.FilterTypeTermMatch,
			Field: aws.String("capacitystatus"),
			Value: aws.String("Used"),
		},
	}

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters:     filters,
		MaxResults:  aws.Int32(1),
	}

	result, err := a.pricingClient.GetProducts(ctx, input)
	if err != nil {
		a.logger.Warnf("Failed to get pricing from API for %s: %v, using fallback", instanceType, err)
		return a.getFallbackPrice(instanceType), nil
	}

	if len(result.PriceList) == 0 {
		a.logger.Warnf("No pricing found for %s, using fallback", instanceType)
		return a.getFallbackPrice(instanceType), nil
	}

	// Parse the pricing JSON
	var priceData map[string]interface{}
	if err := json.Unmarshal([]byte(result.PriceList[0]), &priceData); err != nil {
		return 0, fmt.Errorf("failed to parse pricing data: %w", err)
	}

	// Extract the on-demand price
	price, err := a.extractOnDemandPrice(priceData)
	if err != nil {
		a.logger.Warnf("Failed to extract price for %s: %v, using fallback", instanceType, err)
		return a.getFallbackPrice(instanceType), nil
	}

	return price, nil
}

// GetSpotPrice returns the current spot price for an instance
func (a *AWSProvider) GetSpotPrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	input := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       []types.InstanceType{types.InstanceType(instanceType)},
		ProductDescriptions: []string{"Linux/UNIX"},
		MaxResults:          aws.Int32(1),
	}

	if az != "" {
		input.AvailabilityZone = aws.String(az)
	}

	result, err := a.ec2Client.DescribeSpotPriceHistory(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get spot price: %w", err)
	}

	if len(result.SpotPriceHistory) == 0 {
		// Return 70% of on-demand as estimated spot price
		onDemand, _ := a.GetInstancePrice(ctx, instanceType, region, az)
		return onDemand * 0.7, nil
	}

	price, err := strconv.ParseFloat(*result.SpotPriceHistory[0].SpotPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse spot price: %w", err)
	}

	return price, nil
}

// GetStoragePrice returns the price per GB/month for EBS storage
func (a *AWSProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	// Fallback pricing for common EBS types (per GB/month)
	fallbackPrices := map[string]float64{
		"gp2":      0.10, // General Purpose SSD
		"gp3":      0.08, // General Purpose SSD (newer)
		"io1":      0.125, // Provisioned IOPS SSD
		"io2":      0.125, // Provisioned IOPS SSD (newer)
		"st1":      0.045, // Throughput Optimized HDD
		"sc1":      0.025, // Cold HDD
		"standard": 0.05,  // Magnetic
	}

	if price, ok := fallbackPrices[storageType]; ok {
		return price, nil
	}

	return 0.10, nil // Default to gp2 price
}

// GetNetworkPrice returns the price per GB for network egress
func (a *AWSProvider) GetNetworkPrice(ctx context.Context, region, destination string) (float64, error) {
	// AWS network pricing (per GB)
	// First 10 TB: $0.09
	// Next 40 TB: $0.085
	// Over 150 TB: $0.07
	// For simplicity, we use the first tier
	return 0.09, nil
}

// Helper functions

func (a *AWSProvider) regionToLocation(region string) string {
	// Map AWS regions to pricing API location names
	locations := map[string]string{
		"us-east-1":      "US East (N. Virginia)",
		"us-east-2":      "US East (Ohio)",
		"us-west-1":      "US West (N. California)",
		"us-west-2":      "US West (Oregon)",
		"eu-west-1":      "EU (Ireland)",
		"eu-central-1":   "EU (Frankfurt)",
		"ap-southeast-1": "Asia Pacific (Singapore)",
		"ap-southeast-2": "Asia Pacific (Sydney)",
		"ap-northeast-1": "Asia Pacific (Tokyo)",
	}

	if loc, ok := locations[region]; ok {
		return loc
	}
	return "US East (N. Virginia)" // Default
}

func (a *AWSProvider) extractOnDemandPrice(priceData map[string]interface{}) (float64, error) {
	terms, ok := priceData["terms"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("no terms in pricing data")
	}

	onDemand, ok := terms["OnDemand"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("no OnDemand terms")
	}

	// Get the first (and usually only) offer term
	for _, offerTerm := range onDemand {
		offerTermMap, ok := offerTerm.(map[string]interface{})
		if !ok {
			continue
		}

		priceDimensions, ok := offerTermMap["priceDimensions"].(map[string]interface{})
		if !ok {
			continue
		}

		// Get the first price dimension
		for _, dimension := range priceDimensions {
			dimensionMap, ok := dimension.(map[string]interface{})
			if !ok {
				continue
			}

			pricePerUnit, ok := dimensionMap["pricePerUnit"].(map[string]interface{})
			if !ok {
				continue
			}

			usdPrice, ok := pricePerUnit["USD"].(string)
			if !ok {
				continue
			}

			price, err := strconv.ParseFloat(usdPrice, 64)
			if err != nil {
				return 0, err
			}

			return price, nil
		}
	}

	return 0, fmt.Errorf("could not extract price from terms")
}

func (a *AWSProvider) getFallbackPrice(instanceType string) float64 {
	// Fallback prices for common instance types (hourly USD)
	fallbackPrices := map[string]float64{
		// t3 family
		"t3.micro":   0.0104,
		"t3.small":   0.0208,
		"t3.medium":  0.0416,
		"t3.large":   0.0832,
		"t3.xlarge":  0.1664,
		"t3.2xlarge": 0.3328,

		// m5 family
		"m5.large":    0.096,
		"m5.xlarge":   0.192,
		"m5.2xlarge":  0.384,
		"m5.4xlarge":  0.768,
		"m5.8xlarge":  1.536,
		"m5.12xlarge": 2.304,
		"m5.16xlarge": 3.072,
		"m5.24xlarge": 4.608,

		// c5 family
		"c5.large":    0.085,
		"c5.xlarge":   0.17,
		"c5.2xlarge":  0.34,
		"c5.4xlarge":  0.68,
		"c5.9xlarge":  1.53,
		"c5.12xlarge": 2.04,
		"c5.18xlarge": 3.06,
		"c5.24xlarge": 4.08,

		// r5 family
		"r5.large":    0.126,
		"r5.xlarge":   0.252,
		"r5.2xlarge":  0.504,
		"r5.4xlarge":  1.008,
		"r5.8xlarge":  2.016,
		"r5.12xlarge": 3.024,
		"r5.16xlarge": 4.032,
		"r5.24xlarge": 6.048,
	}

	if price, ok := fallbackPrices[instanceType]; ok {
		return price
	}

	// Estimate based on instance size if not in table
	if strings.Contains(instanceType, "micro") {
		return 0.01
	} else if strings.Contains(instanceType, "small") {
		return 0.02
	} else if strings.Contains(instanceType, "medium") {
		return 0.04
	} else if strings.Contains(instanceType, "large") && !strings.Contains(instanceType, "xlarge") {
		return 0.10
	} else if strings.Contains(instanceType, "xlarge") {
		return 0.20
	}

	return 0.10 // Default fallback
}
