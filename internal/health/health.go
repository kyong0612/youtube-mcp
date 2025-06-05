package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/youtube-transcript-mcp/internal/cache"
	"github.com/youtube-transcript-mcp/internal/youtube"
)

// Checker performs health checks on various system components
type Checker struct {
	cache   cache.Cache
	youtube *youtube.Service
	mu      sync.RWMutex
	checks  map[string]CheckResult
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Status    string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status    string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Checks    map[string]CheckResult `json:"checks"`
	TotalMS   int64                  `json:"total_ms"`
}

// NewChecker creates a new health checker
func NewChecker(cache cache.Cache, youtube *youtube.Service) *Checker {
	return &Checker{
		cache:   cache,
		youtube: youtube,
		checks:  make(map[string]CheckResult),
	}
}

// CheckHealth performs all health checks
func (c *Checker) CheckHealth(ctx context.Context) *HealthStatus {
	start := time.Now()

	// Run all checks in parallel
	var wg sync.WaitGroup
	checkFuncs := map[string]func(context.Context) CheckResult{
		"cache":   c.checkCache,
		"youtube": c.checkYouTube,
		"network": c.checkNetwork,
	}

	results := make(map[string]CheckResult)
	var mu sync.Mutex

	for name, checkFunc := range checkFuncs {
		wg.Add(1)
		go func(name string, fn func(context.Context) CheckResult) {
			defer wg.Done()

			// Use a timeout for each check
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			result := fn(checkCtx)

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, checkFunc)
	}

	wg.Wait()

	// Update cached results
	c.mu.Lock()
	c.checks = results
	c.mu.Unlock()

	// Determine overall status
	overallStatus := "healthy"
	for _, result := range results {
		if result.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		} else if result.Status == "degraded" && overallStatus == "healthy" {
			overallStatus = "degraded"
		}
	}

	return &HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now().UTC(),
		Checks:    results,
		TotalMS:   time.Since(start).Milliseconds(),
	}
}

// checkCache verifies the cache is working
func (c *Checker) checkCache(ctx context.Context) CheckResult {
	start := time.Now()

	// Try to set and get a test value
	testKey := "_health_check_test"
	testValue := time.Now().UnixNano()

	// Set the value
	if err := c.cache.Set(ctx, testKey, testValue, 1*time.Minute); err != nil {
		return CheckResult{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Failed to set cache value: %v", err),
			Timestamp: time.Now().UTC(),
			Details: map[string]interface{}{
				"operation":  "set",
				"latency_ms": time.Since(start).Milliseconds(),
			},
		}
	}

	// Get the value
	value, found := c.cache.Get(ctx, testKey)
	if !found {
		return CheckResult{
			Status:    "unhealthy",
			Message:   "Failed to retrieve cached value",
			Timestamp: time.Now().UTC(),
			Details: map[string]interface{}{
				"operation":  "get",
				"latency_ms": time.Since(start).Milliseconds(),
			},
		}
	}

	// Verify the value
	if retrievedValue, ok := value.(int64); !ok || retrievedValue != testValue {
		return CheckResult{
			Status:    "unhealthy",
			Message:   "Cache returned incorrect value",
			Timestamp: time.Now().UTC(),
			Details: map[string]interface{}{
				"expected":   testValue,
				"actual":     value,
				"latency_ms": time.Since(start).Milliseconds(),
			},
		}
	}

	// Clean up
	c.cache.Delete(ctx, testKey)

	// Get cache size
	size := c.cache.Size(ctx)

	return CheckResult{
		Status:    "healthy",
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"cache_size": size,
			"latency_ms": time.Since(start).Milliseconds(),
		},
	}
}

// checkYouTube verifies YouTube service connectivity
func (c *Checker) checkYouTube(ctx context.Context) CheckResult {
	start := time.Now()

	// Try to extract a video ID (this doesn't make any network calls)
	testVideoID := "dQw4w9WgXcQ"
	_, err := c.youtube.ListAvailableLanguages(ctx, testVideoID)

	if err != nil {
		// Check if it's a network error or YouTube-specific error
		errorType := "unknown"
		if err.Error() != "" {
			if contains(err.Error(), "network") || contains(err.Error(), "timeout") {
				errorType = "network"
			} else if contains(err.Error(), "unavailable") || contains(err.Error(), "not found") {
				errorType = "youtube"
			}
		}

		return CheckResult{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("YouTube service check failed: %v", err),
			Timestamp: time.Now().UTC(),
			Details: map[string]interface{}{
				"error_type": errorType,
				"latency_ms": time.Since(start).Milliseconds(),
			},
		}
	}

	latency := time.Since(start).Milliseconds()
	status := "healthy"
	if latency > 5000 {
		status = "degraded"
	}

	return CheckResult{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"latency_ms": latency,
		},
	}
}

// checkNetwork verifies basic network connectivity
func (c *Checker) checkNetwork(ctx context.Context) CheckResult {
	start := time.Now()

	// Try to reach YouTube
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://www.youtube.com", nil)
	if err != nil {
		return CheckResult{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Failed to create request: %v", err),
			Timestamp: time.Now().UTC(),
		}
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return CheckResult{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Network check failed: %v", err),
			Timestamp: time.Now().UTC(),
			Details: map[string]interface{}{
				"latency_ms": time.Since(start).Milliseconds(),
			},
		}
	}
	defer resp.Body.Close()

	latency := time.Since(start).Milliseconds()
	status := "healthy"
	message := ""

	if resp.StatusCode >= 400 {
		status = "degraded"
		message = fmt.Sprintf("YouTube returned status %d", resp.StatusCode)
	} else if latency > 3000 {
		status = "degraded"
		message = "High network latency"
	}

	return CheckResult{
		Status:    status,
		Message:   message,
		Timestamp: time.Now().UTC(),
		Details: map[string]interface{}{
			"status_code": resp.StatusCode,
			"latency_ms":  latency,
		},
	}
}

// IsHealthy returns true if all checks are passing
func (c *Checker) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, check := range c.checks {
		if check.Status == "unhealthy" {
			return false
		}
	}
	return true
}

// IsReady returns true if the service is ready to handle requests
func (c *Checker) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Service is ready if cache is healthy
	if cacheCheck, ok := c.checks["cache"]; ok {
		return cacheCheck.Status == "healthy"
	}
	return false
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0:len(substr)] == substr || contains(s[1:], substr)))
}
