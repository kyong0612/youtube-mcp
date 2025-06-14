package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/youtube-transcript-mcp/internal/cache"
	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/youtube"
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create config
	cfg := config.DefaultConfig()

	// Create memory cache
	cacheService := cache.NewMemoryCache(100, 100, time.Hour)

	// Create enhanced service
	baseService := youtube.NewService(cfg.YouTube, cacheService, logger)
	service := youtube.NewEnhancedService(baseService)

	ctx := context.Background()

	// Test videos
	testVideos := []struct {
		id   string
		name string
	}{
		{"YaE2GYegLNU", "Productive Peter video"},
		{"dQw4w9WgXcQ", "Rick Astley - Never Gonna Give You Up"},
		{"jNQXAC9IVRw", "Me at the zoo"},
	}

	for _, video := range testVideos {
		fmt.Printf("\n=== Testing video: %s (%s) ===\n", video.id, video.name)
		
		// Test transcript fetching
		start := time.Now()
		transcript, err := service.GetTranscript(ctx, video.id, []string{"en"}, false)
		duration := time.Since(start)
		
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		} else {
			fmt.Printf("✅ Success!\n")
			fmt.Printf("  Title: %s\n", transcript.Title)
			fmt.Printf("  Language: %s\n", transcript.Language)
			fmt.Printf("  Segments: %d\n", len(transcript.Transcript))
			fmt.Printf("  Source: %s\n", transcript.Metadata.Source)
			fmt.Printf("  Duration: %v\n", duration)
			
			// Show first segment
			if len(transcript.Transcript) > 0 {
				fmt.Printf("  First segment: \"%.50s...\"\n", transcript.Transcript[0].Text)
			}
		}
		
		// Test language listing
		languages, err := service.ListAvailableLanguages(ctx, video.id)
		if err != nil {
			fmt.Printf("  Languages: Error - %v\n", err)
		} else {
			fmt.Printf("  Available languages: %d\n", len(languages.Languages))
			for i, lang := range languages.Languages {
				if i < 3 {
					fmt.Printf("    - %s (%s)\n", lang.Code, lang.Name)
				}
			}
		}
		
		// Small delay between tests
		time.Sleep(time.Second)
	}
}