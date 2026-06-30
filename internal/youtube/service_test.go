package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/kyong0612/youtube-mcp/internal/config"
	"github.com/kyong0612/youtube-mcp/internal/models"
)

// Mock cache for testing
type mockCache struct {
	data map[string]any
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string]any),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) (any, bool) {
	val, ok := m.data[key]
	return val, ok
}

func (m *mockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCache) Clear(ctx context.Context) error {
	m.data = make(map[string]any)
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
		expected string
		seconds  float64
	}{
		{"00:00:00,000", 0.0},
		{"00:00:01,500", 1.5},
		{"00:01:05,123", 65.123},
		{"01:01:05,999", 3665.999},
		{"02:00:00,000", 7200.0},
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
		expected string
		seconds  float64
	}{
		{"00:00:00.000", 0.0},
		{"00:00:01.500", 1.5},
		{"00:01:05.123", 65.123},
		{"01:01:05.999", 3665.999},
		{"02:00:00.000", 7200.0},
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
		expected  string
		languages []string
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
		{Text: "Hello", Start: 0.0, Duration: 2.0, End: 2.0},
		{Text: "world", Start: 2.0, Duration: 3.0, End: 5.0},
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
		{Text: "Hello", Start: 0.0, Duration: 2.0, End: 2.0},
		{Text: "world", Start: 2.0, Duration: 3.0, End: 5.0},
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
	_, err = pm.GetProxy(req) // third proxy
	if err != nil {
		t.Fatalf("Failed to get third proxy: %v", err)
	}
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

func TestParseTranscriptXML(t *testing.T) {
	s := &Service{
		logger: slog.Default(),
	}

	tests := []struct {
		name        string
		xmlData     string
		expectError bool
		expectCount int
	}{
		{
			name: "standard transcript format",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<transcript>
	<text start="0" dur="2">Hello world</text>
	<text start="2" dur="3">This is a test</text>
</transcript>`,
			expectError: false,
			expectCount: 2,
		},
		{
			name: "timedtext format with paragraphs",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<timedtext>
	<head>
		<ws id="0"/>
		<wp id="0"/>
	</head>
	<body>
		<p t="0" d="2">
			<s>Hello world</s>
		</p>
		<p t="2" d="3">
			<s>This is a test</s>
		</p>
	</body>
</timedtext>`,
			expectError: false,
			expectCount: 2,
		},
		{
			name: "timedtext format with direct text elements",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<timedtext>
	<body>
		<text start="0" dur="2">Hello world</text>
		<text start="2" dur="3">This is a test</text>
	</body>
</timedtext>`,
			expectError: false,
			expectCount: 2,
		},
		{
			name:        "empty XML",
			xmlData:     "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			xmlData:     "   \n\t   ",
			expectError: true,
		},
		{
			name:        "invalid XML",
			xmlData:     "<invalid><unclosed>",
			expectError: true,
		},
		{
			name: "segments with zero duration",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<transcript>
	<text start="0" dur="0">Hello world</text>
	<text start="2" dur="0">This is a test</text>
</transcript>`,
			expectError: false,
			expectCount: 2,
		},
		{
			name: "HTML entities in text",
			xmlData: `<?xml version="1.0" encoding="utf-8"?>
<transcript>
	<text start="0" dur="2">Hello &amp; world &lt;test&gt;</text>
	<text start="2" dur="3">This is a &quot;test&quot;</text>
</transcript>`,
			expectError: false,
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := s.parseTranscriptXML([]byte(tt.xmlData))

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(segments) != tt.expectCount {
				t.Errorf("Expected %d segments, got %d", tt.expectCount, len(segments))
			}

			// Verify segments have valid data
			for i, segment := range segments {
				if segment.Text == "" {
					t.Errorf("Segment %d has empty text", i)
				}
				if segment.Duration <= 0 {
					// Should have default duration of 2.0
					if segment.Duration != 2.0 {
						t.Errorf("Segment %d has invalid duration: %f", i, segment.Duration)
					}
				}
			}
		})
	}
}

func TestRetryLogic(t *testing.T) {
	// Test retry with backoff
	s := &Service{
		config: config.YouTubeConfig{
			RetryAttempts: 3,
			RetryDelay:    10 * time.Millisecond,
		},
		logger: slog.Default(),
	}

	t.Run("successful operation on first try", func(t *testing.T) {
		attempts := 0
		err := s.retryWithBackoff(context.Background(), "test_op", func() error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("successful operation after retries", func(t *testing.T) {
		attempts := 0
		err := s.retryWithBackoff(context.Background(), "test_op", func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("timeout")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		err := s.retryWithBackoff(context.Background(), "test_op", func() error {
			attempts++
			return fmt.Errorf("invalid video id")
		})

		if err == nil {
			t.Error("Expected error but got none")
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := s.retryWithBackoff(ctx, "test_op", func() error {
			return fmt.Errorf("timeout")
		})

		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

func TestAdaptiveRateLimit(t *testing.T) {
	s := &Service{
		config: config.YouTubeConfig{
			RateLimitPerMinute: 60,
			RateLimitPerHour:   1000,
		},
		logger: slog.Default(),
		rateLimitState: &RateLimitState{
			adaptiveMultiplier: 1.0,
		},
	}

	// Initialize rate limiters
	s.rateLimiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(s.config.RateLimitPerMinute)), s.config.RateLimitPerMinute)
	s.hourlyLimiter = rate.NewLimiter(rate.Every(time.Hour/time.Duration(s.config.RateLimitPerHour)), s.config.RateLimitPerHour)

	t.Run("success reduces multiplier", func(t *testing.T) {
		// Set high multiplier
		s.rateLimitState.adaptiveMultiplier = 3.0

		s.recordRateLimitSuccess()

		if s.rateLimitState.adaptiveMultiplier >= 3.0 {
			t.Error("Expected multiplier to decrease")
		}
	})

	t.Run("rate limit failure increases multiplier", func(t *testing.T) {
		initialMultiplier := s.rateLimitState.adaptiveMultiplier

		s.recordRateLimitFailure(fmt.Errorf("rate limit exceeded"))

		if s.rateLimitState.adaptiveMultiplier <= initialMultiplier {
			t.Error("Expected multiplier to increase")
		}
	})

	t.Run("non-rate-limit failure doesn't affect multiplier", func(t *testing.T) {
		initialMultiplier := s.rateLimitState.adaptiveMultiplier

		s.recordRateLimitFailure(fmt.Errorf("network error"))

		if s.rateLimitState.adaptiveMultiplier != initialMultiplier {
			t.Error("Expected multiplier to remain unchanged")
		}
	})
}

func TestLanguageCodeMatches(t *testing.T) {
	tests := []struct {
		trackCode string
		requested string
		expected  bool
	}{
		{"en", "en", true},
		{"en-US", "en", true},
		{"en", "en-US", false},
		{"es", "en", false},
		{"ja", "ja", true},
		{"pt-BR", "pt", true},
	}

	for _, tt := range tests {
		if got := languageCodeMatches(tt.trackCode, tt.requested); got != tt.expected {
			t.Errorf("languageCodeMatches(%q, %q) = %v, want %v", tt.trackCode, tt.requested, got, tt.expected)
		}
	}
}

func TestAddTranslationParam(t *testing.T) {
	t.Run("adds tlang preserving existing params", func(t *testing.T) {
		out, err := addTranslationParam("https://www.youtube.com/api/timedtext?v=abc&lang=en", "ja")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		u, err := url.Parse(out)
		if err != nil {
			t.Fatalf("result is not a valid URL: %v", err)
		}
		q := u.Query()
		if q.Get("tlang") != "ja" {
			t.Errorf("expected tlang=ja, got %q", q.Get("tlang"))
		}
		if q.Get("v") != "abc" {
			t.Errorf("expected v=abc preserved, got %q", q.Get("v"))
		}
		if q.Get("lang") != "en" {
			t.Errorf("expected lang=en preserved, got %q", q.Get("lang"))
		}
	})

	t.Run("overwrites existing tlang", func(t *testing.T) {
		out, err := addTranslationParam("https://www.youtube.com/api/timedtext?v=abc&tlang=fr", "ja")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		u, _ := url.Parse(out)
		if got := u.Query().Get("tlang"); got != "ja" {
			t.Errorf("expected tlang overwritten to ja, got %q", got)
		}
	})

	t.Run("invalid URL returns error", func(t *testing.T) {
		if _, err := addTranslationParam("://not-a-url", "ja"); err == nil {
			t.Error("expected error for invalid URL, got nil")
		}
	})
}

func TestSelectTranslationSourceTrack(t *testing.T) {
	s := &Service{}

	tracks := []CaptionTrack{
		{LanguageCode: "en", IsTranslatable: true, IsDefault: true},
		{LanguageCode: "ja", IsTranslatable: true},
		{LanguageCode: "ko", IsTranslatable: false},
	}

	tests := []struct {
		name           string
		sourceLanguage string
		tracks         []CaptionTrack
		expectNil      bool
		expectedCode   string
	}{
		{
			name:           "matches requested source language",
			sourceLanguage: "ja",
			tracks:         tracks,
			expectedCode:   "ja",
		},
		{
			name:           "prefers default translatable when no source given",
			sourceLanguage: "",
			tracks:         tracks,
			expectedCode:   "en",
		},
		{
			name:           "falls back to first translatable when source not found",
			sourceLanguage: "fr",
			tracks:         tracks,
			expectedCode:   "en",
		},
		{
			name:           "first translatable when no default",
			sourceLanguage: "",
			tracks:         []CaptionTrack{{LanguageCode: "ko", IsTranslatable: false}, {LanguageCode: "ja", IsTranslatable: true}},
			expectedCode:   "ja",
		},
		{
			name:           "nil when no translatable tracks",
			sourceLanguage: "",
			tracks:         []CaptionTrack{{LanguageCode: "ko", IsTranslatable: false}},
			expectNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.selectTranslationSourceTrack(tt.tracks, tt.sourceLanguage)
			if tt.expectNil {
				if got != nil {
					t.Errorf("expected nil track, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected a track but got nil")
			}
			if got.LanguageCode != tt.expectedCode {
				t.Errorf("expected language %q, got %q", tt.expectedCode, got.LanguageCode)
			}
		})
	}
}

// newTestService builds a Service backed by a mock cache and generous rate limits,
// suitable for exercising the network paths against an httptest server.
func newTestService() *Service {
	cfg := config.YouTubeConfig{
		DefaultLanguages:   []string{"en"},
		RequestTimeout:     30 * time.Second,
		RetryAttempts:      2,
		RetryDelay:         time.Millisecond,
		RateLimitPerMinute: 6000,
		RateLimitPerHour:   60000,
		UserAgent:          "test-agent",
	}
	return NewService(cfg, newMockCache(), slog.Default())
}

// hostRewriteTransport redirects every outgoing request to a fixed target host
// (the httptest server) while preserving the path and query, so code that hardcodes
// www.youtube.com can be tested without live network access.
type hostRewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (t *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	return t.base.RoundTrip(req)
}

const jaTranslatedXML = `<?xml version="1.0" encoding="utf-8"?>
<transcript>
	<text start="0" dur="2">こんにちは世界</text>
	<text start="2" dur="3">これはテストです</text>
</transcript>`

func buildWatchHTML(t *testing.T, tracks []CaptionTrack) string {
	t.Helper()
	pr := PlayerResponse{
		VideoDetails: &VideoDetails{
			Title:     "Test Video",
			ChannelID: "chan-1",
			Author:    "Author",
			ViewCount: "100",
		},
		Captions: &Captions{
			PlayerCaptionsTracklistRenderer: PlayerCaptionsTracklistRenderer{
				CaptionTracks: tracks,
			},
		},
	}
	b, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("failed to marshal player response: %v", err)
	}
	// The parser extracts a single-line `var ytInitialPlayerResponse = {...};` assignment.
	return "<html><body><script>var ytInitialPlayerResponse = " + string(b) + ";</script></body></html>"
}

func TestFetchTranslatedTranscript(t *testing.T) {
	var capturedTLang string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTLang = r.URL.Query().Get("tlang")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(jaTranslatedXML))
	}))
	defer server.Close()

	s := newTestService()

	sourceTrack := &CaptionTrack{
		BaseURL:        server.URL + "/api/timedtext?v=abc&lang=en",
		LanguageCode:   "en",
		IsTranslatable: true,
	}
	videoData := &VideoData{VideoID: "abc123video", Title: "Test Video", ChannelName: "Author"}

	resp, err := s.fetchTranslatedTranscript(context.Background(), videoData, sourceTrack, "ja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedTLang != "ja" {
		t.Errorf("expected tlang=ja sent to timedtext endpoint, got %q", capturedTLang)
	}
	if resp.Language != "ja" {
		t.Errorf("expected Language=ja, got %q", resp.Language)
	}
	if resp.TranscriptType != transcriptTypeTranslated {
		t.Errorf("expected TranscriptType=%q, got %q", transcriptTypeTranslated, resp.TranscriptType)
	}
	if resp.Metadata.Source != translatedSourceMarker {
		t.Errorf("expected Source=%q, got %q", translatedSourceMarker, resp.Metadata.Source)
	}
	if len(resp.Transcript) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(resp.Transcript))
	}
	if resp.Transcript[0].Text != "こんにちは世界" {
		t.Errorf("unexpected first segment text: %q", resp.Transcript[0].Text)
	}
}

func TestTranslateTranscript(t *testing.T) {
	t.Run("auto-translates via tlang when target track is missing", func(t *testing.T) {
		var timedtextTLang string
		var timedtextHit bool
		server := newTranslateTestServer(t,
			[]CaptionTrack{{
				LanguageCode:   "en",
				IsTranslatable: true,
				IsDefault:      true,
			}},
			func(r *http.Request) {
				timedtextHit = true
				timedtextTLang = r.URL.Query().Get("tlang")
			},
		)
		defer server.Close()

		s := newTranslateTestService(t, server)

		resp, err := s.TranslateTranscript(context.Background(), "abc123video", "ja", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !timedtextHit {
			t.Fatal("expected timedtext endpoint to be called")
		}
		if timedtextTLang != "ja" {
			t.Errorf("expected tlang=ja, got %q", timedtextTLang)
		}
		if resp.Language != "ja" {
			t.Errorf("expected Language=ja, got %q", resp.Language)
		}
		if resp.TranscriptType != transcriptTypeTranslated {
			t.Errorf("expected translated transcript type, got %q", resp.TranscriptType)
		}
		if resp.Metadata.Source != translatedSourceMarker {
			t.Errorf("expected source marker %q, got %q", translatedSourceMarker, resp.Metadata.Source)
		}
		if len(resp.Transcript) != 2 {
			t.Errorf("expected 2 segments, got %d", len(resp.Transcript))
		}
	})

	t.Run("returns native track without tlang when target exists", func(t *testing.T) {
		var timedtextTLang string
		var sawTLang bool
		server := newTranslateTestServer(t,
			[]CaptionTrack{
				{LanguageCode: "en", IsTranslatable: true, IsDefault: true},
				{LanguageCode: "ja", IsTranslatable: false},
			},
			func(r *http.Request) {
				if r.URL.Query().Has("tlang") {
					sawTLang = true
					timedtextTLang = r.URL.Query().Get("tlang")
				}
			},
		)
		defer server.Close()

		s := newTranslateTestService(t, server)

		resp, err := s.TranslateTranscript(context.Background(), "abc123video", "ja", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sawTLang {
			t.Errorf("expected no tlang for native track, but saw tlang=%q", timedtextTLang)
		}
		if resp.Language != "ja" {
			t.Errorf("expected Language=ja, got %q", resp.Language)
		}
	})

	t.Run("errors when no translatable track exists", func(t *testing.T) {
		server := newTranslateTestServer(t,
			[]CaptionTrack{{LanguageCode: "en", IsTranslatable: false, IsDefault: true}},
			nil,
		)
		defer server.Close()

		s := newTranslateTestService(t, server)

		_, err := s.TranslateTranscript(context.Background(), "abc123video", "ja", "")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		var transcriptErr *models.TranscriptError
		if te, ok := err.(*models.TranscriptError); ok {
			transcriptErr = te
		} else {
			t.Fatalf("expected *models.TranscriptError, got %T", err)
		}
		if transcriptErr.Type != models.ErrorTypeLanguageNotAvailable {
			t.Errorf("expected error type %q, got %q", models.ErrorTypeLanguageNotAvailable, transcriptErr.Type)
		}
		if len(transcriptErr.Suggestions) == 0 || transcriptErr.Suggestions[0] != "en" {
			t.Errorf("expected suggestions to list available languages, got %v", transcriptErr.Suggestions)
		}
	})
}

// newTranslateTestServer serves a YouTube-like watch page whose caption tracks point
// back at the same server's /api/timedtext endpoint. onTimedtext, when non-nil, is
// invoked for each timedtext request so tests can assert on the query parameters.
func newTranslateTestServer(t *testing.T, tracks []CaptionTrack, onTimedtext func(*http.Request)) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/timedtext"):
			if onTimedtext != nil {
				onTimedtext(r)
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(jaTranslatedXML))
		case strings.HasPrefix(r.URL.Path, "/watch"):
			// Point caption track baseUrls at this server's timedtext endpoint.
			resolved := make([]CaptionTrack, len(tracks))
			for i, tr := range tracks {
				tr.BaseURL = server.URL + "/api/timedtext?v=abc123video&lang=" + tr.LanguageCode
				resolved[i] = tr
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(buildWatchHTML(t, resolved)))
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

// newTranslateTestService returns a Service whose HTTP client redirects all requests
// (including the hardcoded www.youtube.com watch URL) to the test server.
func newTranslateTestService(t *testing.T, server *httptest.Server) *Service {
	t.Helper()
	s := newTestService()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed to parse server URL: %v", err)
	}
	s.httpClient.Transport = &hostRewriteTransport{
		base:   http.DefaultTransport,
		target: target,
	}
	return s
}
