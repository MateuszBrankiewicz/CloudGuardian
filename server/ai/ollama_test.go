package ai

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOllamaIntegration(t *testing.T) {
	baseURL := "http://localhost:11434"
	client := NewOllamaClient(baseURL)

	// Short timeout for checking if up
	checkClient := &http.Client{Timeout: 1 * time.Second}
	resp, err := checkClient.Get(baseURL + "/api/tags")
	if err != nil {
		t.Skip("Ollama is not running on localhost:11434, skipping integration test.")
		return
	}
	defer resp.Body.Close()

	// [AC 2] Wybór modelu
	err = client.CheckModel("not-existing-model-xyz")
	assert.Error(t, err, "Should fail for non-existing model")

	// [AC 4] TDD: Test wysyłający prompt "Hello"
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		t.Fatalf("failed to decode tags: %v", err)
	}

	if len(tags.Models) == 0 {
		t.Log("Ollama is running but has no models pulled. Try 'ollama pull llama3'")
		t.Skip("No models found in Ollama, cannot test Generate.")
		return
	}

	model := tags.Models[0].Name
	t.Logf("Testing with model: %s", model)

	response, err := client.Generate(model, "Say 'Hello' and nothing else.")
	if err != nil {
		t.Fatalf("Ollama Generate failed: %v", err)
	}
	assert.NoError(t, err)
	assert.NotEmpty(t, response)
	t.Logf("Ollama response: %s", response)
}
