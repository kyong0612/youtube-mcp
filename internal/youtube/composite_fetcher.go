package youtube

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/youtube-transcript-mcp/internal/models"
)

// TranscriptFetcher defines the interface for transcript fetchers
type TranscriptFetcher interface {
	FetchTranscript(ctx context.Context, videoID string, languages []string) (*models.TranscriptResponse, error)
	ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error)
}

// CompositeFetcher tries multiple fetchers in order
type CompositeFetcher struct {
	fetchers []TranscriptFetcher
	logger   *slog.Logger
}

// NewCompositeFetcher creates a new composite fetcher
func NewCompositeFetcher(logger *slog.Logger, fetchers ...TranscriptFetcher) *CompositeFetcher {
	return &CompositeFetcher{
		fetchers: fetchers,
		logger:   logger,
	}
}

// FetchTranscript tries each fetcher until one succeeds
func (c *CompositeFetcher) FetchTranscript(ctx context.Context, videoID string, languages []string) (*models.TranscriptResponse, error) {
	var lastErr error
	
	for i, fetcher := range c.fetchers {
		c.logger.Debug("Trying fetcher", 
			"index", i,
			"fetcher_type", fmt.Sprintf("%T", fetcher),
			"video_id", videoID)
		
		response, err := fetcher.FetchTranscript(ctx, videoID, languages)
		if err == nil {
			c.logger.Info("Successfully fetched transcript",
				"fetcher_index", i,
				"fetcher_type", fmt.Sprintf("%T", fetcher),
				"video_id", videoID,
				"language", response.Language)
			return response, nil
		}
		
		c.logger.Debug("Fetcher failed",
			"index", i,
			"fetcher_type", fmt.Sprintf("%T", fetcher),
			"error", err)
		lastErr = err
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	
	return nil, &models.TranscriptError{
		Type:    models.ErrorTypeInternalError,
		Message: "All fetchers failed",
		VideoID: videoID,
	}
}

// ListAvailableLanguages tries each fetcher until one succeeds
func (c *CompositeFetcher) ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error) {
	var lastErr error
	
	for i, fetcher := range c.fetchers {
		response, err := fetcher.ListAvailableLanguages(ctx, videoID)
		if err == nil {
			c.logger.Debug("Successfully listed languages",
				"fetcher_index", i,
				"fetcher_type", fmt.Sprintf("%T", fetcher),
				"video_id", videoID,
				"language_count", len(response.Languages))
			return response, nil
		}
		
		c.logger.Debug("Fetcher failed to list languages",
			"index", i,
			"fetcher_type", fmt.Sprintf("%T", fetcher),
			"error", err)
		lastErr = err
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	
	return nil, &models.TranscriptError{
		Type:    models.ErrorTypeInternalError,
		Message: "All fetchers failed to list languages",
		VideoID: videoID,
	}
}