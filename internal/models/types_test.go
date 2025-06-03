package models

import (
	"testing"
	"time"
)

func TestTranscriptError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *TranscriptError
		want string
	}{
		{
			name: "simple error message",
			err: &TranscriptError{
				Type:    ErrorTypeVideoUnavailable,
				Message: "Video not found",
				VideoID: "test123",
			},
			want: "Video not found",
		},
		{
			name: "error with suggestions",
			err: &TranscriptError{
				Type:        ErrorTypeLanguageNotAvailable,
				Message:     "Language not available",
				VideoID:     "test456",
				Suggestions: []string{"en", "ja"},
			},
			want: "Language not available",
		},
		{
			name: "error with retry after",
			err: &TranscriptError{
				Type:       ErrorTypeRateLimitExceeded,
				Message:    "Rate limit exceeded",
				RetryAfter: 60,
			},
			want: "Rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("TranscriptError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *MCPError
		want string
	}{
		{
			name: "parse error",
			err: &MCPError{
				Code:    MCPErrorCodeParseError,
				Message: "Invalid JSON",
			},
			want: "Invalid JSON",
		},
		{
			name: "method not found with data",
			err: &MCPError{
				Code:    MCPErrorCodeMethodNotFound,
				Message: "Method not found",
				Data:    "unknown_method",
			},
			want: "Method not found",
		},
		{
			name: "internal error",
			err: &MCPError{
				Code:    MCPErrorCodeInternalError,
				Message: "Internal server error",
				Data:    map[string]interface{}{"details": "database connection failed"},
			},
			want: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("MCPError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants have expected values
	errorTypes := map[string]string{
		"VIDEO_UNAVAILABLE":      ErrorTypeVideoUnavailable,
		"NO_TRANSCRIPT_FOUND":    ErrorTypeNoTranscriptFound,
		"TRANSCRIPTS_DISABLED":   ErrorTypeTranscriptsDisabled,
		"INVALID_VIDEO_ID":       ErrorTypeInvalidVideoID,
		"NETWORK_ERROR":          ErrorTypeNetworkError,
		"RATE_LIMIT_EXCEEDED":    ErrorTypeRateLimitExceeded,
		"LANGUAGE_NOT_AVAILABLE": ErrorTypeLanguageNotAvailable,
		"INTERNAL_ERROR":         ErrorTypeInternalError,
		"VALIDATION_ERROR":       ErrorTypeValidationError,
		"AUTHENTICATION_ERROR":   ErrorTypeAuthenticationError,
		"PARSING_ERROR":          ErrorTypeParsingError,
		"TIMEOUT_ERROR":          ErrorTypeTimeout,
		"CAPTCHA_REQUIRED":       ErrorTypeCaptchaRequired,
	}

	for expected, actual := range errorTypes {
		if actual != expected {
			t.Errorf("Expected error type constant %s to equal %s, got %s", expected, expected, actual)
		}
	}
}

func TestMCPErrorCodes(t *testing.T) {
	// Test MCP error codes match JSON-RPC 2.0 specification
	if MCPErrorCodeParseError != -32700 {
		t.Errorf("MCPErrorCodeParseError should be -32700, got %d", MCPErrorCodeParseError)
	}
	if MCPErrorCodeInvalidRequest != -32600 {
		t.Errorf("MCPErrorCodeInvalidRequest should be -32600, got %d", MCPErrorCodeInvalidRequest)
	}
	if MCPErrorCodeMethodNotFound != -32601 {
		t.Errorf("MCPErrorCodeMethodNotFound should be -32601, got %d", MCPErrorCodeMethodNotFound)
	}
	if MCPErrorCodeInvalidParams != -32602 {
		t.Errorf("MCPErrorCodeInvalidParams should be -32602, got %d", MCPErrorCodeInvalidParams)
	}
	if MCPErrorCodeInternalError != -32603 {
		t.Errorf("MCPErrorCodeInternalError should be -32603, got %d", MCPErrorCodeInternalError)
	}
}

func TestDefaultValues(t *testing.T) {
	// Test default constant values
	if DefaultLanguage != "en" {
		t.Errorf("DefaultLanguage should be 'en', got %s", DefaultLanguage)
	}
	if DefaultFormatType != FormatTypePlainText {
		t.Errorf("DefaultFormatType should be 'plain_text', got %s", DefaultFormatType)
	}
	if DefaultMaxLineLength != 80 {
		t.Errorf("DefaultMaxLineLength should be 80, got %d", DefaultMaxLineLength)
	}
	if DefaultCacheTTL != 24*time.Hour {
		t.Errorf("DefaultCacheTTL should be 24 hours, got %v", DefaultCacheTTL)
	}
	if DefaultTimeout != 30*time.Second {
		t.Errorf("DefaultTimeout should be 30 seconds, got %v", DefaultTimeout)
	}
}

func TestTranscriptSegment(t *testing.T) {
	segment := TranscriptSegment{
		Text:     "Hello world",
		Start:    10.5,
		Duration: 2.3,
		End:      12.8,
	}

	if segment.Text != "Hello world" {
		t.Errorf("Expected text 'Hello world', got %s", segment.Text)
	}
	if segment.End != segment.Start+segment.Duration {
		t.Errorf("End time should equal Start + Duration")
	}
}

func TestCacheEntry(t *testing.T) {
	now := time.Now()
	entry := CacheEntry{
		Key:       "test:key",
		Data:      "test data",
		Timestamp: now,
		TTL:       time.Hour,
		HitCount:  5,
	}

	if entry.Key != "test:key" {
		t.Errorf("Expected key 'test:key', got %s", entry.Key)
	}
	if entry.HitCount != 5 {
		t.Errorf("Expected hit count 5, got %d", entry.HitCount)
	}
}