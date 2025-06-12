// Package cache provides caching interfaces and implementations.
package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) (any, bool)

	// Set stores a value in cache with TTL
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes a value from cache
	Delete(ctx context.Context, key string) error

	// Clear removes all values from cache
	Clear(ctx context.Context) error

	// Size returns the number of items in cache
	Size(ctx context.Context) int

	// Close closes the cache and releases resources
	Close() error
}
