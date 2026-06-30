package youtube

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kyong0612/youtube-mcp/internal/models"
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
		defaultFetcher, // Try our implementation first
		kkdaiFetcher,   // Fall back to kkdai library
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

	// Negative cache: terminal failures are cached under a distinct key namespace
	// (CacheKeyPrefixError) so they never collide with successful transcripts. A
	// cached terminal error short-circuits the (slow) fetch and fails fast.
	errorCacheKey := fmt.Sprintf("%s%s:%s", models.CacheKeyPrefixError, videoID, strings.Join(languages, ","))
	if cached, found := s.cache.Get(ctx, errorCacheKey); found {
		if cachedErr, ok := cached.(*models.TranscriptError); ok {
			s.logger.Debug("Returning cached terminal error", "video_id", videoID, "error_type", cachedErr.Type)
			return nil, cachedErr
		}
	}

	// Use default languages if none provided
	if len(languages) == 0 {
		languages = s.config.DefaultLanguages
	}

	// Use composite fetcher
	response, err := s.compositeFetcher.FetchTranscript(ctx, videoID, languages)
	if err != nil {
		// Only terminal (permanent) errors are negative-cached; transient errors
		// (network, rate limit, internal) are left uncached so they are retried.
		if isTerminalError(err) {
			var termErr *models.TranscriptError
			_ = errors.As(err, &termErr)
			if setErr := s.cache.Set(ctx, errorCacheKey, termErr, s.errorTTL); setErr != nil {
				s.logger.Warn("Failed to cache terminal transcript error", "error", setErr)
			}
		}
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
	if err := s.cache.Set(ctx, cacheKey, response, s.transcriptTTL); err != nil {
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
	if err := s.cache.Set(ctx, cacheKey, response, s.languagesTTL); err != nil {
		s.logger.Warn("Failed to cache languages response", "error", err)
	}

	return response, nil
}

// isTerminalError reports whether err is a permanent (terminal) transcript
// failure that is safe to negative-cache. Terminal errors describe conditions
// that will not change on retry within the cache window (the video has no
// captions, is unavailable, has an invalid ID, or lacks the requested
// language). Transient errors (network failures, rate limiting, internal
// errors) are NOT terminal and must be retried rather than cached.
func isTerminalError(err error) bool {
	var transcriptErr *models.TranscriptError
	if !errors.As(err, &transcriptErr) {
		return false
	}

	switch transcriptErr.Type {
	case models.ErrorTypeNoTranscriptFound,
		models.ErrorTypeVideoUnavailable,
		models.ErrorTypeInvalidVideoID,
		models.ErrorTypeLanguageNotAvailable:
		return true
	default:
		return false
	}
}
