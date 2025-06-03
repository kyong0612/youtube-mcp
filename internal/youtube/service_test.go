package youtube

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/models"
)

// Mock cache for testing
type mockCache struct {
	data map[string]interface{}
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]interface{}),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCache) Clear(ctx context.Context) error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *mockCache) Size(ctx context.Context) int {
	return len(m.data)
}

func (m *mockCache) Close() error {
	return nil
}

func TestExtractVideoID(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "valid video ID",
			input:    "dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtube.com watch URL",
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtu.be short URL",
			input:    "https://youtu.be/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtube.com embed URL",
			input:    "https://www.youtube.com/embed/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtube.com v URL",
			input:    "https://www.youtube.com/v/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtube.com shorts URL",
			input:    "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtube.com live URL",
			input:    "https://www.youtube.com/live/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "URL with additional parameters",
			input:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:      "invalid URL",
			input:     "https://example.com/video",
			expectErr: true,
		},
		{
			name:      "empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "invalid video ID format",
			input:     "invalid123",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.extractVideoID(tt.input)
			
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestCleanTranscriptText(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTML entities",
			input:    "Hello &amp; world &lt;test&gt; &quot;quoted&quot; &#39;single&#39;",
			expected: "Hello & world <test> \"quoted\" 'single'",
		},
		{
			name:     "non-breaking space",
			input:    "Hello&nbsp;world",
			expected: "Hello world",
		},
		{
			name:     "line breaks",
			input:    "Hello\nworld\r\ntest",
			expected: "Hello world test",
		},
		{
			name:     "multiple spaces",
			input:    "Hello    world     test",
			expected: "Hello world test",
		},
		{
			name:     "leading and trailing spaces",
			input:    "   Hello world   ",
			expected: "Hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.cleanTranscriptText(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatSRTTime(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{65.123, "00:01:05,123"},
		{3665.999, "01:01:05,999"},
		{7200, "02:00:00,000"},
	}

	for _, tt := range tests {
		result := s.formatSRTTime(tt.seconds)
		if result != tt.expected {
			t.Errorf("formatSRTTime(%f) = %s, want %s", tt.seconds, result, tt.expected)
		}
	}
}

func TestFormatVTTTime(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{65.123, "00:01:05.123"},
		{3665.999, "01:01:05.999"},
		{7200, "02:00:00.000"},
	}

	for _, tt := range tests {
		result := s.formatVTTTime(tt.seconds)
		if result != tt.expected {
			t.Errorf("formatVTTTime(%f) = %s, want %s", tt.seconds, result, tt.expected)
		}
	}
}

func TestCountWords(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		text     string
		expected int
	}{
		{"Hello world", 2},
		{"  Multiple   spaces  ", 2},
		{"One", 1},
		{"", 0},
		{"   ", 0},
		{"Hello, world! How are you?", 5},
	}

	for _, tt := range tests {
		result := s.countWords(tt.text)
		if result != tt.expected {
			t.Errorf("countWords(%q) = %d, want %d", tt.text, result, tt.expected)
		}
	}
}

func TestCalculateDuration(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		segments []models.TranscriptSegment
		expected float64
	}{
		{
			segments: []models.TranscriptSegment{
				{Start: 0, Duration: 2, End: 2},
				{Start: 2, Duration: 3, End: 5},
				{Start: 5, Duration: 2.5, End: 7.5},
			},
			expected: 7.5,
		},
		{
			segments: []models.TranscriptSegment{},
			expected: 0,
		},
		{
			segments: []models.TranscriptSegment{
				{Start: 10, Duration: 5, End: 15},
			},
			expected: 15,
		},
	}

	for _, tt := range tests {
		result := s.calculateDuration(tt.segments)
		if result != tt.expected {
			t.Errorf("calculateDuration() = %f, want %f", result, tt.expected)
		}
	}
}

func TestSelectBestTrack(t *testing.T) {
	s := &Service{}
	
	tracks := []CaptionTrack{
		{LanguageCode: "en", IsDefault: true},
		{LanguageCode: "ja"},
		{LanguageCode: "es"},
		{LanguageCode: "en-US"},
		{LanguageCode: "en-GB"},
	}

	tests := []struct {
		name      string
		languages []string
		expected  string
	}{
		{
			name:      "exact match",
			languages: []string{"ja"},
			expected:  "ja",
		},
		{
			name:      "prefer first in list",
			languages: []string{"fr", "es", "ja"},
			expected:  "es",
		},
		{
			name:      "prefix match",
			languages: []string{"en"},
			expected:  "en",
		},
		{
			name:      "default when no match",
			languages: []string{"fr", "de"},
			expected:  "en",
		},
		{
			name:      "empty languages uses default",
			languages: []string{},
			expected:  "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.selectBestTrack(tracks, tt.languages)
			if result == nil {
				t.Fatal("Expected track but got nil")
			}
			if result.LanguageCode != tt.expected {
				t.Errorf("Expected language %s, got %s", tt.expected, result.LanguageCode)
			}
		})
	}
}

func TestGetTranscriptType(t *testing.T) {
	s := &Service{}
	
	tests := []struct {
		track    *CaptionTrack
		expected string
	}{
		{
			track:    &CaptionTrack{Kind: "asr"},
			expected: models.TranscriptTypeAuto,
		},
		{
			track:    &CaptionTrack{VssID: "a.en"},
			expected: models.TranscriptTypeGenerated,
		},
		{
			track:    &CaptionTrack{Kind: "", VssID: "en"},
			expected: models.TranscriptTypeManual,
		},
	}

	for _, tt := range tests {
		result := s.getTranscriptType(tt.track)
		if result != tt.expected {
			t.Errorf("Expected type %s, got %s", tt.expected, result)
		}
	}
}

func TestFormatAsPlainText(t *testing.T) {
	s := &Service{}
	
	segments := []models.TranscriptSegment{
		{Text: "Hello", Start: 0},
		{Text: "world", Start: 2},
		{Text: "test", Start: 5},
	}

	// Without timestamps
	result := s.formatAsPlainText(segments, false)
	expected := "Hello world test"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// With timestamps
	result = s.formatAsPlainText(segments, true)
	if !strings.Contains(result, "[0.0s]") || !strings.Contains(result, "[2.0s]") {
		t.Error("Expected timestamps in output")
	}
}

func TestFormatAsSRT(t *testing.T) {
	s := &Service{}
	
	segments := []models.TranscriptSegment{
		{Text: "Hello", Start: 0, Duration: 2, End: 2},
		{Text: "world", Start: 2, Duration: 3, End: 5},
	}

	result := s.formatAsSRT(segments)
	
	// Check for SRT format markers
	if !strings.Contains(result, "1\n") {
		t.Error("Expected sequence number")
	}
	if !strings.Contains(result, "00:00:00,000 --> 00:00:02,000") {
		t.Error("Expected timestamp range")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("Expected transcript text")
	}
}

func TestFormatAsVTT(t *testing.T) {
	s := &Service{}
	
	segments := []models.TranscriptSegment{
		{Text: "Hello", Start: 0, Duration: 2, End: 2},
		{Text: "world", Start: 2, Duration: 3, End: 5},
	}

	result := s.formatAsVTT(segments)
	
	// Check for VTT format markers
	if !strings.HasPrefix(result, "WEBVTT") {
		t.Error("Expected WEBVTT header")
	}
	if !strings.Contains(result, "00:00:00.000 --> 00:00:02.000") {
		t.Error("Expected timestamp range")
	}
}

func TestProxyManager(t *testing.T) {
	proxies := []string{
		"http://proxy1.com:8080",
		"http://proxy2.com:8080",
		"http://proxy3.com:8080",
	}
	
	pm := &ProxyManager{
		proxies: proxies,
	}

	// Test rotation
	req := httptest.NewRequest("GET", "http://example.com", nil)
	
	proxy1, err := pm.GetProxy(req)
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}
	if proxy1.String() != proxies[0] {
		t.Errorf("Expected first proxy %s, got %s", proxies[0], proxy1.String())
	}

	proxy2, err := pm.GetProxy(req)
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}
	if proxy2.String() != proxies[1] {
		t.Errorf("Expected second proxy %s, got %s", proxies[1], proxy2.String())
	}

	// Test wrap around
	pm.GetProxy(req) // third proxy
	proxy4, err := pm.GetProxy(req)
	if err != nil {
		t.Fatalf("Failed to get proxy: %v", err)
	}
	if proxy4.String() != proxies[0] {
		t.Errorf("Expected to wrap around to first proxy %s, got %s", proxies[0], proxy4.String())
	}
}

func TestServiceWithCache(t *testing.T) {
	cfg := config.YouTubeConfig{
		DefaultLanguages:   []string{"en"},
		RequestTimeout:     30 * time.Second,
		RetryAttempts:      3,
		RetryDelay:         time.Second,
		RateLimitPerMinute: 60,
		UserAgent:          "test-agent",
	}
	
	mockCache := newMockCache()
	logger := slog.Default()
	
	service := NewService(cfg, mockCache, logger)
	
	// Test that service is properly initialized
	if service.config.UserAgent != "test-agent" {
		t.Errorf("Expected user agent 'test-agent', got '%s'", service.config.UserAgent)
	}
	if service.cache == nil {
		t.Error("Expected cache to be set")
	}
	if service.logger == nil {
		t.Error("Expected logger to be set")
	}
}

// Mock HTTP server for testing
func setupMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			// Return mock YouTube page
			w.Write([]byte(`
				<html>
				<script>
				var ytInitialPlayerResponse = {
					"videoDetails": {
						"videoId": "test123",
						"title": "Test Video",
						"shortDescription": "Test Description",
						"channelId": "channel123",
						"author": "Test Channel",
						"viewCount": "1000",
						"isLiveContent": false
					},
					"captions": {
						"playerCaptionsTracklistRenderer": {
							"captionTracks": [{
								"baseUrl": "/api/timedtext?v=test123&lang=en",
								"name": {"simpleText": "English"},
								"vssId": "en",
								"languageCode": "en",
								"isTranslatable": true,
								"isDefault": true
							}]
						}
					}
				};
				</script>
				</html>
			`))
		case "/api/timedtext":
			// Return mock transcript XML
			w.Header().Set("Content-Type", "text/xml")
			w.Write([]byte(`
				<?xml version="1.0" encoding="utf-8"?>
				<transcript>
					<text start="0" dur="2">Hello world</text>
					<text start="2" dur="3">This is a test</text>
				</transcript>
			`))
		default:
			w.WriteHeader(404)
		}
	}))
}