package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GenerateRequest to Ollama API
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// GenerateResponse from Ollama API
type GenerateResponse struct {
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	Response  string    `json:"response"`
	Done      bool      `json:"done"`
}

type OllamaClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second, // AI can take time
		},
	}
}

// Generate sends a prompt to Ollama and returns the response
func (c *OllamaClient) Generate(model, prompt string) (string, error) {
	reqBody := GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return genResp.Response, nil
}

// CheckModel checks if the specified model is available in Ollama
func (c *OllamaClient) CheckModel(model string) error {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API error listing models: status %d", resp.StatusCode)
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to decode tags: %w", err)
	}

	for _, m := range tags.Models {
		if m.Name == model || m.Name == model+":latest" {
			return nil
		}
	}

	return fmt.Errorf("model %s not found in Ollama", model)
}
