// Package cost provides price caching functionality.
package cost

import (
	"sync"
	"time"
)

// priceCache provides thread-safe caching for pricing data.
type priceCache struct {
	mu        sync.RWMutex
	ttl       time.Duration
	prices    map[string]cachedPrice
	spotCache map[string]cachedSpotPrice
}

type cachedPrice struct {
	price     float64
	expiresAt time.Time
}

type cachedSpotPrice struct {
	price     *SpotPrice
	expiresAt time.Time
}

func newPriceCache(ttl time.Duration) *priceCache {
	return &priceCache{
		ttl:       ttl,
		prices:    make(map[string]cachedPrice),
		spotCache: make(map[string]cachedSpotPrice),
	}
}

func (c *priceCache) get(key string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if cached, ok := c.prices[key]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.price, true
		}
	}
	return 0, false
}

func (c *priceCache) set(key string, price float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prices[key] = cachedPrice{price: price, expiresAt: time.Now().Add(c.ttl)}
}

func (c *priceCache) getSpot(key string) (*SpotPrice, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if cached, ok := c.spotCache[key]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.price, true
		}
	}
	return nil, false
}

func (c *priceCache) setSpot(key string, price *SpotPrice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.spotCache[key] = cachedSpotPrice{price: price, expiresAt: time.Now().Add(c.ttl)}
}
