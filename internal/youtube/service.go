package youtube

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/youtube-transcript-mcp/internal/cache"
	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/models"
	"golang.org/x/time/rate"
)

// Service handles YouTube transcript operations
type Service struct {
	config         config.YouTubeConfig
	httpClient     *http.Client
	cache          cache.Cache
	rateLimiter    *rate.Limiter
	hourlyLimiter  *rate.Limiter
	proxyManager   *ProxyManager
	logger         *slog.Logger
	rateLimitState *RateLimitState
}

// RateLimitState tracks rate limiting state for adaptive behavior
type RateLimitState struct {
	mu                  sync.RWMutex
	consecutiveFailures int
	lastFailureTime     time.Time
	backoffUntil        time.Time
	adaptiveMultiplier  float64
}

// ProxyManager manages proxy rotation
type ProxyManager struct {
	proxies      []string
	currentIndex int
	mu           sync.Mutex
}

// NewService creates a new YouTube service instance
func NewService(cfg config.YouTubeConfig, cache cache.Cache, logger *slog.Logger) *Service {
	httpClient := &http.Client{
		Timeout: cfg.RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Configure proxy if enabled
	var proxyManager *ProxyManager
	if cfg.EnableProxyRotation && len(cfg.ProxyList) > 0 {
		proxyManager = &ProxyManager{
			proxies: cfg.ProxyList,
		}
		httpClient.Transport = &http.Transport{
			Proxy: proxyManager.GetProxy,
		}
	} else if cfg.ProxyURL != "" {
		proxyURL, _ := url.Parse(cfg.ProxyURL)
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	// Create rate limiters with safe defaults
	rateLimitPerMinute := cfg.RateLimitPerMinute
	if rateLimitPerMinute <= 0 {
		rateLimitPerMinute = 60 // Default to 60 requests per minute
	}

	rateLimitPerHour := cfg.RateLimitPerHour
	if rateLimitPerHour <= 0 {
		rateLimitPerHour = 1000 // Default to 1000 requests per hour
	}

	rateLimiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(rateLimitPerMinute)), rateLimitPerMinute)
	hourlyLimiter := rate.NewLimiter(rate.Every(time.Hour/time.Duration(rateLimitPerHour)), rateLimitPerHour)

	return &Service{
		config:        cfg,
		httpClient:    httpClient,
		cache:         cache,
		rateLimiter:   rateLimiter,
		hourlyLimiter: hourlyLimiter,
		proxyManager:  proxyManager,
		logger:        logger,
		rateLimitState: &RateLimitState{
			adaptiveMultiplier: 1.0,
		},
	}
}

// GetTranscript retrieves transcript for a single video
func (s *Service) GetTranscript(ctx context.Context, videoIdentifier string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error) {
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
			s.logger.Debug("Returning cached transcript", slog.String("video_id", videoID))
			return transcript, nil
		}
	}

	// Use default languages if none provided
	if len(languages) == 0 {
		languages = s.config.DefaultLanguages
	}

	// Wait for rate limiters with adaptive backoff
	if err := s.waitForRateLimit(ctx); err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeRateLimitExceeded,
			Message: fmt.Sprintf("Rate limit exceeded: %s", err.Error()),
			VideoID: videoID,
		}
	}

	// Fetch video page to get initial data
	videoData, err := s.fetchVideoData(ctx, videoID)
	if err != nil {
		s.recordRateLimitFailure(err)
		return nil, err
	}

	// Extract available caption tracks
	captionTracks, err := s.extractCaptionTracks(videoData)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNoTranscriptFound,
			Message: fmt.Sprintf("No captions found: %s", err.Error()),
			VideoID: videoID,
		}
	}

	// Find best matching caption track
	selectedTrack := s.selectBestTrack(captionTracks, languages)
	if selectedTrack == nil {
		return nil, &models.TranscriptError{
			Type:        models.ErrorTypeLanguageNotAvailable,
			Message:     fmt.Sprintf("No captions available for requested languages: %v", languages),
			VideoID:     videoID,
			Suggestions: s.getAvailableLanguageCodes(captionTracks),
		}
	}

	// Fetch the transcript
	transcript, err := s.fetchTranscriptFromTrack(ctx, selectedTrack)
	if err != nil {
		s.recordRateLimitFailure(err)
		return nil, err
	}

	// Build response
	response := &models.TranscriptResponse{
		VideoID:        videoID,
		Title:          videoData.Title,
		Description:    videoData.Description,
		Language:       selectedTrack.LanguageCode,
		TranscriptType: s.getTranscriptType(selectedTrack),
		Transcript:     transcript,
		Metadata: models.TranscriptMetadata{
			ExtractionTimestamp: time.Now().UTC(),
			LanguageDetected:    selectedTrack.LanguageCode,
			Source:              "youtube",
			ChannelID:           videoData.ChannelID,
			ChannelName:         videoData.ChannelName,
			PublishedAt:         videoData.PublishedAt,
			ViewCount:           videoData.ViewCount,
			LikeCount:           videoData.LikeCount,
			CommentCount:        videoData.CommentCount,
		},
	}

	// Format transcript if needed
	if !preserveFormatting {
		response.FormattedText = s.formatTranscriptText(transcript)
	}

	// Calculate metadata
	response.WordCount = s.countWords(response.FormattedText)
	response.CharCount = len(response.FormattedText)
	response.DurationSeconds = s.calculateDuration(transcript)

	// Cache the result
	if err := s.cache.Set(ctx, cacheKey, response, s.config.RequestTimeout); err != nil {
		s.logger.Warn("Failed to cache transcript response", "error", err)
	}

	// Record success for adaptive rate limiting
	s.recordRateLimitSuccess()

	return response, nil
}

// GetMultipleTranscripts retrieves transcripts for multiple videos
func (s *Service) GetMultipleTranscripts(ctx context.Context, videoIdentifiers []string, languages []string, continueOnError bool) (*models.MultipleTranscriptResponse, error) {
	response := &models.MultipleTranscriptResponse{
		Results:    make([]models.TranscriptResult, 0, len(videoIdentifiers)),
		Errors:     make([]models.TranscriptError, 0),
		TotalCount: len(videoIdentifiers),
	}

	// Use semaphore for concurrent processing
	sem := make(chan struct{}, s.config.MaxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, videoIdentifier := range videoIdentifiers {
		wg.Add(1)
		go func(vid string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			start := time.Now()
			transcript, err := s.GetTranscript(ctx, vid, languages, false)
			processingTime := time.Since(start)

			result := models.TranscriptResult{
				VideoID:        vid,
				ProcessingTime: processingTime,
			}

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				if transcriptErr, ok := err.(*models.TranscriptError); ok {
					result.Success = false
					result.Error = transcriptErr
					response.Errors = append(response.Errors, *transcriptErr)
					response.ErrorCount++
				} else {
					transcriptErr := &models.TranscriptError{
						Type:    models.ErrorTypeInternalError,
						Message: err.Error(),
						VideoID: vid,
					}
					result.Success = false
					result.Error = transcriptErr
					response.Errors = append(response.Errors, *transcriptErr)
					response.ErrorCount++
				}
			} else {
				result.Success = true
				result.Transcript = transcript
				response.SuccessCount++
			}

			response.Results = append(response.Results, result)
		}(videoIdentifier)
	}

	wg.Wait()
	return response, nil
}

// ListAvailableLanguages lists available transcript languages for a video
func (s *Service) ListAvailableLanguages(ctx context.Context, videoIdentifier string) (*models.AvailableLanguagesResponse, error) {
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

	// Fetch video data
	videoData, err := s.fetchVideoData(ctx, videoID)
	if err != nil {
		return nil, err
	}

	// Extract caption tracks
	captionTracks, err := s.extractCaptionTracks(videoData)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNoTranscriptFound,
			Message: "No captions available for this video",
			VideoID: videoID,
		}
	}

	// Build language list
	languages := make([]models.LanguageInfo, 0, len(captionTracks))
	defaultLang := ""
	translatableCount := 0

	for i := range captionTracks {
		track := &captionTracks[i]
		lang := models.LanguageInfo{
			Code:         track.LanguageCode,
			Name:         track.Name.SimpleText,
			NativeName:   track.Name.SimpleText,
			Type:         s.getTranscriptType(track),
			IsTranslated: track.IsTranslatable,
			IsDefault:    track.IsDefault,
		}

		languages = append(languages, lang)

		if track.IsDefault {
			defaultLang = track.LanguageCode
		}
		if track.IsTranslatable {
			translatableCount++
		}
	}

	response := &models.AvailableLanguagesResponse{
		VideoID:           videoID,
		Languages:         languages,
		DefaultLanguage:   defaultLang,
		TranslatableCount: translatableCount,
	}

	// Cache the result
	if err := s.cache.Set(ctx, cacheKey, response, s.config.RequestTimeout); err != nil {
		s.logger.Warn("Failed to cache transcript response", "error", err)
	}

	return response, nil
}

// TranslateTranscript translates a transcript to target language
func (s *Service) TranslateTranscript(ctx context.Context, videoIdentifier, targetLanguage, sourceLanguage string) (*models.TranscriptResponse, error) {
	// First get available languages
	availableLanguages, err := s.ListAvailableLanguages(ctx, videoIdentifier)
	if err != nil {
		return nil, err
	}

	// Check if target language is available
	var targetFound bool
	for _, lang := range availableLanguages.Languages {
		if lang.Code == targetLanguage {
			targetFound = true
			break
		}
	}

	if !targetFound {
		// Try to get auto-translated version
		return s.GetTranscript(ctx, videoIdentifier, []string{targetLanguage}, false)
	}

	// Get transcript in target language
	return s.GetTranscript(ctx, videoIdentifier, []string{targetLanguage}, false)
}

// FormatTranscript formats a transcript according to specified format
func (s *Service) FormatTranscript(ctx context.Context, videoIdentifier, formatType string, includeTimestamps bool) (*models.TranscriptResponse, error) {
	transcript, err := s.GetTranscript(ctx, videoIdentifier, nil, true)
	if err != nil {
		return nil, err
	}

	switch formatType {
	case models.FormatTypePlainText:
		transcript.FormattedText = s.formatAsPlainText(transcript.Transcript, includeTimestamps)
	case models.FormatTypeParagraphs:
		transcript.FormattedText = s.formatAsParagraphs(transcript.Transcript, includeTimestamps)
	case models.FormatTypeSentences:
		transcript.FormattedText = s.formatAsSentences(transcript.Transcript, includeTimestamps)
	case models.FormatTypeSRT:
		transcript.FormattedText = s.formatAsSRT(transcript.Transcript)
	case models.FormatTypeVTT:
		transcript.FormattedText = s.formatAsVTT(transcript.Transcript)
	case models.FormatTypeJSON:
		jsonBytes, err := json.MarshalIndent(transcript.Transcript, "", "  ")
		if err != nil {
			return nil, err
		}
		transcript.FormattedText = string(jsonBytes)
	default:
		transcript.FormattedText = s.formatTranscriptText(transcript.Transcript)
	}

	transcript.WordCount = s.countWords(transcript.FormattedText)
	transcript.CharCount = len(transcript.FormattedText)

	return transcript, nil
}

// fetchVideoData fetches initial video data from YouTube
func (s *Service) fetchVideoData(ctx context.Context, videoID string) (*VideoData, error) {
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.config.UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.doHTTPRequestWithRetry(ctx, req)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: fmt.Sprintf("Failed to fetch video page: %s", err.Error()),
			VideoID: videoID,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeVideoUnavailable,
			Message: "Video not found or unavailable",
			VideoID: videoID,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: fmt.Sprintf("HTTP error: %d", resp.StatusCode),
			VideoID: videoID,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract video data from the page
	return s.parseVideoData(string(body), videoID)
}

// parseVideoData extracts video metadata and caption information from the HTML
func (s *Service) parseVideoData(html string, videoID string) (*VideoData, error) {
	videoData := &VideoData{
		VideoID: videoID,
	}

	// Extract initial player response
	playerResponseRegex := regexp.MustCompile(`var ytInitialPlayerResponse = ({.+?});`)
	matches := playerResponseRegex.FindStringSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("failed to extract player response")
	}

	var playerResponse PlayerResponse
	if err := json.Unmarshal([]byte(matches[1]), &playerResponse); err != nil {
		return nil, fmt.Errorf("failed to parse player response: %w", err)
	}

	// Extract video details
	if details := playerResponse.VideoDetails; details != nil {
		videoData.Title = details.Title
		videoData.Description = details.ShortDescription
		videoData.ChannelID = details.ChannelID
		videoData.ChannelName = details.Author
		videoData.ViewCount, _ = strconv.ParseInt(details.ViewCount, 10, 64)
		videoData.IsLive = details.IsLiveContent
	}

	// Extract caption tracks
	if captions := playerResponse.Captions; captions != nil {
		videoData.CaptionTracks = captions.PlayerCaptionsTracklistRenderer.CaptionTracks
	}

	// Extract additional metadata from initial data if needed in future
	// Currently not extracting additional metadata but keeping regex for future use
	_ = regexp.MustCompile(`var ytInitialData = ({.+?});`)

	return videoData, nil
}

// extractVideoID extracts video ID from various YouTube URL formats
func (s *Service) extractVideoID(identifier string) (string, error) {
	// If it's already a video ID (11 characters, alphanumeric + underscore + dash)
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{11}$`, identifier); matched {
		return identifier, nil
	}

	// Extract from URL
	patterns := []string{
		`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/)([a-zA-Z0-9_-]{11})`,
		`youtube\.com/shorts/([a-zA-Z0-9_-]{11})`,
		`youtube\.com/live/([a-zA-Z0-9_-]{11})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(identifier)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not extract video ID from: %s", identifier)
}

// extractCaptionTracks extracts caption tracks from video data
func (s *Service) extractCaptionTracks(videoData *VideoData) ([]CaptionTrack, error) {
	if len(videoData.CaptionTracks) == 0 {
		return nil, fmt.Errorf("no caption tracks available")
	}

	return videoData.CaptionTracks, nil
}

// selectBestTrack selects the best matching caption track based on language preferences
func (s *Service) selectBestTrack(tracks []CaptionTrack, languages []string) *CaptionTrack {
	// First try to find exact match
	for _, lang := range languages {
		for i, track := range tracks {
			if track.LanguageCode == lang {
				return &tracks[i]
			}
		}
	}

	// Then try to find by language prefix (e.g., "en" matches "en-US")
	for _, lang := range languages {
		for i, track := range tracks {
			if strings.HasPrefix(track.LanguageCode, lang+"-") {
				return &tracks[i]
			}
		}
	}

	// If no match, return the default track
	for i, track := range tracks {
		if track.IsDefault {
			return &tracks[i]
		}
	}

	// Return first available track
	if len(tracks) > 0 {
		return &tracks[0]
	}

	return nil
}

// fetchTranscriptFromTrack fetches the actual transcript data from a caption track
func (s *Service) fetchTranscriptFromTrack(ctx context.Context, track *CaptionTrack) ([]models.TranscriptSegment, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", track.BaseURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.config.UserAgent)

	resp, err := s.doHTTPRequestWithRetry(ctx, req)
	if err != nil {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: fmt.Sprintf("Failed to fetch transcript: %s", err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &models.TranscriptError{
			Type:    models.ErrorTypeNetworkError,
			Message: fmt.Sprintf("HTTP error fetching transcript: %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Debug: Log the response size and first part
	s.logger.Debug("Fetched transcript data",
		"size", len(body),
		"url", track.BaseURL,
		"preview", string(body[:min(200, len(body))]))

	// Check if body is empty
	if len(body) == 0 {
		return nil, fmt.Errorf("empty transcript response")
	}

	// Parse the transcript XML
	return s.parseTranscriptXML(body)
}

// Text represents a text element in XML transcripts
type Text struct {
	Start float64 `xml:"start,attr"`
	Dur   float64 `xml:"dur,attr"`
	Text  string  `xml:",chardata"`
}

// parseTranscriptXML parses YouTube's transcript XML format with improved error handling
func (s *Service) parseTranscriptXML(data []byte) ([]models.TranscriptSegment, error) {
	// First, check if the data is empty or has minimal content
	if len(data) == 0 {
		return nil, fmt.Errorf("empty XML response")
	}

	// Clean the data - remove any BOM or leading whitespace
	cleanData := strings.TrimSpace(string(data))
	if cleanData == "" {
		return nil, fmt.Errorf("XML response contains only whitespace")
	}

	// Log the XML structure for debugging
	s.logger.Debug("Parsing XML transcript",
		"size", len(data),
		"starts_with", cleanData[:min(50, len(cleanData))],
		"contains_transcript", strings.Contains(cleanData, "<transcript"),
		"contains_timedtext", strings.Contains(cleanData, "<timedtext"))

	type Transcript struct {
		XMLName xml.Name `xml:"transcript"`
		Texts   []Text   `xml:"text"`
	}

	type TimedText struct {
		XMLName xml.Name `xml:"timedtext"`
		Head    struct {
			Ws struct {
				WinStyles []interface{} `xml:"ws"`
			} `xml:"ws"`
			Wp struct {
				PenStyles []interface{} `xml:"wp"`
			} `xml:"wp"`
		} `xml:"head"`
		Body struct {
			Paragraphs []struct {
				Start     float64 `xml:"t,attr"`
				Dur       float64 `xml:"d,attr"`
				Sentences []Text  `xml:"s"`
				Text      string  `xml:",chardata"`
			} `xml:"p"`
			Texts []Text `xml:"text"`
		} `xml:"body"`
	}

	var segments []models.TranscriptSegment

	// Try parsing as standard <transcript> format first
	var transcript Transcript
	if err := xml.Unmarshal([]byte(cleanData), &transcript); err == nil && len(transcript.Texts) > 0 {
		s.logger.Debug("Successfully parsed as transcript format", "count", len(transcript.Texts))
		segments = s.convertTextsToSegments(transcript.Texts)
	} else {
		// Try parsing as <timedtext> format
		var timedtext TimedText
		if err := xml.Unmarshal([]byte(cleanData), &timedtext); err == nil {
			s.logger.Debug("Successfully parsed as timedtext format", "paragraphs", len(timedtext.Body.Paragraphs), "texts", len(timedtext.Body.Texts))

			// Check if we have text elements directly in body
			if len(timedtext.Body.Texts) > 0 {
				segments = s.convertTextsToSegments(timedtext.Body.Texts)
			} else {
				// Extract from paragraphs
				for _, p := range timedtext.Body.Paragraphs {
					if len(p.Sentences) > 0 {
						// Use sentences if available
						segments = append(segments, s.convertTextsToSegments(p.Sentences)...)
					} else if strings.TrimSpace(p.Text) != "" {
						// Use paragraph text directly
						cleanedText := s.cleanTranscriptText(p.Text)
						if cleanedText != "" {
							duration := p.Dur
							if duration <= 0 {
								duration = 2.0 // Default duration
							}
							segment := models.TranscriptSegment{
								Text:     cleanedText,
								Start:    p.Start,
								Duration: duration,
								End:      p.Start + duration,
							}
							segments = append(segments, segment)
						}
					}
				}
			}
		} else {
			// Log the XML content for debugging
			s.logger.Error("Failed to parse XML in any known format",
				"xml_preview", cleanData[:min(500, len(cleanData))],
				"parse_error", err)
			return nil, fmt.Errorf("failed to parse transcript XML: unknown format or corrupted data")
		}
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("no transcript segments found in XML")
	}

	s.logger.Debug("Successfully parsed transcript", "segments", len(segments))
	return segments, nil
}

// convertTextsToSegments converts Text structs to TranscriptSegment structs
func (s *Service) convertTextsToSegments(texts []Text) []models.TranscriptSegment {
	segments := make([]models.TranscriptSegment, 0, len(texts))
	for _, text := range texts {
		// Clean and decode text
		cleanedText := s.cleanTranscriptText(text.Text)
		if cleanedText == "" {
			continue
		}

		duration := text.Dur
		if duration <= 0 {
			duration = 2.0 // Default duration for segments with missing duration
		}

		segment := models.TranscriptSegment{
			Text:     cleanedText,
			Start:    text.Start,
			Duration: duration,
			End:      text.Start + duration,
		}
		segments = append(segments, segment)
	}
	return segments
}

// cleanTranscriptText cleans and decodes transcript text
func (s *Service) cleanTranscriptText(text string) string {
	// Decode HTML entities
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Remove line breaks within text
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")

	// Normalize whitespace
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return text
}

// getTranscriptType determines the type of transcript
func (s *Service) getTranscriptType(track *CaptionTrack) string {
	if track.Kind == "asr" {
		return models.TranscriptTypeAuto
	}
	if track.VssID != "" && strings.Contains(track.VssID, ".") {
		return models.TranscriptTypeGenerated
	}
	return models.TranscriptTypeManual
}

// getAvailableLanguageCodes extracts language codes from caption tracks
func (s *Service) getAvailableLanguageCodes(tracks []CaptionTrack) []string {
	codes := make([]string, 0, len(tracks))
	for _, track := range tracks {
		codes = append(codes, track.LanguageCode)
	}
	return codes
}

// Format functions

func (s *Service) formatTranscriptText(segments []models.TranscriptSegment) string {
	var builder strings.Builder

	for i, segment := range segments {
		builder.WriteString(segment.Text)
		if i < len(segments)-1 {
			builder.WriteString(" ")
		}
	}

	return builder.String()
}

func (s *Service) formatAsPlainText(segments []models.TranscriptSegment, includeTimestamps bool) string {
	var builder strings.Builder

	for _, segment := range segments {
		if includeTimestamps {
			builder.WriteString(fmt.Sprintf("[%.1fs] ", segment.Start))
		}
		builder.WriteString(segment.Text)
		builder.WriteString(" ")
	}

	return strings.TrimSpace(builder.String())
}

func (s *Service) formatAsParagraphs(segments []models.TranscriptSegment, includeTimestamps bool) string {
	var builder strings.Builder
	var currentParagraph strings.Builder

	for i, segment := range segments {
		if includeTimestamps && currentParagraph.Len() == 0 {
			currentParagraph.WriteString(fmt.Sprintf("[%.1fs] ", segment.Start))
		}
		currentParagraph.WriteString(segment.Text)
		currentParagraph.WriteString(" ")

		// Start new paragraph every 5 segments or at natural breaks
		if (i+1)%5 == 0 || strings.HasSuffix(strings.TrimSpace(segment.Text), ".") {
			builder.WriteString(strings.TrimSpace(currentParagraph.String()))
			builder.WriteString("\n\n")
			currentParagraph.Reset()
		}
	}

	// Add remaining text
	if currentParagraph.Len() > 0 {
		builder.WriteString(strings.TrimSpace(currentParagraph.String()))
	}

	return strings.TrimSpace(builder.String())
}

func (s *Service) formatAsSentences(segments []models.TranscriptSegment, includeTimestamps bool) string {
	var builder strings.Builder

	for _, segment := range segments {
		if includeTimestamps {
			builder.WriteString(fmt.Sprintf("[%.1fs] ", segment.Start))
		}
		builder.WriteString(segment.Text)

		// Add period if not present
		if !strings.HasSuffix(strings.TrimSpace(segment.Text), ".") &&
			!strings.HasSuffix(strings.TrimSpace(segment.Text), "!") &&
			!strings.HasSuffix(strings.TrimSpace(segment.Text), "?") {
			builder.WriteString(".")
		}
		builder.WriteString("\n")
	}

	return strings.TrimSpace(builder.String())
}

func (s *Service) formatAsSRT(segments []models.TranscriptSegment) string {
	var builder strings.Builder

	for i, segment := range segments {
		// Sequence number
		builder.WriteString(fmt.Sprintf("%d\n", i+1))

		// Timestamp
		startTime := s.formatSRTTime(segment.Start)
		endTime := s.formatSRTTime(segment.End)
		builder.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))

		// Text
		builder.WriteString(segment.Text)
		builder.WriteString("\n\n")
	}

	return strings.TrimSpace(builder.String())
}

func (s *Service) formatAsVTT(segments []models.TranscriptSegment) string {
	var builder strings.Builder

	// VTT header
	builder.WriteString("WEBVTT\n\n")

	for _, segment := range segments {
		// Timestamp
		startTime := s.formatVTTTime(segment.Start)
		endTime := s.formatVTTTime(segment.End)
		builder.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))

		// Text
		builder.WriteString(segment.Text)
		builder.WriteString("\n\n")
	}

	return strings.TrimSpace(builder.String())
}

func (s *Service) formatSRTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds-float64(int(seconds)))*1000 + 0.5) // Round to nearest millisecond

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}

func (s *Service) formatVTTTime(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int((seconds-float64(int(seconds)))*1000 + 0.5) // Round to nearest millisecond

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}

func (s *Service) countWords(text string) int {
	words := strings.Fields(text)
	return len(words)
}

func (s *Service) calculateDuration(segments []models.TranscriptSegment) float64 {
	if len(segments) == 0 {
		return 0
	}

	lastSegment := segments[len(segments)-1]
	return lastSegment.End
}

// waitForRateLimit waits for both minute and hourly rate limiters with adaptive behavior
func (s *Service) waitForRateLimit(ctx context.Context) error {
	s.rateLimitState.mu.RLock()
	backoffUntil := s.rateLimitState.backoffUntil
	adaptiveMultiplier := s.rateLimitState.adaptiveMultiplier
	s.rateLimitState.mu.RUnlock()

	// Check if we're in adaptive backoff period
	if time.Now().Before(backoffUntil) {
		waitTime := time.Until(backoffUntil)
		s.logger.Debug("Waiting for adaptive backoff period",
			"wait_time", waitTime,
			"multiplier", adaptiveMultiplier)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue
		}
	}

	// Wait for minute-based rate limiter with adaptive delay
	reservation := s.rateLimiter.ReserveN(time.Now(), 1)
	if !reservation.OK() {
		return fmt.Errorf("rate limiter reservation failed")
	}

	waitTime := reservation.DelayFrom(time.Now())
	if waitTime > 0 {
		s.logger.Debug("Waiting for minute rate limiter",
			"wait_time", waitTime,
			"adaptive_multiplier", adaptiveMultiplier)

		select {
		case <-ctx.Done():
			reservation.Cancel()
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue
		}
	}

	// Wait for hourly rate limiter
	if err := s.hourlyLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("hourly rate limit exceeded: %w", err)
	}

	return nil
}

// recordRateLimitSuccess records a successful operation for adaptive rate limiting
func (s *Service) recordRateLimitSuccess() {
	s.rateLimitState.mu.Lock()
	defer s.rateLimitState.mu.Unlock()

	// Gradually reduce adaptive multiplier back to 1.0
	if s.rateLimitState.adaptiveMultiplier > 1.0 {
		s.rateLimitState.adaptiveMultiplier = math.Max(1.0, s.rateLimitState.adaptiveMultiplier*0.9)
		s.logger.Debug("Reducing adaptive rate limit multiplier",
			"multiplier", s.rateLimitState.adaptiveMultiplier)
	}

	// Reset consecutive failures
	if s.rateLimitState.consecutiveFailures > 0 {
		s.rateLimitState.consecutiveFailures = 0
		s.logger.Debug("Reset consecutive failures counter")
	}
}

// recordRateLimitFailure records a failed operation for adaptive rate limiting
func (s *Service) recordRateLimitFailure(err error) {
	s.rateLimitState.mu.Lock()
	defer s.rateLimitState.mu.Unlock()

	// Check if this is a rate limit related failure
	if strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
		strings.Contains(strings.ToLower(err.Error()), "429") ||
		strings.Contains(strings.ToLower(err.Error()), "quota") {

		s.rateLimitState.consecutiveFailures++
		s.rateLimitState.lastFailureTime = time.Now()

		// Increase adaptive multiplier
		s.rateLimitState.adaptiveMultiplier = math.Min(5.0, s.rateLimitState.adaptiveMultiplier*1.5)

		// Set backoff period based on consecutive failures
		backoffDuration := time.Duration(s.rateLimitState.consecutiveFailures) * 30 * time.Second
		if backoffDuration > 5*time.Minute {
			backoffDuration = 5 * time.Minute
		}
		s.rateLimitState.backoffUntil = time.Now().Add(backoffDuration)

		s.logger.Warn("Rate limit failure detected, increasing adaptive backoff",
			"consecutive_failures", s.rateLimitState.consecutiveFailures,
			"multiplier", s.rateLimitState.adaptiveMultiplier,
			"backoff_until", s.rateLimitState.backoffUntil,
			"error", err)
	}
}

// retryWithBackoff executes a function with exponential backoff retry logic
func (s *Service) retryWithBackoff(ctx context.Context, operation string, fn func() error) error {
	maxRetries := s.config.RetryAttempts
	if maxRetries <= 0 {
		maxRetries = 3 // Default retry attempts
	}

	baseDelay := s.config.RetryDelay
	if baseDelay <= 0 {
		baseDelay = time.Second // Default base delay
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))

			// Add jitter to prevent thundering herd
			jitter := time.Duration(float64(delay) * 0.1 * (0.5 - float64(time.Now().UnixNano()%1000)/1000))
			delay += jitter

			// Cap maximum delay at 30 seconds
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			s.logger.Debug("Retrying operation with backoff",
				"operation", operation,
				"attempt", attempt,
				"delay", delay,
				"last_error", lastErr)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		lastErr = fn()
		if lastErr == nil {
			if attempt > 0 {
				s.logger.Info("Operation succeeded after retry",
					"operation", operation,
					"attempts", attempt+1)
			}
			return nil
		}

		// Check if this is a retryable error
		if !s.isRetryableError(lastErr) {
			s.logger.Debug("Non-retryable error, stopping retry",
				"operation", operation,
				"error", lastErr)
			break
		}

		s.logger.Debug("Operation failed, will retry",
			"operation", operation,
			"attempt", attempt+1,
			"max_attempts", maxRetries+1,
			"error", lastErr)
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w", operation, maxRetries+1, lastErr)
}

// isRetryableError determines if an error should trigger a retry
func (s *Service) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network-related errors that are typically transient
	errStr := strings.ToLower(err.Error())

	// Timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return true
	}

	// Connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection closed") {
		return true
	}

	// DNS errors
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dns") {
		return true
	}

	// Check for specific HTTP status codes that are retryable
	if strings.Contains(errStr, "HTTP error") {
		// Extract status code if possible
		if strings.Contains(errStr, "500") || // Internal Server Error
			strings.Contains(errStr, "502") || // Bad Gateway
			strings.Contains(errStr, "503") || // Service Unavailable
			strings.Contains(errStr, "504") || // Gateway Timeout
			strings.Contains(errStr, "429") { // Too Many Requests
			return true
		}
	}

	return false
}

// doHTTPRequestWithRetry performs an HTTP request with retry logic
func (s *Service) doHTTPRequestWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	retryErr := s.retryWithBackoff(ctx, fmt.Sprintf("HTTP %s %s", req.Method, req.URL), func() error {
		// Clone request for retry safety
		reqClone := req.Clone(ctx)

		resp, err = s.httpClient.Do(reqClone)
		if err != nil {
			return err
		}

		// Check for retryable HTTP status codes
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			resp.Body.Close()
			return fmt.Errorf("HTTP error: %d", resp.StatusCode)
		}

		return nil
	})

	if retryErr != nil {
		return nil, retryErr
	}

	return resp, nil
}

// ProxyManager methods

func (pm *ProxyManager) GetProxy(req *http.Request) (*url.URL, error) {
	if pm == nil || len(pm.proxies) == 0 {
		return nil, nil
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	proxy := pm.proxies[pm.currentIndex]
	pm.currentIndex = (pm.currentIndex + 1) % len(pm.proxies)

	return url.Parse(proxy)
}

// Data structures for parsing YouTube responses

type VideoData struct {
	VideoID       string
	Title         string
	Description   string
	ChannelID     string
	ChannelName   string
	PublishedAt   string
	ViewCount     int64
	LikeCount     int64
	CommentCount  int64
	IsLive        bool
	CaptionTracks []CaptionTrack
}

type PlayerResponse struct {
	VideoDetails *VideoDetails `json:"videoDetails"`
	Captions     *Captions     `json:"captions"`
}

type VideoDetails struct {
	VideoID          string `json:"videoId"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	ChannelID        string `json:"channelId"`
	Author           string `json:"author"`
	ViewCount        string `json:"viewCount"`
	IsLiveContent    bool   `json:"isLiveContent"`
}

type Captions struct {
	PlayerCaptionsTracklistRenderer PlayerCaptionsTracklistRenderer `json:"playerCaptionsTracklistRenderer"`
}

type PlayerCaptionsTracklistRenderer struct {
	CaptionTracks []CaptionTrack `json:"captionTracks"`
}

type CaptionTrack struct {
	BaseURL        string   `json:"baseUrl"`
	Name           NameText `json:"name"`
	VssID          string   `json:"vssId"`
	LanguageCode   string   `json:"languageCode"`
	Kind           string   `json:"kind,omitempty"`
	IsTranslatable bool     `json:"isTranslatable"`
	IsDefault      bool     `json:"isDefault,omitempty"`
}

type NameText struct {
	SimpleText string `json:"simpleText"`
}
