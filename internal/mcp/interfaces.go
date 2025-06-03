package mcp

import (
	"context"

	"github.com/youtube-transcript-mcp/internal/models"
)

// YouTubeService defines the interface for YouTube transcript operations
type YouTubeService interface {
	GetTranscript(ctx context.Context, videoID string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error)
	GetMultipleTranscripts(ctx context.Context, videoIDs []string, languages []string, continueOnError bool) (*models.MultipleTranscriptResponse, error)
	ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error)
	TranslateTranscript(ctx context.Context, videoID, targetLang, sourceLang string) (*models.TranscriptResponse, error)
	FormatTranscript(ctx context.Context, videoID, formatType string, includeTimestamps bool) (*models.TranscriptResponse, error)
}