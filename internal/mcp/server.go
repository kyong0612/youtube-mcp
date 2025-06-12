package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/youtube-transcript-mcp/internal/config"
	"github.com/youtube-transcript-mcp/internal/models"
)

// Server implements the MCP server
type Server struct {
	youtube      YouTubeService
	validator    *validator.Validate
	logger       *slog.Logger
	activeTools  sync.Map
	config       config.MCPConfig
	requestCount int64
	mu           sync.RWMutex
}

// NewServer creates a new MCP server instance
func NewServer(youtubeService YouTubeService, cfg config.MCPConfig, logger *slog.Logger) *Server {
	return &Server{
		youtube:   youtubeService,
		config:    cfg,
		validator: validator.New(),
		logger:    logger,
	}
}

// HandleMCP handles MCP requests
func (s *Server) HandleMCP(w http.ResponseWriter, r *http.Request) {
	// Check request size limit
	if r.ContentLength > s.config.MaxRequestSize {
		s.sendError(w, nil, models.MCPErrorCodeInvalidRequest, "Request too large", nil)
		return
	}

	// Set timeout for request processing
	ctx, cancel := context.WithTimeout(r.Context(), s.config.RequestTimeout)
	defer cancel()

	var request models.MCPRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.sendError(w, nil, models.MCPErrorCodeParseError, "Parse error", err.Error())
		return
	}

	// Log request
	s.logger.Debug("Received MCP request",
		slog.String("method", request.Method),
		slog.Any("id", request.ID),
	)

	// Increment request count
	s.incrementRequestCount()

	var response *models.MCPResponse
	var err error

	switch request.Method {
	case models.MCPMethodInitialize:
		response = s.handleInitialize(ctx, request)
	case models.MCPMethodListTools:
		response = s.handleListTools(ctx, request)
	case models.MCPMethodCallTool:
		response = s.handleCallTool(ctx, request)
	case models.MCPMethodListResources:
		response = s.handleListResources(ctx, request)
	case models.MCPMethodReadResource:
		response = s.handleReadResource(ctx, request)
	case models.MCPMethodListPrompts:
		response = s.handleListPrompts(ctx, request)
	case models.MCPMethodGetPrompt:
		response = s.handleGetPrompt(ctx, request)
	case models.MCPMethodSetLoggingLevel:
		response = s.handleSetLoggingLevel(ctx, request)
	default:
		s.sendError(w, request.ID, models.MCPErrorCodeMethodNotFound, "Method not found", request.Method)
		return
	}

	if err != nil {
		s.logger.Error("Error handling MCP request",
			slog.String("method", request.Method),
			slog.Any("error", err),
		)
		s.sendError(w, request.ID, models.MCPErrorCodeInternalError, "Internal error", err.Error())
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode response", slog.Any("error", err))
	}
}

// handleInitialize handles the initialize method
func (s *Server) handleInitialize(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	result := models.MCPInitializeResponse{
		ProtocolVersion: s.config.Version,
		ServerInfo: models.MCPServerInfo{
			Name:    s.config.ServerName,
			Version: s.config.ServerVersion,
		},
		Capabilities: models.MCPServerCapabilities{
			Tools: models.MCPToolsCapability{
				ListChanged: false,
			},
		},
	}

	// Add optional capabilities
	if s.config.EnableResources {
		result.Capabilities.Resources = models.MCPResourcesCapability{
			Subscribe:   false,
			ListChanged: false,
		}
	}

	if s.config.EnablePrompts {
		result.Capabilities.Prompts = models.MCPPromptsCapability{
			ListChanged: false,
		}
	}

	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}
}

// handleListTools handles the tools/list method
func (s *Server) handleListTools(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	tools := s.getAvailableTools()

	result := models.MCPToolsListResponse{
		Tools: tools,
	}

	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}
}

// handleCallTool handles the tools/call method
func (s *Server) handleCallTool(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	params, ok := request.Params.(map[string]any)
	if !ok {
		return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, "Invalid parameters")
	}

	// Extract tool call parameters
	var toolCall models.MCPToolCallParams
	paramsBytes, _ := json.Marshal(params)
	if err := json.Unmarshal(paramsBytes, &toolCall); err != nil {
		return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, "Invalid tool call parameters")
	}

	// Check if tool is enabled
	if !s.config.Tools[toolCall.Name] {
		return s.errorResponse(request.ID, models.MCPErrorCodeMethodNotFound, fmt.Sprintf("Tool '%s' is not enabled", toolCall.Name))
	}

	// Track active tool execution
	s.trackToolExecution(toolCall.Name, true)
	defer s.trackToolExecution(toolCall.Name, false)

	// Execute tool with timeout
	toolCtx, cancel := context.WithTimeout(ctx, s.config.RequestTimeout)
	defer cancel()

	result, err := s.executeTool(toolCtx, toolCall.Name, toolCall.Arguments)
	if err != nil {
		if mcpErr, ok := err.(*models.MCPError); ok {
			return &models.MCPResponse{
				JSONRPC: "2.0",
				ID:      request.ID,
				Error:   mcpErr,
			}
		}

		return s.errorResponse(request.ID, models.MCPErrorCodeInternalError, err.Error())
	}

	// Format result as MCP tool result
	toolResult := models.MCPToolResult{
		Content: []models.MCPContent{
			{
				Type: "text",
				Text: result,
			},
		},
	}

	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  toolResult,
	}
}

// executeTool executes the specified tool with given arguments
func (s *Server) executeTool(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	s.logger.Info("Executing tool",
		slog.String("tool", toolName),
		slog.Any("arguments", arguments),
	)

	switch toolName {
	case models.ToolGetTranscript:
		return s.executeGetTranscript(ctx, arguments)
	case models.ToolGetMultipleTranscripts:
		return s.executeGetMultipleTranscripts(ctx, arguments)
	case models.ToolTranslateTranscript:
		return s.executeTranslateTranscript(ctx, arguments)
	case models.ToolFormatTranscript:
		return s.executeFormatTranscript(ctx, arguments)
	case models.ToolListLanguages:
		return s.executeListLanguages(ctx, arguments)
	default:
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeMethodNotFound,
			Message: fmt.Sprintf("Unknown tool: %s", toolName),
		}
	}
}

// executeGetTranscript executes the get_transcript tool
func (s *Server) executeGetTranscript(ctx context.Context, arguments map[string]any) (string, error) {
	var params models.GetTranscriptParams

	if err := s.mapToStruct(arguments, &params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Invalid parameters: %v", err),
		}
	}

	if err := s.validator.Struct(params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
	}

	// Execute the tool
	result, err := s.youtube.GetTranscript(
		ctx,
		params.VideoIdentifier,
		params.Languages,
		params.PreserveFormatting,
	)
	if err != nil {
		// If it's already an MCP error, return it
		if mcpErr, ok := err.(*models.TranscriptError); ok {
			return "", &models.MCPError{
				Code:    models.MCPErrorCodeServerError,
				Message: mcpErr.Message,
				Data: map[string]any{
					"type":        mcpErr.Type,
					"video_id":    mcpErr.VideoID,
					"suggestions": mcpErr.Suggestions,
				},
			}
		}
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: err.Error(),
		}
	}

	// Optionally filter response based on parameters
	if !params.IncludeMetadata {
		result.Metadata = models.TranscriptMetadata{
			ExtractionTimestamp: result.Metadata.ExtractionTimestamp,
			Source:              result.Metadata.Source,
		}
	}

	if !params.IncludeTimestamps {
		// Remove timestamps from segments
		for i := range result.Transcript {
			result.Transcript[i].Start = 0
			result.Transcript[i].Duration = 0
			result.Transcript[i].End = 0
		}
	}

	// Convert to JSON string
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: fmt.Sprintf("Failed to serialize result: %v", err),
		}
	}

	return string(jsonBytes), nil
}

// executeGetMultipleTranscripts executes the get_multiple_transcripts tool
func (s *Server) executeGetMultipleTranscripts(ctx context.Context, arguments map[string]any) (string, error) {
	var params models.GetMultipleTranscriptsParams

	if err := s.mapToStruct(arguments, &params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Invalid parameters: %v", err),
		}
	}

	if err := s.validator.Struct(params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
	}

	// Execute the tool
	result, err := s.youtube.GetMultipleTranscripts(
		ctx,
		params.VideoIdentifiers,
		params.Languages,
		params.ContinueOnError,
	)
	if err != nil && !params.ContinueOnError {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: err.Error(),
		}
	}

	// Optionally filter metadata
	if !params.IncludeMetadata {
		for i := range result.Results {
			if result.Results[i].Transcript != nil {
				result.Results[i].Transcript.Metadata = models.TranscriptMetadata{
					ExtractionTimestamp: result.Results[i].Transcript.Metadata.ExtractionTimestamp,
					Source:              result.Results[i].Transcript.Metadata.Source,
				}
			}
		}
	}

	// Convert to JSON string
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: fmt.Sprintf("Failed to serialize result: %v", err),
		}
	}

	return string(jsonBytes), nil
}

// executeTranslateTranscript executes the translate_transcript tool
func (s *Server) executeTranslateTranscript(ctx context.Context, arguments map[string]any) (string, error) {
	var params models.TranslateTranscriptParams

	if err := s.mapToStruct(arguments, &params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Invalid parameters: %v", err),
		}
	}

	if err := s.validator.Struct(params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
	}

	// Execute the tool
	result, err := s.youtube.TranslateTranscript(
		ctx,
		params.VideoIdentifier,
		params.TargetLanguage,
		params.SourceLanguage,
	)
	if err != nil {
		if transcriptErr, ok := err.(*models.TranscriptError); ok {
			return "", &models.MCPError{
				Code:    models.MCPErrorCodeServerError,
				Message: transcriptErr.Message,
				Data: map[string]any{
					"type":     transcriptErr.Type,
					"video_id": transcriptErr.VideoID,
				},
			}
		}
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: err.Error(),
		}
	}

	// Optionally remove timestamps
	if !params.PreserveTimestamps {
		for i := range result.Transcript {
			result.Transcript[i].Start = 0
			result.Transcript[i].Duration = 0
			result.Transcript[i].End = 0
		}
	}

	// Convert to JSON string
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: fmt.Sprintf("Failed to serialize result: %v", err),
		}
	}

	return string(jsonBytes), nil
}

// executeFormatTranscript executes the format_transcript tool
func (s *Server) executeFormatTranscript(ctx context.Context, arguments map[string]any) (string, error) {
	var params models.FormatTranscriptParams

	if err := s.mapToStruct(arguments, &params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Invalid parameters: %v", err),
		}
	}

	if err := s.validator.Struct(params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
	}

	// Set default format type if not specified
	if params.FormatType == "" {
		params.FormatType = models.FormatTypePlainText
	}

	// Execute the tool
	result, err := s.youtube.FormatTranscript(
		ctx,
		params.VideoIdentifier,
		params.FormatType,
		params.IncludeTimestamps,
	)
	if err != nil {
		if transcriptErr, ok := err.(*models.TranscriptError); ok {
			return "", &models.MCPError{
				Code:    models.MCPErrorCodeServerError,
				Message: transcriptErr.Message,
				Data: map[string]any{
					"type":     transcriptErr.Type,
					"video_id": transcriptErr.VideoID,
				},
			}
		}
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: err.Error(),
		}
	}

	// For certain format types, return the formatted text directly
	if params.FormatType == models.FormatTypeSRT ||
		params.FormatType == models.FormatTypeVTT ||
		params.FormatType == models.FormatTypePlainText {
		return result.FormattedText, nil
	}

	// For other formats, return structured response
	response := map[string]any{
		"video_id":       result.VideoID,
		"title":          result.Title,
		"language":       result.Language,
		"format_type":    params.FormatType,
		"formatted_text": result.FormattedText,
		"word_count":     result.WordCount,
		"char_count":     result.CharCount,
		"duration":       result.DurationSeconds,
	}

	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: fmt.Sprintf("Failed to serialize result: %v", err),
		}
	}

	return string(jsonBytes), nil
}

// executeListLanguages executes the list_available_languages tool
func (s *Server) executeListLanguages(ctx context.Context, arguments map[string]any) (string, error) {
	var params models.ListLanguagesParams

	if err := s.mapToStruct(arguments, &params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Invalid parameters: %v", err),
		}
	}

	if err := s.validator.Struct(params); err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInvalidParams,
			Message: fmt.Sprintf("Validation error: %v", err),
		}
	}

	// Execute the tool
	result, err := s.youtube.ListAvailableLanguages(ctx, params.VideoIdentifier)
	if err != nil {
		if transcriptErr, ok := err.(*models.TranscriptError); ok {
			return "", &models.MCPError{
				Code:    models.MCPErrorCodeServerError,
				Message: transcriptErr.Message,
				Data: map[string]any{
					"type":     transcriptErr.Type,
					"video_id": transcriptErr.VideoID,
				},
			}
		}
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: err.Error(),
		}
	}

	// Filter out auto-generated if requested
	if !params.IncludeAuto {
		filtered := make([]models.LanguageInfo, 0)
		for _, lang := range result.Languages {
			if lang.Type != models.TranscriptTypeAuto {
				filtered = append(filtered, lang)
			}
		}
		result.Languages = filtered
	}

	// Convert to JSON string
	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", &models.MCPError{
			Code:    models.MCPErrorCodeInternalError,
			Message: fmt.Sprintf("Failed to serialize result: %v", err),
		}
	}

	return string(jsonBytes), nil
}

// getAvailableTools returns the list of available MCP tools
func (s *Server) getAvailableTools() []models.MCPTool {
	tools := []models.MCPTool{}

	if s.config.Tools[models.ToolGetTranscript] {
		tools = append(tools, models.MCPTool{
			Name:        models.ToolGetTranscript,
			Description: "Get transcript for a YouTube video in specified languages",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"video_identifier": map[string]any{
						"type":        "string",
						"description": "YouTube video URL, video ID, or watch URL",
					},
					"languages": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"description": "Preferred language codes (e.g., ['en', 'ja']). If not specified, uses default languages.",
					},
					"preserve_formatting": map[string]any{
						"type":        "boolean",
						"description": "Whether to preserve original formatting with timestamps",
						"default":     false,
					},
					"include_metadata": map[string]any{
						"type":        "boolean",
						"description": "Whether to include video metadata (channel, views, etc.)",
						"default":     true,
					},
					"include_timestamps": map[string]any{
						"type":        "boolean",
						"description": "Whether to include timestamp information in segments",
						"default":     true,
					},
				},
				"required": []string{"video_identifier"},
			},
		})
	}

	if s.config.Tools[models.ToolGetMultipleTranscripts] {
		tools = append(tools, models.MCPTool{
			Name:        models.ToolGetMultipleTranscripts,
			Description: "Get transcripts for multiple YouTube videos",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"video_identifiers": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"description": "List of YouTube video URLs or IDs (max 50)",
						"minItems":    1,
						"maxItems":    50,
					},
					"languages": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
						"description": "Preferred language codes",
					},
					"continue_on_error": map[string]any{
						"type":        "boolean",
						"description": "Continue processing other videos if one fails",
						"default":     true,
					},
					"include_metadata": map[string]any{
						"type":        "boolean",
						"description": "Whether to include video metadata",
						"default":     false,
					},
					"parallel": map[string]any{
						"type":        "boolean",
						"description": "Process videos in parallel for faster results",
						"default":     true,
					},
				},
				"required": []string{"video_identifiers"},
			},
		})
	}

	if s.config.Tools[models.ToolTranslateTranscript] {
		tools = append(tools, models.MCPTool{
			Name:        models.ToolTranslateTranscript,
			Description: "Translate a video transcript to a target language",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"video_identifier": map[string]any{
						"type":        "string",
						"description": "YouTube video URL or ID",
					},
					"target_language": map[string]any{
						"type":        "string",
						"description": "Target language code (e.g., 'ja', 'es', 'fr')",
						"minLength":   2,
						"maxLength":   5,
					},
					"source_language": map[string]any{
						"type":        "string",
						"description": "Source language code (optional, auto-detected if not specified)",
					},
					"preserve_timestamps": map[string]any{
						"type":        "boolean",
						"description": "Whether to preserve timestamp information",
						"default":     true,
					},
				},
				"required": []string{"video_identifier", "target_language"},
			},
		})
	}

	if s.config.Tools[models.ToolFormatTranscript] {
		tools = append(tools, models.MCPTool{
			Name:        models.ToolFormatTranscript,
			Description: "Format a transcript in various styles",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"video_identifier": map[string]any{
						"type":        "string",
						"description": "YouTube video URL or ID",
					},
					"format_type": map[string]any{
						"type":        "string",
						"enum":        []string{"plain_text", "paragraphs", "sentences", "srt", "vtt", "json"},
						"description": "Output format type",
						"default":     "plain_text",
					},
					"include_timestamps": map[string]any{
						"type":        "boolean",
						"description": "Include timestamps in the formatted output",
						"default":     false,
					},
					"timestamp_format": map[string]any{
						"type":        "string",
						"enum":        []string{"seconds", "hms", "ms"},
						"description": "Timestamp format (seconds, HH:MM:SS, or HH:MM:SS,mmm)",
						"default":     "seconds",
					},
					"max_line_length": map[string]any{
						"type":        "integer",
						"description": "Maximum characters per line (for subtitle formats)",
						"default":     80,
						"minimum":     20,
						"maximum":     200,
					},
				},
				"required": []string{"video_identifier"},
			},
		})
	}

	if s.config.Tools[models.ToolListLanguages] {
		tools = append(tools, models.MCPTool{
			Name:        models.ToolListLanguages,
			Description: "List available transcript languages for a video",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"video_identifier": map[string]any{
						"type":        "string",
						"description": "YouTube video URL or ID",
					},
					"include_auto": map[string]any{
						"type":        "boolean",
						"description": "Include auto-generated transcripts in the list",
						"default":     true,
					},
				},
				"required": []string{"video_identifier"},
			},
		})
	}

	return tools
}

// Additional handler methods for optional MCP features

func (s *Server) handleListResources(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	if !s.config.EnableResources {
		return s.errorResponse(request.ID, models.MCPErrorCodeMethodNotFound, "Resources not enabled")
	}

	// Return empty resource list for now
	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: map[string]any{
			"resources": []any{},
		},
	}
}

func (s *Server) handleReadResource(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	if !s.config.EnableResources {
		return s.errorResponse(request.ID, models.MCPErrorCodeMethodNotFound, "Resources not enabled")
	}

	return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, "No resources available")
}

func (s *Server) handleListPrompts(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	if !s.config.EnablePrompts {
		return s.errorResponse(request.ID, models.MCPErrorCodeMethodNotFound, "Prompts not enabled")
	}

	// Return empty prompt list for now
	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result: map[string]any{
			"prompts": []any{},
		},
	}
}

func (s *Server) handleGetPrompt(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	if !s.config.EnablePrompts {
		return s.errorResponse(request.ID, models.MCPErrorCodeMethodNotFound, "Prompts not enabled")
	}

	return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, "No prompts available")
}

func (s *Server) handleSetLoggingLevel(ctx context.Context, request models.MCPRequest) *models.MCPResponse {
	if !s.config.EnableLogging {
		return s.errorResponse(request.ID, models.MCPErrorCodeMethodNotFound, "Logging control not enabled")
	}

	params, ok := request.Params.(map[string]any)
	if !ok {
		return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, "Invalid parameters")
	}

	level, ok := params["level"].(string)
	if !ok {
		return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, "Level parameter required")
	}

	// Validate and set logging level
	validLevels := []string{"debug", "info", "warn", "error"}
	isValid := false
	for _, validLevel := range validLevels {
		if strings.ToLower(level) == validLevel {
			isValid = true
			break
		}
	}

	if !isValid {
		return s.errorResponse(request.ID, models.MCPErrorCodeInvalidParams, fmt.Sprintf("Invalid logging level: %s", level))
	}

	s.logger.Info("Logging level changed", slog.String("new_level", level))

	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  map[string]any{"success": true},
	}
}

// Helper methods

func (s *Server) mapToStruct(input map[string]any, output any) error {
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, output)
}

func (s *Server) errorResponse(id any, code int, message string) *models.MCPResponse {
	return &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &models.MCPError{
			Code:    code,
			Message: message,
		},
	}
}

func (s *Server) sendError(w http.ResponseWriter, id any, code int, message string, data any) {
	response := &models.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &models.MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // MCP uses 200 OK even for errors
	json.NewEncoder(w).Encode(response)
}

func (s *Server) incrementRequestCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestCount++
}

func (s *Server) trackToolExecution(toolName string, start bool) {
	if start {
		s.activeTools.Store(toolName, time.Now())
	} else {
		s.activeTools.Delete(toolName)
	}
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeToolCount := 0
	s.activeTools.Range(func(key, value any) bool {
		activeToolCount++
		return true
	})

	return map[string]any{
		"request_count":    s.requestCount,
		"active_tools":     activeToolCount,
		"enabled_tools":    s.getEnabledToolCount(),
		"server_version":   s.config.ServerVersion,
		"protocol_version": s.config.Version,
	}
}

func (s *Server) getEnabledToolCount() int {
	count := 0
	for _, enabled := range s.config.Tools {
		if enabled {
			count++
		}
	}
	return count
}
