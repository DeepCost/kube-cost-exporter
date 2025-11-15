package pricing

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
)

// GCPProvider implements the Provider interface for Google Cloud Platform
type GCPProvider struct {
	project string
	logger  *logrus.Logger
}

// NewGCPProvider creates a new GCP pricing provider
func NewGCPProvider(project string) (*GCPProvider, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &GCPProvider{
		project: project,
		logger:  logger,
	}, nil
}

// GetInstancePrice returns the on-demand hourly price for a GCE instance
func (g *GCPProvider) GetInstancePrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	// Use fallback pricing for GCP instances
	// In production, this would use the Cloud Billing API
	return g.getFallbackPrice(instanceType, region), nil
}

// GetSpotPrice returns the preemptible VM price
func (g *GCPProvider) GetSpotPrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	// Preemptible VMs are typically 60-91% cheaper than regular instances
	onDemand, _ := g.GetInstancePrice(ctx, instanceType, region, az)
	return onDemand * 0.30, nil // Approximately 70% discount
}

// GetStoragePrice returns the price per GB/month for persistent disks
func (g *GCPProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	// GCP storage pricing (per GB/month)
	fallbackPrices := map[string]float64{
		"pd-standard": 0.040, // Standard persistent disk
		"pd-balanced": 0.100, // Balanced persistent disk
		"pd-ssd":      0.170, // SSD persistent disk
		"pd-extreme":  0.125, // Extreme persistent disk (per IOPS provisioned)
	}

	if price, ok := fallbackPrices[storageType]; ok {
		return price, nil
	}

	return 0.100, nil // Default to balanced pricing
}

// GetNetworkPrice returns the price per GB for network egress
func (g *GCPProvider) GetNetworkPrice(ctx context.Context, region, destination string) (float64, error) {
	// GCP network pricing (per GB)
	// Egress within same region: free
	// Egress to different region in same continent: $0.01
	// Egress to internet (first 1TB): $0.12
	if destination == "" || destination == region {
		return 0.0, nil
	}
	return 0.12, nil
}

// getFallbackPrice returns fallback pricing for common GCP instance types
func (g *GCPProvider) getFallbackPrice(instanceType, region string) float64 {
	// Extract machine family and size
	// Format: n1-standard-1, n2-standard-4, e2-medium, etc.

	fallbackPrices := map[string]float64{
		// e2 family (cost-optimized)
		"e2-micro":     0.0084,
		"e2-small":     0.0167,
		"e2-medium":    0.0334,
		"e2-standard-2": 0.0669,
		"e2-standard-4": 0.1338,
		"e2-standard-8": 0.2676,
		"e2-standard-16": 0.5352,

		// n1 family (general purpose)
		"n1-standard-1":  0.0475,
		"n1-standard-2":  0.0950,
		"n1-standard-4":  0.1900,
		"n1-standard-8":  0.3800,
		"n1-standard-16": 0.7600,
		"n1-standard-32": 1.5200,
		"n1-standard-64": 3.0400,

		// n2 family (newer general purpose)
		"n2-standard-2":  0.0971,
		"n2-standard-4":  0.1942,
		"n2-standard-8":  0.3884,
		"n2-standard-16": 0.7768,
		"n2-standard-32": 1.5536,
		"n2-standard-64": 3.1072,

		// c2 family (compute-optimized)
		"c2-standard-4":  0.2088,
		"c2-standard-8":  0.4176,
		"c2-standard-16": 0.8352,
		"c2-standard-30": 1.5660,
		"c2-standard-60": 3.1320,

		// m1 family (memory-optimized)
		"m1-megamem-96":   10.6740,
		"m1-ultramem-40":  6.3039,
		"m1-ultramem-80":  12.6078,
		"m1-ultramem-160": 25.2156,
	}

	if price, ok := fallbackPrices[instanceType]; ok {
		// Adjust for region (some regions are more expensive)
		regionMultiplier := 1.0
		if strings.Contains(region, "asia") {
			regionMultiplier = 1.1
		} else if strings.Contains(region, "australia") {
			regionMultiplier = 1.2
		}
		return price * regionMultiplier
	}

	// Estimate based on instance family
	if strings.HasPrefix(instanceType, "e2-") {
		return 0.05
	} else if strings.HasPrefix(instanceType, "n1-") {
		return 0.10
	} else if strings.HasPrefix(instanceType, "n2-") {
		return 0.12
	} else if strings.HasPrefix(instanceType, "c2-") {
		return 0.20
	}

	return 0.10 // Default fallback
}
