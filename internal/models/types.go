package models

import (
	"time"
)

// TranscriptSegment represents a single segment of transcript with timing information
type TranscriptSegment struct {
	Text     string  `json:"text"`
	Start    float64 `json:"start"`
	Duration float64 `json:"duration"`
	End      float64 `json:"end,omitempty"`
}

// TranscriptResponse represents the complete transcript response with metadata
type TranscriptResponse struct {
	VideoID         string               `json:"video_id"`
	Title           string               `json:"title,omitempty"`
	Description     string               `json:"description,omitempty"`
	Language        string               `json:"language"`
	TranscriptType  string               `json:"transcript_type"` // "manual", "generated", or "auto"
	Transcript      []TranscriptSegment  `json:"transcript"`
	FormattedText   string               `json:"formatted_text,omitempty"`
	WordCount       int                  `json:"word_count"`
	CharCount       int                  `json:"char_count"`
	DurationSeconds float64              `json:"duration_seconds"`
	Metadata        TranscriptMetadata   `json:"metadata"`
}

// TranscriptMetadata contains detailed metadata about the transcript
type TranscriptMetadata struct {
	ExtractionTimestamp time.Time `json:"extraction_timestamp"`
	LanguageDetected    string    `json:"language_detected,omitempty"`
	Confidence          float64   `json:"confidence,omitempty"`
	Source              string    `json:"source"`
	ChannelID           string    `json:"channel_id,omitempty"`
	ChannelName         string    `json:"channel_name,omitempty"`
	PublishedAt         string    `json:"published_at,omitempty"`
	ViewCount           int64     `json:"view_count,omitempty"`
	LikeCount           int64     `json:"like_count,omitempty"`
	CommentCount        int64     `json:"comment_count,omitempty"`
}

// MultipleTranscriptResponse represents response for multiple videos
type MultipleTranscriptResponse struct {
	Results    []TranscriptResult `json:"results"`
	Errors     []TranscriptError  `json:"errors,omitempty"`
	TotalCount int                `json:"total_count"`
	SuccessCount int              `json:"success_count"`
	ErrorCount int                `json:"error_count"`
}

// TranscriptResult represents a single video result in batch processing
type TranscriptResult struct {
	VideoID    string              `json:"video_id"`
	Success    bool                `json:"success"`
	Transcript *TranscriptResponse `json:"transcript,omitempty"`
	Error      *TranscriptError    `json:"error,omitempty"`
	ProcessingTime time.Duration     `json:"processing_time,omitempty"`
}

// TranscriptError represents an error in transcript processing
type TranscriptError struct {
	Type        string   `json:"type"`
	Message     string   `json:"message"`
	VideoID     string   `json:"video_id,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
	RetryAfter  int      `json:"retry_after,omitempty"` // seconds
}

// Error implements the error interface
func (e *TranscriptError) Error() string {
	return e.Message
}

// LanguageInfo represents information about available languages
type LanguageInfo struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	NativeName   string `json:"native_name,omitempty"`
	Type         string `json:"type"` // "manual", "generated", or "auto"
	IsTranslated bool   `json:"is_translated"`
	IsDefault    bool   `json:"is_default"`
}

// AvailableLanguagesResponse represents response for available languages
type AvailableLanguagesResponse struct {
	VideoID           string         `json:"video_id"`
	Languages         []LanguageInfo `json:"languages"`
	DefaultLanguage   string         `json:"default_language"`
	TranslatableCount int            `json:"translatable_count"`
}

// MCPRequest represents a Model Context Protocol request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a Model Context Protocol response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	return e.Message
}

// GetTranscriptParams represents parameters for get_transcript tool
type GetTranscriptParams struct {
	VideoIdentifier     string   `json:"video_identifier" validate:"required"`
	Languages           []string `json:"languages,omitempty"`
	PreserveFormatting  bool     `json:"preserve_formatting,omitempty"`
	IncludeMetadata     bool     `json:"include_metadata,omitempty"`
	IncludeTimestamps   bool     `json:"include_timestamps,omitempty"`
}

// GetMultipleTranscriptsParams represents parameters for batch processing
type GetMultipleTranscriptsParams struct {
	VideoIdentifiers []string `json:"video_identifiers" validate:"required,min=1,max=50"`
	Languages        []string `json:"languages,omitempty"`
	ContinueOnError  bool     `json:"continue_on_error,omitempty"`
	IncludeMetadata  bool     `json:"include_metadata,omitempty"`
	Parallel         bool     `json:"parallel,omitempty"`
}

// TranslateTranscriptParams represents parameters for translation
type TranslateTranscriptParams struct {
	VideoIdentifier string `json:"video_identifier" validate:"required"`
	TargetLanguage  string `json:"target_language" validate:"required,len=2"`
	SourceLanguage  string `json:"source_language,omitempty"`
	PreserveTimestamps bool `json:"preserve_timestamps,omitempty"`
}

// FormatTranscriptParams represents parameters for formatting
type FormatTranscriptParams struct {
	VideoIdentifier    string `json:"video_identifier" validate:"required"`
	FormatType         string `json:"format_type,omitempty"` // "plain_text", "paragraphs", "sentences", "srt", "vtt", "json"
	IncludeTimestamps  bool   `json:"include_timestamps,omitempty"`
	TimestampFormat    string `json:"timestamp_format,omitempty"` // "seconds", "hms", "ms"
	MaxLineLength      int    `json:"max_line_length,omitempty"`
}

// ListLanguagesParams represents parameters for listing languages
type ListLanguagesParams struct {
	VideoIdentifier string `json:"video_identifier" validate:"required"`
	IncludeAuto     bool   `json:"include_auto,omitempty"`
}

// MCPTool represents an MCP tool definition
type MCPTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// MCPToolsListResponse represents the response to list_tools
type MCPToolsListResponse struct {
	Tools []MCPTool `json:"tools"`
}

// MCPInitializeResponse represents the response to initialize
type MCPInitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      MCPServerInfo          `json:"serverInfo"`
	Capabilities    MCPServerCapabilities  `json:"capabilities"`
}

// MCPServerInfo contains server information
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCPServerCapabilities describes server capabilities
type MCPServerCapabilities struct {
	Tools      MCPToolsCapability      `json:"tools,omitempty"`
	Resources  MCPResourcesCapability  `json:"resources,omitempty"`
	Prompts    MCPPromptsCapability    `json:"prompts,omitempty"`
}

// MCPToolsCapability describes tools capability
type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// MCPResourcesCapability describes resources capability
type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

// MCPPromptsCapability describes prompts capability
type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// MCPToolCallParams represents parameters for tool call
type MCPToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPToolResult represents the result of a tool call
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
}

// MCPContent represents content in MCP format
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// VideoInfo represents basic video information
type VideoInfo struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	Duration        string    `json:"duration,omitempty"`
	ChannelID       string    `json:"channel_id,omitempty"`
	ChannelName     string    `json:"channel_name,omitempty"`
	ThumbnailURL    string    `json:"thumbnail_url,omitempty"`
	UploadDate      time.Time `json:"upload_date,omitempty"`
	ViewCount       int64     `json:"view_count,omitempty"`
	LikeCount       int64     `json:"like_count,omitempty"`
	DislikeCount    int64     `json:"dislike_count,omitempty"`
	CommentCount    int64     `json:"comment_count,omitempty"`
	IsLiveContent   bool      `json:"is_live_content,omitempty"`
	IsPrivate       bool      `json:"is_private,omitempty"`
	IsDeleted       bool      `json:"is_deleted,omitempty"`
}

// CacheEntry represents a cached transcript entry
type CacheEntry struct {
	Key       string              `json:"key"`
	Data      interface{}         `json:"data"`
	Timestamp time.Time           `json:"timestamp"`
	TTL       time.Duration       `json:"ttl"`
	HitCount  int                 `json:"hit_count"`
}

// ErrorType constants
const (
	ErrorTypeVideoUnavailable     = "VIDEO_UNAVAILABLE"
	ErrorTypeNoTranscriptFound    = "NO_TRANSCRIPT_FOUND"
	ErrorTypeTranscriptsDisabled  = "TRANSCRIPTS_DISABLED"
	ErrorTypeInvalidVideoID       = "INVALID_VIDEO_ID"
	ErrorTypeNetworkError         = "NETWORK_ERROR"
	ErrorTypeRateLimitExceeded    = "RATE_LIMIT_EXCEEDED"
	ErrorTypeLanguageNotAvailable = "LANGUAGE_NOT_AVAILABLE"
	ErrorTypeInternalError        = "INTERNAL_ERROR"
	ErrorTypeValidationError      = "VALIDATION_ERROR"
	ErrorTypeAuthenticationError  = "AUTHENTICATION_ERROR"
	ErrorTypeParsingError         = "PARSING_ERROR"
	ErrorTypeTimeout              = "TIMEOUT_ERROR"
	ErrorTypeCaptchaRequired      = "CAPTCHA_REQUIRED"
)

// MCP Method constants
const (
	MCPMethodListTools      = "tools/list"
	MCPMethodCallTool       = "tools/call"
	MCPMethodInitialize     = "initialize"
	MCPMethodListResources  = "resources/list"
	MCPMethodReadResource   = "resources/read"
	MCPMethodListPrompts    = "prompts/list"
	MCPMethodGetPrompt      = "prompts/get"
	MCPMethodSetLoggingLevel = "logging/setLevel"
)

// Tool name constants
const (
	ToolGetTranscript          = "get_transcript"
	ToolGetMultipleTranscripts = "get_multiple_transcripts"
	ToolTranslateTranscript    = "translate_transcript"
	ToolFormatTranscript       = "format_transcript"
	ToolListLanguages          = "list_available_languages"
)

// Format type constants
const (
	FormatTypePlainText  = "plain_text"
	FormatTypeParagraphs = "paragraphs"
	FormatTypeSentences  = "sentences"
	FormatTypeSRT        = "srt"
	FormatTypeVTT        = "vtt"
	FormatTypeJSON       = "json"
)

// Transcript type constants
const (
	TranscriptTypeManual    = "manual"
	TranscriptTypeGenerated = "generated"
	TranscriptTypeAuto      = "auto"
)

// Timestamp format constants
const (
	TimestampFormatSeconds = "seconds"
	TimestampFormatHMS     = "hms"      // HH:MM:SS
	TimestampFormatMS      = "ms"       // HH:MM:SS,mmm
)

// MCP Error codes
const (
	MCPErrorCodeParseError     = -32700
	MCPErrorCodeInvalidRequest = -32600
	MCPErrorCodeMethodNotFound = -32601
	MCPErrorCodeInvalidParams  = -32602
	MCPErrorCodeInternalError  = -32603
	MCPErrorCodeServerError    = -32000
)

// Cache key prefixes
const (
	CacheKeyPrefixTranscript = "transcript:"
	CacheKeyPrefixLanguages  = "languages:"
	CacheKeyPrefixVideoInfo  = "videoinfo:"
	CacheKeyPrefixError      = "error:"
)

// Default values
const (
	DefaultLanguage      = "en"
	DefaultFormatType    = FormatTypePlainText
	DefaultMaxLineLength = 80
	DefaultCacheTTL      = 24 * time.Hour
	DefaultErrorCacheTTL = 15 * time.Minute
	DefaultTimeout       = 30 * time.Second
	DefaultRetryAttempts = 3
	DefaultRetryDelay    = time.Second
)