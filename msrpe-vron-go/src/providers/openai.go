package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
)

// --- OpenAI Request/Response Structs ---

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponseFormat enforces JSON schema via OpenAI's native structured output.
type openAIResponseFormat struct {
	Type       string                 `json:"type"`
	JSONSchema map[string]interface{} `json:"json_schema,omitempty"`
}

type openAIRequest struct {
	Model          string               `json:"model"`
	Messages       []openAIMessage      `json:"messages"`
	Temperature    float32              `json:"temperature,omitempty"`
	ResponseFormat openAIResponseFormat `json:"response_format"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// --- OpenAIInferenceProvider ---

type OpenAIInferenceProvider struct {
	BaseURL     string
	APIKey      string
	Model       string
	ValTimeout  time.Duration
	ExecTimeout time.Duration
	Temperature float64
}

func NewOpenAIInferenceProvider(baseURL, apiKey, model string, valTimeout, execTimeout time.Duration, temp float64) *OpenAIInferenceProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIInferenceProvider{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       model,
		ValTimeout:  valTimeout,
		ExecTimeout: execTimeout,
		Temperature: temp,
	}
}

func (p *OpenAIInferenceProvider) Validate(ctx context.Context) error {
	url := fmt.Sprintf("%s/models", strings.TrimSuffix(p.BaseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	client := &http.Client{Timeout: p.ValTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openai validation failed with status %d", resp.StatusCode)
	}
	return nil
}

// GenerateStructured calls OpenAI with native json_schema response_format enforcement.
func (p *OpenAIInferenceProvider) GenerateStructured(ctx context.Context, systemPrompt string, vronCtx vron.VRonContext) (*vron.VRonOutput, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("VRON_LLM_API_KEY is not set")
	}

	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.BaseURL, "/"))
	userPrompt := buildUserPrompt(vronCtx)

	reqBody := openAIRequest{
		Model:       p.Model,
		Temperature: float32(p.Temperature),
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		// Native structured output: forces the model to output valid VRonOutput JSON
		ResponseFormat: openAIResponseFormat{
			Type: "json_schema",
			JSONSchema: map[string]interface{}{
				"name":   "VRonOutput",
				"strict": true,
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"response":        map[string]interface{}{"type": "string"},
						"confidence":      map[string]interface{}{"type": "integer"},
						"need_memory":     map[string]interface{}{"type": "boolean"},
						"memory_query":    map[string]interface{}{"type": "string"},
						"need_test":       map[string]interface{}{"type": "boolean"},
						"test_query":      map[string]interface{}{"type": "string"},
						"facts":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"result":          map[string]interface{}{"type": "string"},
						"reasoning":       map[string]interface{}{"type": "string"},
						"observation":     map[string]interface{}{"type": "string"},
						"serotonin_delta": map[string]interface{}{"type": "integer"},
						"tasks":           map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"message":         map[string]interface{}{"type": "string"},
						"query":           map[string]interface{}{"type": "string"},
						"action":          map[string]interface{}{"type": "string"},
					},
					"additionalProperties": true,
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := &http.Client{Timeout: p.ExecTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr openAIResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("openai API returned non-200 status: %d", resp.StatusCode)
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from OpenAI")
	}

	rawJSON := oaiResp.Choices[0].Message.Content
	utils.LogDebug("LLM API Call (OpenAI) | Sent: %d chars | Received: %d chars", len(jsonData), len(rawJSON))

	var output vron.VRonOutput
	if err := json.Unmarshal([]byte(rawJSON), &output); err != nil {
		return nil, fmt.Errorf("failed to parse VRonOutput from OpenAI response: %w", err)
	}

	return &output, nil
}
