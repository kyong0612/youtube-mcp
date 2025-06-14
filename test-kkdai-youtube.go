package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kkdai/youtube/v2"
)

func main() {
	videoID := "YaE2GYegLNU"
	fmt.Printf("Testing kkdai/youtube library with video: %s\n", videoID)

	client := youtube.Client{}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get video info
	fmt.Println("\n1. Fetching video info...")
	video, err := client.GetVideoContext(ctx, videoID)
	if err != nil {
		log.Printf("Error getting video: %v", err)
		return
	}

	fmt.Printf("Title: %s\n", video.Title)
	fmt.Printf("Author: %s\n", video.Author)
	fmt.Printf("Duration: %s\n", video.Duration)

	// List available caption tracks
	fmt.Printf("\n2. Available caption tracks: %d\n", len(video.CaptionTracks))
	for i, track := range video.CaptionTracks {
		fmt.Printf("  [%d] Language: %s, Name: %s\n", i, track.LanguageCode, track.Name.SimpleText)
	}

	// Try to get transcript
	fmt.Println("\n3. Fetching transcript...")
	transcript, err := client.GetTranscript(video, "en")
	if err != nil {
		fmt.Printf("Error getting transcript: %v\n", err)
		
		// Try with first available language
		if len(video.CaptionTracks) > 0 {
			firstLang := video.CaptionTracks[0].LanguageCode
			fmt.Printf("\n4. Retrying with first available language: %s\n", firstLang)
			transcript, err = client.GetTranscript(video, firstLang)
			if err != nil {
				log.Printf("Error getting transcript with %s: %v", firstLang, err)
				return
			}
		} else {
			return
		}
	}

	// Display first few segments
	fmt.Printf("\n5. First 5 transcript segments:\n")
	for i, segment := range transcript {
		if i >= 5 {
			break
		}
		fmt.Printf("  [%.1fs - %.1fs] %s\n", 
			float64(segment.StartMs)/1000.0,
			float64(segment.StartMs+segment.Duration)/1000.0,
			segment.Text)
	}

	fmt.Printf("\nTotal segments: %d\n", len(transcript))
}