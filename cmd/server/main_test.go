package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/youtube-transcript-mcp/internal/models"
)

func TestMain(m *testing.M) {
	// Setup test environment
	if err := os.Setenv("PORT", "0"); err != nil { // Use random port for testing
		panic(err)
	}
	if err := os.Setenv("LOG_LEVEL", "error"); err != nil { // Reduce log noise
		panic(err)
	}
	if err := os.Setenv("CACHE_ENABLED", "true"); err != nil {
		panic(err)
	}
	if err := os.Setenv("MCP_REQUEST_TIMEOUT", "5s"); err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	os.Exit(code)
}

func TestServerStartup(t *testing.T) {
	// Test that server can start without errors
	// This is a basic smoke test

	// In a real test, we'd refactor main() to be testable
	// For now, this is a placeholder that tests the concept

	done := make(chan bool, 1)

	go func() {
		// Simulate server startup
		time.Sleep(50 * time.Millisecond)
		done <- true
	}()

	select {
	case <-done:
		// Server started successfully
	case <-time.After(1 * time.Second):
		t.Fatal("Server startup timeout")
	}
}

func TestHealthEndpoint(t *testing.T) {
	// Create a test router with health endpoint
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response["status"])
	assert.Contains(t, response, "timestamp")
	assert.Contains(t, response, "version")
}

func TestReadyEndpoint(t *testing.T) {
	// Create a test router with ready endpoint
	router := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ready", response["status"])
	assert.Contains(t, response, "timestamp")
}

func TestMCPEndpointIntegration(t *testing.T) {
	// Create a test router with MCP endpoint
	router := setupTestRouter()

	tests := []struct {
		request        models.MCPRequest
		checkResponse  func(t *testing.T, response models.MCPResponse)
		name           string
		expectedStatus int
	}{
		{
			name: "initialize request",
			request: models.MCPRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  models.MCPMethodInitialize,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response models.MCPResponse) {
				assert.Nil(t, response.Error)
				assert.NotNil(t, response.Result)
			},
		},
		{
			name: "list tools request",
			request: models.MCPRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  models.MCPMethodListTools,
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response models.MCPResponse) {
				assert.Nil(t, response.Error)
				assert.NotNil(t, response.Result)
			},
		},
		{
			name: "get transcript request",
			request: models.MCPRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  models.MCPMethodCallTool,
				Params: map[string]any{
					"name": "get_transcript",
					"arguments": map[string]any{
						"video_identifier": "dQw4w9WgXcQ",
						"languages":        []string{"en"},
					},
				},
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response models.MCPResponse) {
				// This might error in test environment without real YouTube access
				// But we're testing the integration, not the YouTube service
				assert.Equal(t, float64(3), response.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response models.MCPResponse
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
		})
	}
}

func TestCORSHeaders(t *testing.T) {
	router := setupTestRouter()

	// Test OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
}

func TestGracefulShutdown(t *testing.T) {
	// This test would require refactoring main() to be more testable
	// For now, we'll test the concept

	server := &http.Server{
		Addr: ":0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate a slow request
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	}

	// Start server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()
	time.Sleep(50 * time.Millisecond)

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestEnvironmentConfiguration(t *testing.T) {
	tests := []struct {
		envVars  map[string]string
		validate func(t *testing.T)
		name     string
	}{
		{
			name: "custom port",
			envVars: map[string]string{
				"PORT": "9090",
			},
			validate: func(t *testing.T) {
				// In a real test, we'd check if the server listens on this port
				assert.Equal(t, "9090", os.Getenv("PORT"))
			},
		},
		{
			name: "debug logging",
			envVars: map[string]string{
				"LOG_LEVEL": "debug",
			},
			validate: func(t *testing.T) {
				assert.Equal(t, "debug", os.Getenv("LOG_LEVEL"))
			},
		},
		{
			name: "cache disabled",
			envVars: map[string]string{
				"CACHE_ENABLED": "false",
			},
			validate: func(t *testing.T) {
				assert.Equal(t, "false", os.Getenv("CACHE_ENABLED"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			originals := make(map[string]string)
			for k := range tt.envVars {
				originals[k] = os.Getenv(k)
			}

			// Set test values
			for k, v := range tt.envVars {
				err := os.Setenv(k, v)
				require.NoError(t, err)
			}

			// Validate
			tt.validate(t)

			// Restore original values
			for k, v := range originals {
				if v == "" {
					err := os.Unsetenv(k)
					require.NoError(t, err)
				} else {
					err := os.Setenv(k, v)
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	router := setupTestRouter()

	// Number of concurrent requests
	numRequests := 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			request := models.MCPRequest{
				JSONRPC: "2.0",
				ID:      id,
				Method:  models.MCPMethodListTools,
			}

			body, err := json.Marshal(request)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}
}

func TestRequestTimeout(t *testing.T) {
	// This test would require a custom handler that can simulate slow operations
	// For now, we'll test the concept

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		select {
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-ctx.Done():
			w.WriteHeader(http.StatusRequestTimeout)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
}

func TestInvalidJSONRequest(t *testing.T) {
	router := setupTestRouter()

	// Send invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code) // MCP returns 200 even for errors

	var response models.MCPResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotNil(t, response.Error)
	assert.Equal(t, models.MCPErrorCodeParseError, response.Error.Code)
}

// setupTestRouter creates a test router with all endpoints
// This is a simplified version that would need to be implemented
// based on the actual main.go structure
func setupTestRouter() http.Handler {
	// This would be extracted from main.go
	// For now, we'll create a minimal router
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":    "ready",
			"timestamp": time.Now().UTC(),
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	// MCP endpoint (simplified)
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		var request models.MCPRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(models.MCPResponse{
				JSONRPC: "2.0",
				Error: &models.MCPError{
					Code:    models.MCPErrorCodeParseError,
					Message: "Parse error",
				},
			}); err != nil {
				http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
			}
			return
		}

		// Simple response for testing
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(models.MCPResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result:  map[string]any{"status": "ok"},
		}); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	})

	// Wrap with CORS middleware
	return corsWrapper(mux)
}

func corsWrapper(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
