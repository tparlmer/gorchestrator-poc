package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaClient provides a simple HTTP client for interacting with Ollama API
// It handles code generation requests without streaming for simplicity
type OllamaClient struct {
	endpoint string       // Base URL for Ollama API (e.g., "http://localhost:11434")
	model    string       // Model to use for generation (e.g., "codellama:7b")
	client   *http.Client // HTTP client with timeout configuration
}

// NewOllamaClient creates a new Ollama API client
func NewOllamaClient(endpoint, model string) *OllamaClient {
	return &OllamaClient{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 5 * time.Minute, // Generous timeout for code generation
		},
	}
}

// generateRequest represents the request payload for Ollama's generate endpoint
type generateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options"`
}

// generateResponse represents the response from Ollama's generate endpoint
type generateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
}

// Complete sends a prompt to Ollama and returns the generated response
// Uses non-streaming mode for simplicity and waits for complete response
func (o *OllamaClient) Complete(ctx context.Context, prompt string) (string, error) {
	// Prepare request payload with low temperature for consistent code generation
	payload := generateRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false, // Wait for complete response
		Options: map[string]interface{}{
			"temperature": 0.2,  // Low temperature for deterministic code generation
			"top_p":       0.9,  // Slightly limit token pool for quality
			"num_predict": 4096, // Max tokens to generate
		},
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request with context for cancellation support
	url := fmt.Sprintf("%s/api/generate", o.endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request to Ollama
	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	// Decode response
	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Verify generation completed
	if !result.Done {
		return "", fmt.Errorf("generation incomplete")
	}

	return result.Response, nil
}

// tagsResponse represents the response from Ollama's tags endpoint
type tagsResponse struct {
	Models []struct {
		Name       string    `json:"name"`
		ModifiedAt time.Time `json:"modified_at"`
		Size       int64     `json:"size"`
		Digest     string    `json:"digest"`
	} `json:"models"`
}

// HealthCheck verifies Ollama is running and the specified model is available
func (o *OllamaClient) HealthCheck(ctx context.Context) error {
	// Check if Ollama is running by fetching available models
	url := fmt.Sprintf("%s/api/tags", o.endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Use a shorter timeout for health check
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama not reachable at %s: %w", o.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama health check failed with status %d", resp.StatusCode)
	}

	// Parse available models
	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to decode models list: %w", err)
	}

	// Check if our model is available
	modelFound := false
	for _, model := range tags.Models {
		if model.Name == o.model {
			modelFound = true
			break
		}
	}

	if !modelFound {
		return fmt.Errorf("model %s not found in Ollama", o.model)
	}

	return nil
}
