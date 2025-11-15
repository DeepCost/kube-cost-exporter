package pricing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NewPricingCache creates a new pricing cache
func NewPricingCache(provider Provider) *PricingCache {
	return &PricingCache{
		provider: provider,
		cache:    make(map[string]*CacheEntry),
	}
}

var cacheMutex sync.RWMutex

// GetInstancePrice returns cached instance price or fetches from provider
func (pc *PricingCache) GetInstancePrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	key := fmt.Sprintf("instance:%s:%s:%s", instanceType, region, az)
	ttl := 1 * time.Hour

	return pc.getOrFetch(ctx, key, ttl, func() (float64, error) {
		return pc.provider.GetInstancePrice(ctx, instanceType, region, az)
	})
}

// GetSpotPrice returns cached spot price or fetches from provider
func (pc *PricingCache) GetSpotPrice(ctx context.Context, instanceType, region, az string) (float64, error) {
	key := fmt.Sprintf("spot:%s:%s:%s", instanceType, region, az)
	ttl := 5 * time.Minute

	return pc.getOrFetch(ctx, key, ttl, func() (float64, error) {
		return pc.provider.GetSpotPrice(ctx, instanceType, region, az)
	})
}

// GetStoragePrice returns cached storage price or fetches from provider
func (pc *PricingCache) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	key := fmt.Sprintf("storage:%s:%s", storageType, region)
	ttl := 24 * time.Hour

	return pc.getOrFetch(ctx, key, ttl, func() (float64, error) {
		return pc.provider.GetStoragePrice(ctx, storageType, region)
	})
}

// GetNetworkPrice returns cached network price or fetches from provider
func (pc *PricingCache) GetNetworkPrice(ctx context.Context, region, destination string) (float64, error) {
	key := fmt.Sprintf("network:%s:%s", region, destination)
	ttl := 1 * time.Hour

	return pc.getOrFetch(ctx, key, ttl, func() (float64, error) {
		return pc.provider.GetNetworkPrice(ctx, region, destination)
	})
}

// getOrFetch gets from cache or fetches and caches
func (pc *PricingCache) getOrFetch(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (float64, error)) (float64, error) {
	// Try to get from cache
	cacheMutex.RLock()
	entry, exists := pc.cache[key]
	cacheMutex.RUnlock()

	if exists && time.Now().Before(entry.ExpiresAt) {
		return entry.Value, nil
	}

	// Fetch from provider
	value, err := fetchFunc()
	if err != nil {
		return 0, err
	}

	// Store in cache
	cacheMutex.Lock()
	pc.cache[key] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	cacheMutex.Unlock()

	return value, nil
}
