package cache

import (
	"context"
	"sync"
	"time"

	"github.com/youtube-transcript-mcp/internal/models"
)

// MemoryCache implements an in-memory cache
type MemoryCache struct {
	mu              sync.RWMutex
	items           map[string]*models.CacheEntry
	maxSize         int
	maxMemory       int64
	currentMemory   int64
	stopCh          chan struct{}
	cleanupInterval time.Duration
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache(maxSize int, maxMemoryMB int, cleanupInterval time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items:           make(map[string]*models.CacheEntry),
		maxSize:         maxSize,
		maxMemory:       int64(maxMemoryMB) * 1024 * 1024,
		stopCh:          make(chan struct{}),
		cleanupInterval: cleanupInterval,
	}

	// Start cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// Get retrieves a value from cache
func (mc *MemoryCache) Get(_ context.Context, key string) (interface{}, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.items[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.Timestamp) > entry.TTL {
		// Don't delete here to avoid deadlock, let cleanup handle it
		return nil, false
	}

	// Update hit count
	entry.HitCount++

	return entry.Data, true
}

// Set stores a value in cache with TTL
func (mc *MemoryCache) Set(_ context.Context, key string, value interface{}, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check size limit
	if len(mc.items) >= mc.maxSize {
		// Evict least recently used item
		mc.evictLRU()
	}

	entry := &models.CacheEntry{
		Key:       key,
		Data:      value,
		Timestamp: time.Now(),
		TTL:       ttl,
		HitCount:  0,
	}

	mc.items[key] = entry
	return nil
}

// Delete removes a value from cache
func (mc *MemoryCache) Delete(_ context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.items, key)
	return nil
}

// Clear removes all values from cache
func (mc *MemoryCache) Clear(_ context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*models.CacheEntry)
	mc.currentMemory = 0
	return nil
}

// Size returns the number of items in cache
func (mc *MemoryCache) Size(_ context.Context) int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return len(mc.items)
}

// Close closes the cache and releases resources
func (mc *MemoryCache) Close() error {
	close(mc.stopCh)
	return nil
}

// cleanupLoop periodically removes expired entries
func (mc *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(mc.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.cleanup()
		case <-mc.stopCh:
			return
		}
	}
}

// cleanup removes expired entries
func (mc *MemoryCache) cleanup() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, entry := range mc.items {
		if now.Sub(entry.Timestamp) > entry.TTL {
			delete(mc.items, key)
		}
	}
}

// evictLRU evicts the least recently used item
func (mc *MemoryCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range mc.items {
		if oldestKey == "" || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
		}
	}

	if oldestKey != "" {
		delete(mc.items, oldestKey)
	}
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	totalHits := 0
	for _, entry := range mc.items {
		totalHits += entry.HitCount
	}

	return map[string]interface{}{
		"size":          len(mc.items),
		"maxSize":       mc.maxSize,
		"totalHits":     totalHits,
		"currentMemory": mc.currentMemory,
		"maxMemory":     mc.maxMemory,
	}
}
