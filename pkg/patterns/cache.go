package patterns

import (
	"sync"
	"time"
)

// Cache stores pattern descriptions with in-memory caching
type Cache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

// cacheEntry represents a cached pattern description
type cacheEntry struct {
	pattern   *PatternDescription
	timestamp time.Time
	hits      int
}

// NewCache creates a new cache with specified TTL
func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a pattern from cache
func (c *Cache) Get(signature string) *PatternDescription {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[signature]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Since(entry.timestamp) > c.ttl {
		return nil
	}

	// Update hit count (best effort, no lock upgrade)
	go func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if e, ok := c.entries[signature]; ok {
			e.hits++
		}
	}()

	return entry.pattern
}

// Set stores a pattern in cache
func (c *Cache) Set(signature string, pattern *PatternDescription) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[signature] = &cacheEntry{
		pattern:   pattern,
		timestamp: time.Now(),
		hits:      0,
	}
}

// Has checks if a signature exists in cache
func (c *Cache) Has(signature string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[signature]
	if !exists {
		return false
	}

	// Check if expired
	return time.Since(entry.timestamp) <= c.ttl
}

// Delete removes a signature from cache
func (c *Cache) Delete(signature string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, signature)
}

// Clear removes all entries from cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Size returns the number of entries in cache
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// Stats returns cache statistics
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalHits, validEntries int
	oldestTime := time.Now()

	for _, entry := range c.entries {
		if time.Since(entry.timestamp) <= c.ttl {
			validEntries++
			totalHits += entry.hits
			if entry.timestamp.Before(oldestTime) {
				oldestTime = entry.timestamp
			}
		}
	}

	return CacheStats{
		Size:        len(c.entries),
		ValidSize:   validEntries,
		TotalHits:   totalHits,
		OldestEntry: oldestTime,
	}
}

// CacheStats contains cache statistics
type CacheStats struct {
	Size        int
	ValidSize   int
	TotalHits   int
	OldestEntry time.Time
}

// cleanupLoop periodically removes expired entries
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for sig, entry := range c.entries {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.entries, sig)
		}
	}
}

// GetOrSet atomically gets a value or sets it if it doesn't exist
// Returns (value, true) if found in cache, (nil, false) if not
func (c *Cache) GetOrSet(signature string, compute func() (*PatternDescription, error)) (*PatternDescription, bool, error) {
	// Fast path: check cache with read lock
	if pattern := c.Get(signature); pattern != nil {
		return pattern, true, nil
	}

	// Slow path: compute and store
	pattern, err := compute()
	if err != nil {
		return nil, false, err
	}

	c.Set(signature, pattern)
	return pattern, false, nil
}

// HitRate returns cache hit rate (0-1) based on recent activity
// Note: This is approximate since we don't track misses explicitly
func (c *Cache) HitRate() float64 {
	stats := c.Stats()
	if stats.Size == 0 {
		return 0.0
	}

	// Rough estimate: if entries have many hits, cache is effective
	avgHits := float64(stats.TotalHits) / float64(stats.ValidSize)

	// Normalize to 0-1 range (assume 10+ hits = good cache performance)
	rate := avgHits / 10.0
	if rate > 1.0 {
		rate = 1.0
	}

	return rate
}
