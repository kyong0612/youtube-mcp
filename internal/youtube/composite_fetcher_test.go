package youtube

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyong0612/youtube-mcp/internal/models"
)

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

// MockFetcher is a mock implementation of TranscriptFetcher for testing
type MockFetcher struct {
	name       string
	shouldFail bool
}

func (m *MockFetcher) FetchTranscript(ctx context.Context, videoID string, languages []string) (*models.TranscriptResponse, error) {
	if m.shouldFail {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: "mock error",
			VideoID: videoID,
		}
	}

	return &models.TranscriptResponse{
		VideoID:  videoID,
		Title:    "Mock Video",
		Language: "en",
		Metadata: models.TranscriptMetadata{
			Source: m.name,
		},
	}, nil
}

func (m *MockFetcher) ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error) {
	if m.shouldFail {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: "mock error",
			VideoID: videoID,
		}
	}

	return &models.AvailableLanguagesResponse{
		VideoID: videoID,
		Languages: []models.LanguageInfo{
			{Code: "en", Name: "English"},
		},
	}, nil
}

func TestCompositeFetcher_FallbackBehavior(t *testing.T) {
	logger := setupTestLogger()

	// Create fetchers - first one fails, second succeeds
	fetcher1 := &MockFetcher{shouldFail: true, name: "fetcher1"}
	fetcher2 := &MockFetcher{shouldFail: false, name: "fetcher2"}

	composite := NewCompositeFetcher(logger, fetcher1, fetcher2)

	// Test transcript fetching
	ctx := context.Background()
	response, err := composite.FetchTranscript(ctx, "test123", []string{"en"})

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "fetcher2", response.Metadata.Source)

	// Test language listing
	languages, err := composite.ListAvailableLanguages(ctx, "test123")

	assert.NoError(t, err)
	assert.NotNil(t, languages)
	assert.Len(t, languages.Languages, 1)
}

func TestNewEnhancedService_FetcherOrder(t *testing.T) {
	// The enhanced service must try the maintained kkdai library before
	// falling back to the custom HTML/regex scraper. Guard the order so a
	// future refactor does not silently regress reliability.
	svc := &Service{logger: setupTestLogger()}
	enhanced := NewEnhancedService(svc)

	fetchers := enhanced.compositeFetcher.fetchers
	if assert.Len(t, fetchers, 2) {
		assert.IsType(t, &KkdaiFetcher{}, fetchers[0], "first fetcher should be KkdaiFetcher")
		assert.IsType(t, &DefaultFetcher{}, fetchers[1], "second fetcher should be DefaultFetcher")
	}
}

func TestCompositeFetcher_AllFail(t *testing.T) {
	logger := setupTestLogger()

	// Create fetchers - both fail
	fetcher1 := &MockFetcher{shouldFail: true, name: "fetcher1"}
	fetcher2 := &MockFetcher{shouldFail: true, name: "fetcher2"}

	composite := NewCompositeFetcher(logger, fetcher1, fetcher2)

	// Test transcript fetching
	ctx := context.Background()
	response, err := composite.FetchTranscript(ctx, "test123", []string{"en"})

	assert.Error(t, err)
	assert.Nil(t, response)
}
