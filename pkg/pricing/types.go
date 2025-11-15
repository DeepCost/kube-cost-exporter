package pricing

import (
	"context"
	"time"
)

// Provider defines the interface for cloud pricing providers
type Provider interface {
	// GetInstancePrice returns the on-demand hourly price for an instance type
	GetInstancePrice(ctx context.Context, instanceType, region, az string) (float64, error)

	// GetSpotPrice returns the current spot price for an instance type
	GetSpotPrice(ctx context.Context, instanceType, region, az string) (float64, error)

	// GetStoragePrice returns the monthly price per GB for storage
	GetStoragePrice(ctx context.Context, storageType, region string) (float64, error)

	// GetNetworkPrice returns the price per GB for network egress
	GetNetworkPrice(ctx context.Context, region, destination string) (float64, error)
}

// PricingCache wraps a provider with caching
type PricingCache struct {
	provider Provider
	cache    map[string]*CacheEntry
}

// CacheEntry represents a cached pricing value
type CacheEntry struct {
	Value     float64
	ExpiresAt time.Time
}

// NodePricing contains all pricing information for a node
type NodePricing struct {
	InstanceType   string
	Region         string
	AvailabilityZone string
	OnDemandPrice  float64
	SpotPrice      float64
	IsSpot         bool
	LastUpdated    time.Time
}

// StoragePricing contains pricing for persistent volumes
type StoragePricing struct {
	StorageClass string
	Region       string
	PricePerGBMonth float64
	LastUpdated  time.Time
}
