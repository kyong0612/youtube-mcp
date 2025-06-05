package youtube

import (
	"context"

	"github.com/youtube-transcript-mcp/internal/models"
)

// ServiceInterface defines the interface for YouTube transcript operations
type ServiceInterface interface {
	GetTranscript(ctx context.Context, videoIdentifier string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error)
	GetMultipleTranscripts(ctx context.Context, videoIdentifiers []string, languages []string, continueOnError bool) (*models.MultipleTranscriptResponse, error)
	ListAvailableLanguages(ctx context.Context, videoIdentifier string) (*models.AvailableLanguagesResponse, error)
	TranslateTranscript(ctx context.Context, videoIdentifier, targetLanguage, sourceLanguage string) (*models.TranscriptResponse, error)
	FormatTranscript(ctx context.Context, videoIdentifier, formatType string, includeTimestamps bool) (*models.TranscriptResponse, error)
}
