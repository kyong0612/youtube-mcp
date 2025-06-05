package cache

import (
	"context"
	"testing"
	"time"

	"github.com/youtube-transcript-mcp/internal/models"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	cache := NewMemoryCache(100, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	// Test setting and getting a value
	key := "test:key"
	value := "test value"
	ttl := time.Minute

	err := cache.Set(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	got, found := cache.Get(ctx, key)
	if !found {
		t.Fatal("Expected to find value but got not found")
	}

	if got.(string) != value {
		t.Errorf("Expected value %s, got %v", value, got)
	}
}

func TestMemoryCache_GetNonExistent(t *testing.T) {
	cache := NewMemoryCache(100, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	_, found := cache.Get(ctx, "non-existent")
	if found {
		t.Error("Expected not found for non-existent key")
	}
}

func TestMemoryCache_Expiration(t *testing.T) {
	cache := NewMemoryCache(100, 100, 50*time.Millisecond)
	defer cache.Close()

	ctx := context.Background()

	// Set value with short TTL
	key := "expire:key"
	value := "will expire"
	ttl := 100 * time.Millisecond

	err := cache.Set(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Value should exist immediately
	_, found := cache.Get(ctx, key)
	if !found {
		t.Error("Expected to find value immediately after setting")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Value should be expired
	_, found = cache.Get(ctx, key)
	if found {
		t.Error("Expected value to be expired")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(100, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	key := "delete:key"
	value := "to be deleted"

	// Set value
	err := cache.Set(ctx, key, value, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Verify it exists
	_, found := cache.Get(ctx, key)
	if !found {
		t.Fatal("Expected to find value before deletion")
	}

	// Delete
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// Verify it's gone
	_, found = cache.Get(ctx, key)
	if found {
		t.Error("Expected value to be deleted")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(100, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	// Add multiple values
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		err := cache.Set(ctx, key, i, time.Hour)
		if err != nil {
			t.Fatalf("Failed to set value %d: %v", i, err)
		}
	}

	// Verify size
	if size := cache.Size(ctx); size != 5 {
		t.Errorf("Expected size 5, got %d", size)
	}

	// Clear cache
	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify empty
	if size := cache.Size(ctx); size != 0 {
		t.Errorf("Expected size 0 after clear, got %d", size)
	}
}

func TestMemoryCache_MaxSize(t *testing.T) {
	maxSize := 3
	cache := NewMemoryCache(maxSize, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	// Fill cache to max
	for i := 0; i < maxSize+2; i++ {
		key := string(rune('a' + i))
		err := cache.Set(ctx, key, i, time.Hour)
		if err != nil {
			t.Fatalf("Failed to set value %d: %v", i, err)
		}
	}

	// Should not exceed max size
	if size := cache.Size(ctx); size > maxSize {
		t.Errorf("Cache size %d exceeds max size %d", size, maxSize)
	}

	// Oldest entries should be evicted
	_, found := cache.Get(ctx, "a")
	if found {
		t.Error("Expected oldest entry 'a' to be evicted")
	}

	// Newest entries should still exist
	_, found = cache.Get(ctx, "e")
	if !found {
		t.Error("Expected newest entry 'e' to exist")
	}
}

func TestMemoryCache_HitCount(t *testing.T) {
	cache := NewMemoryCache(100, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	key := "hit:key"
	value := "test"

	// Set value
	err := cache.Set(ctx, key, value, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get multiple times
	for i := 0; i < 5; i++ {
		_, found := cache.Get(ctx, key)
		if !found {
			t.Fatal("Expected to find value")
		}
	}

	// Check hit count in stats
	stats := cache.Stats()
	if totalHits, ok := stats["totalHits"].(int); !ok || totalHits < 5 {
		t.Errorf("Expected at least 5 hits, got %v", stats["totalHits"])
	}
}

func TestMemoryCache_ComplexTypes(t *testing.T) {
	cache := NewMemoryCache(100, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()

	// Test with TranscriptResponse
	transcript := &models.TranscriptResponse{
		VideoID:  "test123",
		Title:    "Test Video",
		Language: "en",
		Transcript: []models.TranscriptSegment{
			{Text: "Hello", Start: 0, Duration: 2},
			{Text: "World", Start: 2, Duration: 2},
		},
		WordCount: 2,
		Metadata: models.TranscriptMetadata{
			ExtractionTimestamp: time.Now(),
			Source:              "test",
		},
	}

	key := "transcript:test123"
	err := cache.Set(ctx, key, transcript, time.Hour)
	if err != nil {
		t.Fatalf("Failed to set transcript: %v", err)
	}

	got, found := cache.Get(ctx, key)
	if !found {
		t.Fatal("Expected to find transcript")
	}

	gotTranscript, ok := got.(*models.TranscriptResponse)
	if !ok {
		t.Fatal("Expected TranscriptResponse type")
	}

	if gotTranscript.VideoID != transcript.VideoID {
		t.Errorf("Expected video ID %s, got %s", transcript.VideoID, gotTranscript.VideoID)
	}
	if len(gotTranscript.Transcript) != len(transcript.Transcript) {
		t.Errorf("Expected %d segments, got %d", len(transcript.Transcript), len(gotTranscript.Transcript))
	}
}

func TestMemoryCache_Cleanup(t *testing.T) {
	// Short cleanup interval for testing
	cache := NewMemoryCache(100, 100, 100*time.Millisecond)
	defer cache.Close()

	ctx := context.Background()

	// Add expired entry
	key := "cleanup:key"
	err := cache.Set(ctx, key, "value", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Wait for expiration and cleanup
	time.Sleep(200 * time.Millisecond)

	// Should be cleaned up
	_, found := cache.Get(ctx, key)
	if found {
		t.Error("Expected expired entry to be cleaned up")
	}
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache(1000, 100, time.Hour)
	defer cache.Close()

	ctx := context.Background()
	done := make(chan bool)

	// Multiple goroutines setting values
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j%10))
				cache.Set(ctx, key, id*100+j, time.Hour)
			}
			done <- true
		}(i)
	}

	// Multiple goroutines getting values
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a'+id)) + string(rune('0'+j%10))
				cache.Get(ctx, key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Cache should still be functional
	if size := cache.Size(ctx); size == 0 {
		t.Error("Expected cache to have some entries after concurrent access")
	}
}
