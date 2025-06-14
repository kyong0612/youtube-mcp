package youtube

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/youtube-transcript-mcp/internal/models"
)

// DefaultFetcher wraps the existing Service implementation as a TranscriptFetcher
type DefaultFetcher struct {
	service *Service
}

// NewDefaultFetcher creates a new DefaultFetcher
func NewDefaultFetcher(service *Service) *DefaultFetcher {
	return &DefaultFetcher{
		service: service,
	}
}

// FetchTranscript delegates to the service's GetTranscript method
func (d *DefaultFetcher) FetchTranscript(ctx context.Context, videoID string, languages []string) (*models.TranscriptResponse, error) {
	return d.service.GetTranscript(ctx, videoID, languages, false)
}

// ListAvailableLanguages delegates to the service's ListAvailableLanguages method
func (d *DefaultFetcher) ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error) {
	return d.service.ListAvailableLanguages(ctx, videoID)
}

// EnhancedService provides a service with fallback fetchers
type EnhancedService struct {
	*Service
	compositeFetcher *CompositeFetcher
}

// NewEnhancedService creates a new enhanced service with multiple fetchers
func NewEnhancedService(service *Service) *EnhancedService {
	// Create fetchers
	defaultFetcher := NewDefaultFetcher(service)
	kkdaiFetcher := NewKkdaiFetcher()
	
	// Create composite fetcher with fallback order
	compositeFetcher := NewCompositeFetcher(
		service.logger,
		defaultFetcher,  // Try our implementation first
		kkdaiFetcher,    // Fall back to kkdai library
	)
	
	return &EnhancedService{
		Service:          service,
		compositeFetcher: compositeFetcher,
	}
}

// GetTranscript overrides the original method to use composite fetcher
func (s *EnhancedService) GetTranscript(ctx context.Context, videoIdentifier string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error) {
	videoID, err := s.extractVideoID(videoIdentifier)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeInvalidVideoID,
			Message: fmt.Sprintf("Invalid video identifier: %s", err.Error()),
			VideoID: videoIdentifier,
		}
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s%s:%s", models.CacheKeyPrefixTranscript, videoID, strings.Join(languages, ","))
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		if transcript, ok := cached.(*models.TranscriptResponse); ok {
			s.logger.Debug("Returning cached transcript", "video_id", videoID)
			return transcript, nil
		}
	}

	// Use default languages if none provided
	if len(languages) == 0 {
		languages = s.config.DefaultLanguages
	}

	// Use composite fetcher
	response, err := s.compositeFetcher.FetchTranscript(ctx, videoID, languages)
	if err != nil {
		return nil, err
	}

	// Format transcript if needed
	if !preserveFormatting && response.FormattedText == "" {
		response.FormattedText = s.formatTranscriptText(response.Transcript)
	}

	// Ensure metadata is calculated
	if response.WordCount == 0 {
		response.WordCount = s.countWords(response.FormattedText)
	}
	if response.CharCount == 0 {
		response.CharCount = len(response.FormattedText)
	}
	if response.DurationSeconds == 0 {
		response.DurationSeconds = s.calculateDuration(response.Transcript)
	}

	// Cache the result
	if err := s.cache.Set(ctx, cacheKey, response, time.Hour*24); err != nil {
		s.logger.Warn("Failed to cache transcript response", "error", err)
	}

	return response, nil
}

// ListAvailableLanguages overrides to use composite fetcher
func (s *EnhancedService) ListAvailableLanguages(ctx context.Context, videoIdentifier string) (*models.AvailableLanguagesResponse, error) {
	videoID, err := s.extractVideoID(videoIdentifier)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeInvalidVideoID,
			Message: fmt.Sprintf("Invalid video identifier: %s", err.Error()),
			VideoID: videoIdentifier,
		}
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s%s", models.CacheKeyPrefixLanguages, videoID)
	if cached, found := s.cache.Get(ctx, cacheKey); found {
		if languages, ok := cached.(*models.AvailableLanguagesResponse); ok {
			return languages, nil
		}
	}

	// Use composite fetcher
	response, err := s.compositeFetcher.ListAvailableLanguages(ctx, videoID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if err := s.cache.Set(ctx, cacheKey, response, time.Hour*6); err != nil {
		s.logger.Warn("Failed to cache languages response", "error", err)
	}

	return response, nil
}