package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/models"
)

// Mock YouTube service for testing
type mockYouTubeService struct {
	getTranscriptFunc          func(ctx context.Context, videoID string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error)
	getMultipleTranscriptsFunc func(ctx context.Context, videoIDs []string, languages []string, continueOnError bool) (*models.MultipleTranscriptResponse, error)
	listAvailableLanguagesFunc func(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error)
	translateTranscriptFunc    func(ctx context.Context, videoID, targetLang, sourceLang string) (*models.TranscriptResponse, error)
	formatTranscriptFunc       func(ctx context.Context, videoID, formatType string, includeTimestamps bool) (*models.TranscriptResponse, error)
}

func (m *mockYouTubeService) GetTranscript(ctx context.Context, videoID string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error) {
	if m.getTranscriptFunc != nil {
		return m.getTranscriptFunc(ctx, videoID, languages, preserveFormatting)
	}
	return &models.TranscriptResponse{
		VideoID:  videoID,
		Language: "en",
		Transcript: []models.TranscriptSegment{
			{Text: "Hello", Start: 0, Duration: 2},
		},
	}, nil
}

func (m *mockYouTubeService) GetMultipleTranscripts(ctx context.Context, videoIDs []string, languages []string, continueOnError bool) (*models.MultipleTranscriptResponse, error) {
	if m.getMultipleTranscriptsFunc != nil {
		return m.getMultipleTranscriptsFunc(ctx, videoIDs, languages, continueOnError)
	}
	return &models.MultipleTranscriptResponse{
		Results: []models.TranscriptResult{
			{VideoID: videoIDs[0], Success: true},
		},
		TotalCount:   len(videoIDs),
		SuccessCount: len(videoIDs),
	}, nil
}

func (m *mockYouTubeService) ListAvailableLanguages(ctx context.Context, videoID string) (*models.AvailableLanguagesResponse, error) {
	if m.listAvailableLanguagesFunc != nil {
		return m.listAvailableLanguagesFunc(ctx, videoID)
	}
	return &models.AvailableLanguagesResponse{
		VideoID: videoID,
		Languages: []models.LanguageInfo{
			{Code: "en", Name: "English", Type: "manual"},
		},
	}, nil
}

func (m *mockYouTubeService) TranslateTranscript(ctx context.Context, videoID, targetLang, sourceLang string) (*models.TranscriptResponse, error) {
	if m.translateTranscriptFunc != nil {
		return m.translateTranscriptFunc(ctx, videoID, targetLang, sourceLang)
	}
	return &models.TranscriptResponse{
		VideoID:  videoID,
		Language: targetLang,
	}, nil
}

func (m *mockYouTubeService) FormatTranscript(ctx context.Context, videoID, formatType string, includeTimestamps bool) (*models.TranscriptResponse, error) {
	if m.formatTranscriptFunc != nil {
		return m.formatTranscriptFunc(ctx, videoID, formatType, includeTimestamps)
	}
	return &models.TranscriptResponse{
		VideoID:       videoID,
		FormattedText: "Formatted text",
	}, nil
}

func TestHandleMCP_Initialize(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		Version:        "2024-11-05",
		ServerName:     "test-server",
		ServerVersion:  "1.0.0",
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
		Tools: map[string]bool{
			"get_transcript": true,
		},
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  models.MCPMethodInitialize,
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
	}

	result, ok := response.Result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	if result["protocolVersion"] != cfg.Version {
		t.Errorf("Expected protocol version %s, got %v", cfg.Version, result["protocolVersion"])
	}
}

func TestHandleMCP_ListTools(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
		Tools: map[string]bool{
			"get_transcript":           true,
			"get_multiple_transcripts": true,
			"translate_transcript":     false,
		},
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  models.MCPMethodListTools,
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	result, ok := response.Result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatal("Expected tools to be an array")
	}

	// Should only have 2 enabled tools
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}

func TestHandleMCP_CallTool_GetTranscript(t *testing.T) {
	mockYT := &mockYouTubeService{
		getTranscriptFunc: func(ctx context.Context, videoID string, languages []string, preserveFormatting bool) (*models.TranscriptResponse, error) {
			return &models.TranscriptResponse{
				VideoID:  videoID,
				Title:    "Test Video",
				Language: "en",
				Transcript: []models.TranscriptSegment{
					{Text: "Hello world", Start: 0, Duration: 2},
				},
				WordCount: 2,
			}, nil
		},
	}

	cfg := config.MCPConfig{
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
		Tools: map[string]bool{
			"get_transcript": true,
		},
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  models.MCPMethodCallTool,
		Params: map[string]any{
			"name": "get_transcript",
			"arguments": map[string]any{
				"video_identifier": "test123",
				"languages":        []string{"en"},
			},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Error != nil {
		t.Errorf("Unexpected error: %v", response.Error)
	}

	result, ok := response.Result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatal("Expected content array")
	}
}

func TestHandleMCP_InvalidMethod(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "invalid/method",
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error for invalid method")
	}

	if response.Error.Code != models.MCPErrorCodeMethodNotFound {
		t.Errorf("Expected method not found error, got %d", response.Error.Code)
	}
}

func TestHandleMCP_InvalidJSON(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error for invalid JSON")
	}

	if response.Error.Code != models.MCPErrorCodeParseError {
		t.Errorf("Expected parse error, got %d", response.Error.Code)
	}
}

func TestHandleMCP_DisabledTool(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
		Tools: map[string]bool{
			"get_transcript": false, // Disabled
		},
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  models.MCPMethodCallTool,
		Params: map[string]any{
			"name": "get_transcript",
			"arguments": map[string]any{
				"video_identifier": "test123",
			},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error for disabled tool")
	}
}

func TestGetStats(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		ServerVersion:  "1.0.0",
		Version:        "2024-11-05",
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
		Tools: map[string]bool{
			"get_transcript":       true,
			"translate_transcript": true,
		},
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	// Make a request to increment counter
	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  models.MCPMethodInitialize,
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	// Get stats
	stats := server.GetStats()

	if stats["request_count"].(int64) < 1 {
		t.Error("Expected request count to be at least 1")
	}

	if stats["enabled_tools"].(int) != 2 {
		t.Errorf("Expected 2 enabled tools, got %v", stats["enabled_tools"])
	}

	if stats["server_version"] != "1.0.0" {
		t.Errorf("Expected server version 1.0.0, got %v", stats["server_version"])
	}
}

func TestMapToStruct(t *testing.T) {
	server := &Server{}

	input := map[string]any{
		"video_identifier":    "test123",
		"languages":           []any{"en", "ja"},
		"preserve_formatting": true,
	}

	var params models.GetTranscriptParams
	err := server.mapToStruct(input, &params)

	if err != nil {
		t.Fatalf("Failed to map to struct: %v", err)
	}

	if params.VideoIdentifier != "test123" {
		t.Errorf("Expected video_identifier 'test123', got %s", params.VideoIdentifier)
	}

	if len(params.Languages) != 2 {
		t.Errorf("Expected 2 languages, got %d", len(params.Languages))
	}

	if !params.PreserveFormatting {
		t.Error("Expected preserve_formatting to be true")
	}
}

func TestValidation(t *testing.T) {
	mockYT := &mockYouTubeService{}
	cfg := config.MCPConfig{
		MaxRequestSize: 5 * 1024 * 1024, // 5MB
		RequestTimeout: 60 * time.Second,
		Tools: map[string]bool{
			"get_transcript": true,
		},
	}
	logger := slog.Default()

	server := NewServer(mockYT, cfg, logger)

	// Test with missing required field
	request := models.MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  models.MCPMethodCallTool,
		Params: map[string]any{
			"name": "get_transcript",
			"arguments": map[string]any{
				// Missing video_identifier
				"languages": []string{"en"},
			},
		},
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.HandleMCP(rec, req)

	var response models.MCPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected validation error")
	}

	if response.Error.Code != models.MCPErrorCodeInvalidParams {
		t.Errorf("Expected invalid params error, got %d", response.Error.Code)
	}
}
