package youtube

import (
	"context"
	"fmt"
	"time"

	"github.com/kkdai/youtube/v2"

	"github.com/youtube-transcript-mcp/internal/models"
)

// KkdaiFetcher uses the kkdai/youtube library to fetch transcripts
type KkdaiFetcher struct {
	client *youtube.Client
}

// NewKkdaiFetcher creates a new KkdaiFetcher instance
func NewKkdaiFetcher() *KkdaiFetcher {
	return &KkdaiFetcher{
		client: &youtube.Client{},
	}
}

// FetchTranscript fetches transcript using kkdai/youtube library
func (k *KkdaiFetcher) FetchTranscript(ctx context.Context, videoID string, languages []string) (*models.TranscriptResponse, error) {
	// Get video info
	video, err := k.client.GetVideoContext(ctx, videoID)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: fmt.Sprintf("Failed to get video info: %s", err.Error()),
			VideoID: videoID,
		}
	}

	// Check if transcripts are available
	if len(video.CaptionTracks) == 0 {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNoTranscriptFound,
			Message: "No captions available for this video",
			VideoID: videoID,
		}
	}

	// Try to get transcript with preferred languages
	var transcript []youtube.TranscriptSegment
	var selectedLang string
	var transcriptErr error

	for _, lang := range languages {
		transcript, err = k.client.GetTranscript(video, lang)
		if err == nil {
			selectedLang = lang
			break
		}
		transcriptErr = err
	}

	// If no preferred language found, try first available
	if transcript == nil && len(video.CaptionTracks) > 0 {
		firstLang := video.CaptionTracks[0].LanguageCode
		transcript, err = k.client.GetTranscript(video, firstLang)
		if err == nil {
			selectedLang = firstLang
		} else {
			transcriptErr = err
		}
	}

	if transcript == nil {
		errMsg := "Failed to fetch transcript"
		if transcriptErr != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, transcriptErr)
		}
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: errMsg,
			VideoID: videoID,
		}
	}

	// Convert to our transcript format
	segments := make([]models.TranscriptSegment, 0, len(transcript))
	for _, seg := range transcript {
		segment := models.TranscriptSegment{
			Text:     seg.Text,
			Start:    float64(seg.StartMs) / 1000.0,
			Duration: float64(seg.Duration) / 1000.0,
			End:      float64(seg.StartMs+seg.Duration) / 1000.0,
		}
		segments = append(segments, segment)
	}

	// Build response
	response := &models.TranscriptResponse{
		VideoID:        videoID,
		Title:          video.Title,
		Description:    video.Description,
		Language:       selectedLang,
		TranscriptType: models.TranscriptTypeAuto, // kkdai library doesn't distinguish types
		Transcript:     segments,
		Metadata: models.TranscriptMetadata{
			ExtractionTimestamp: time.Now().UTC(),
			LanguageDetected:    selectedLang,
			Source:              "kkdai/youtube",
			ChannelName:         video.Author,
			PublishedAt:         video.PublishDate.Format(time.RFC3339),
		},
	}

	// Format transcript text
	var text string
	for i, segment := range segments {
		text += segment.Text
		if i < len(segments)-1 {
			text += " "
		}
	}
	response.FormattedText = text

	// Calculate metadata
	response.WordCount = k.countWords(text)
	response.CharCount = len(text)
	response.DurationSeconds = video.Duration.Seconds()

	return response, nil
}

// ListAvailableLanguages lists available languages using kkdai/youtube
func (k *KkdaiFetcher) ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error) {
	video, err := k.client.GetVideoContext(ctx, videoID)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: fmt.Sprintf("Failed to get video info: %s", err.Error()),
			VideoID: videoID,
		}
	}

	languages := make([]models.LanguageInfo, 0, len(video.CaptionTracks))
	defaultLang := ""

	for _, track := range video.CaptionTracks {
		lang := models.LanguageInfo{
			Code:       track.LanguageCode,
			Name:       track.Name.SimpleText,
			NativeName: track.Name.SimpleText,
			Type:       models.TranscriptTypeAuto,
		}
		languages = append(languages, lang)

		// First track is usually default
		if defaultLang == "" {
			defaultLang = track.LanguageCode
		}
	}

	return &models.AvailableLanguagesResponse{
		VideoID:         videoID,
		Languages:       languages,
		DefaultLanguage: defaultLang,
	}, nil
}

func (k *KkdaiFetcher) countWords(text string) int {
	if text == "" {
		return 0
	}
	wordCount := 1
	for _, char := range text {
		if char == ' ' || char == '\n' || char == '\t' {
			wordCount++
		}
	}
	return wordCount
}
