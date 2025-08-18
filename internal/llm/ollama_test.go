package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewOllamaClient verifies client creation with correct parameters
func TestNewOllamaClient(t *testing.T) {
	endpoint := "http://localhost:11434"
	model := "codellama:7b"

	client := NewOllamaClient(endpoint, model)

	if client.endpoint != endpoint {
		t.Errorf("Expected endpoint %s, got %s", endpoint, client.endpoint)
	}

	if client.model != model {
		t.Errorf("Expected model %s, got %s", model, client.model)
	}

	if client.client == nil {
		t.Error("HTTP client should not be nil")
	}

	if client.client.Timeout != 5*time.Minute {
		t.Errorf("Expected timeout of 5 minutes, got %v", client.client.Timeout)
	}
}

// TestComplete tests the Complete method with various scenarios
func TestComplete(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse generateResponse
		serverStatus   int
		wantError      bool
		errorContains  string
	}{
		{
			name: "successful completion",
			serverResponse: generateResponse{
				Model:    "codellama:7b",
				Response: "generated code here",
				Done:     true,
			},
			serverStatus: http.StatusOK,
			wantError:    false,
		},
		{
			name: "incomplete generation",
			serverResponse: generateResponse{
				Model:    "codellama:7b",
				Response: "partial",
				Done:     false,
			},
			serverStatus:  http.StatusOK,
			wantError:     true,
			errorContains: "generation incomplete",
		},
		{
			name:          "server error",
			serverStatus:  http.StatusInternalServerError,
			wantError:     true,
			errorContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				if r.URL.Path != "/api/generate" {
					t.Errorf("Expected path /api/generate, got %s", r.URL.Path)
				}

				// Verify method
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}

				// Verify content type
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Send response
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			}))
			defer server.Close()

			// Create client with test server URL
			client := NewOllamaClient(server.URL, "codellama:7b")

			// Call Complete
			ctx := context.Background()
			response, err := client.Complete(ctx, "test prompt")

			// Check error
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if response != tt.serverResponse.Response {
					t.Errorf("Expected response '%s', got '%s'", tt.serverResponse.Response, response)
				}
			}
		})
	}
}

// TestHealthCheck tests the health check functionality
func TestHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		models        []string
		targetModel   string
		serverStatus  int
		wantError     bool
		errorContains string
	}{
		{
			name:         "model found",
			models:       []string{"codellama:7b", "llama2:13b"},
			targetModel:  "codellama:7b",
			serverStatus: http.StatusOK,
			wantError:    false,
		},
		{
			name:          "model not found",
			models:        []string{"llama2:13b"},
			targetModel:   "codellama:7b",
			serverStatus:  http.StatusOK,
			wantError:     true,
			errorContains: "model codellama:7b not found",
		},
		{
			name:          "server error",
			serverStatus:  http.StatusInternalServerError,
			wantError:     true,
			errorContains: "health check failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request path
				if r.URL.Path != "/api/tags" {
					t.Errorf("Expected path /api/tags, got %s", r.URL.Path)
				}

				// Send response
				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					// Build response with models
					response := tagsResponse{}
					for _, model := range tt.models {
						response.Models = append(response.Models, struct {
							Name       string    `json:"name"`
							ModifiedAt time.Time `json:"modified_at"`
							Size       int64     `json:"size"`
							Digest     string    `json:"digest"`
						}{
							Name:       model,
							ModifiedAt: time.Now(),
							Size:       1000000,
							Digest:     "abc123",
						})
					}
					json.NewEncoder(w).Encode(response)
				}
			}))
			defer server.Close()

			// Create client with test server URL
			client := NewOllamaClient(server.URL, tt.targetModel)

			// Call HealthCheck
			ctx := context.Background()
			err := client.HealthCheck(ctx)

			// Check error
			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestCompleteWithTimeout tests that context cancellation works properly
func TestCompleteWithTimeout(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(generateResponse{
			Response: "delayed response",
			Done:     true,
		})
	}))
	defer server.Close()

	// Create client with test server
	client := NewOllamaClient(server.URL, "codellama:7b")

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Call should timeout
	_, err := client.Complete(ctx, "test prompt")
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline error, got: %v", err)
	}
}

// TestRequestPayload verifies the request payload structure
func TestRequestPayload(t *testing.T) {
	var capturedPayload generateRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture and decode the request payload
		json.NewDecoder(r.Body).Decode(&capturedPayload)

		// Send successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(generateResponse{
			Response: "test",
			Done:     true,
		})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "testmodel")
	ctx := context.Background()

	prompt := "generate some code"
	client.Complete(ctx, prompt)

	// Verify payload
	if capturedPayload.Model != "testmodel" {
		t.Errorf("Expected model 'testmodel', got '%s'", capturedPayload.Model)
	}

	if capturedPayload.Prompt != prompt {
		t.Errorf("Expected prompt '%s', got '%s'", prompt, capturedPayload.Prompt)
	}

	if capturedPayload.Stream != false {
		t.Error("Stream should be false")
	}

	// Check options
	if capturedPayload.Options["temperature"] != 0.2 {
		t.Errorf("Expected temperature 0.2, got %v", capturedPayload.Options["temperature"])
	}

	if capturedPayload.Options["top_p"] != 0.9 {
		t.Errorf("Expected top_p 0.9, got %v", capturedPayload.Options["top_p"])
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && s[:len(s)] != "" &&
		(s == substr || len(s) > len(substr) && (containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
