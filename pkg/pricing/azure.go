package pricing

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// AzureProvider implements the Provider interface for Microsoft Azure
type AzureProvider struct {
	subscriptionID string
	logger         *logrus.Logger
}

// NewAzureProvider creates a new Azure pricing provider
func NewAzureProvider(subscriptionID string) (*AzureProvider, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &AzureProvider{
		subscriptionID: subscriptionID,
		logger:         logger,
	}, nil
}

// GetInstancePrice returns the on-demand hourly price for an Azure VM
func (a *AzureProvider) GetInstancePrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	// Use fallback pricing for Azure VMs
	// In production, this would use the Azure Retail Prices API
	return a.getFallbackPrice(instanceType, region), nil
}

// GetSpotPrice returns the spot VM price
func (a *AzureProvider) GetSpotPrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	// Azure Spot VMs typically offer 60-90% discount
	onDemand, _ := a.GetInstancePrice(ctx, instanceType, region, az)
	return onDemand * 0.30, nil // Approximately 70% discount
}

// GetStoragePrice returns the price per GB/month for managed disks
func (a *AzureProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	// Azure managed disk pricing (per GB/month)
	fallbackPrices := map[string]float64{
		"Standard_LRS":    0.040,  // Standard HDD
		"StandardSSD_LRS": 0.075,  // Standard SSD
		"Premium_LRS":     0.135,  // Premium SSD
		"UltraSSD_LRS":    0.000125, // Ultra SSD (per provisioned IOPS)
	}

	if price, ok := fallbackPrices[storageType]; ok {
		return price, nil
	}

	return 0.075, nil // Default to Standard SSD pricing
}

// GetNetworkPrice returns the price per GB for network egress
func (a *AzureProvider) GetNetworkPrice(ctx context.Context, region, destination string) (float64, error) {
	// Azure network pricing (per GB)
	// First 5 GB/month: free
	// 5 GB - 10 TB: $0.087
	// 10 TB - 50 TB: $0.083
	// Over 50 TB: $0.081
	return 0.087, nil
}

// getFallbackPrice returns fallback pricing for common Azure instance types
func (a *AzureProvider) getFallbackPrice(instanceType, region string) float64 {
	// Azure VM pricing varies by series and size

	fallbackPrices := map[string]float64{
		// B-series (burstable)
		"Standard_B1s":  0.0104,
		"Standard_B1ms": 0.0207,
		"Standard_B2s":  0.0416,
		"Standard_B2ms": 0.0832,
		"Standard_B4ms": 0.1664,
		"Standard_B8ms": 0.3328,

		// D-series (general purpose)
		"Standard_D2s_v3":  0.096,
		"Standard_D4s_v3":  0.192,
		"Standard_D8s_v3":  0.384,
		"Standard_D16s_v3": 0.768,
		"Standard_D32s_v3": 1.536,
		"Standard_D48s_v3": 2.304,
		"Standard_D64s_v3": 3.072,

		// F-series (compute-optimized)
		"Standard_F2s_v2":  0.085,
		"Standard_F4s_v2":  0.169,
		"Standard_F8s_v2":  0.338,
		"Standard_F16s_v2": 0.677,
		"Standard_F32s_v2": 1.353,
		"Standard_F48s_v2": 2.030,
		"Standard_F64s_v2": 2.706,

		// E-series (memory-optimized)
		"Standard_E2s_v3":  0.126,
		"Standard_E4s_v3":  0.252,
		"Standard_E8s_v3":  0.504,
		"Standard_E16s_v3": 1.008,
		"Standard_E32s_v3": 2.016,
		"Standard_E48s_v3": 3.024,
		"Standard_E64s_v3": 4.032,

		// N-series (GPU)
		"Standard_NC6":    0.90,
		"Standard_NC12":   1.80,
		"Standard_NC24":   3.60,
		"Standard_NC6s_v3": 3.06,
	}

	if price, ok := fallbackPrices[instanceType]; ok {
		// Adjust for region
		regionMultiplier := 1.0
		if strings.Contains(region, "eastus") || strings.Contains(region, "westus") {
			regionMultiplier = 1.0
		} else if strings.Contains(region, "europe") {
			regionMultiplier = 1.1
		} else if strings.Contains(region, "asia") {
			regionMultiplier = 1.15
		}
		return price * regionMultiplier
	}

	// Estimate based on instance series
	if strings.Contains(instanceType, "B1") {
		return 0.02
	} else if strings.Contains(instanceType, "B2") {
		return 0.05
	} else if strings.Contains(instanceType, "D2") || strings.Contains(instanceType, "F2") {
		return 0.10
	} else if strings.Contains(instanceType, "D4") || strings.Contains(instanceType, "F4") {
		return 0.20
	} else if strings.Contains(instanceType, "E2") {
		return 0.13
	}

	return 0.10 // Default fallback
}
