package youtube

import (
	"context"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/models"
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
	_, _ = pm.GetProxy(req) // third proxy
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
