package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
)

// --- Gemini Request/Response Structs ---

type geminiPart struct {
	Text string `json:"text"`
}
type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}
type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

// geminiSchema defines a JSON schema object for the Gemini response_schema field.
type geminiSchema struct {
	Type       string                  `json:"type"`
	Properties map[string]geminiSchema `json:"properties,omitempty"`
	Required   []string                `json:"required,omitempty"`
	Enum       []string                `json:"enum,omitempty"`
	Items      *geminiSchema           `json:"items,omitempty"`
}

type geminiGenerationConfig struct {
	ResponseMIMEType string       `json:"responseMimeType"`
	ResponseSchema   geminiSchema `json:"responseSchema"`
}

type geminiGenerateRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig  `json:"generationConfig,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason,omitempty"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// --- GeminiInferenceProvider ---

type GeminiInferenceProvider struct {
	APIKey      string
	Model       string
	ValTimeout  time.Duration
	ExecTimeout time.Duration
}

func NewGeminiInferenceProvider(apiKey, model string, valTimeout, execTimeout time.Duration) *GeminiInferenceProvider {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiInferenceProvider{
		APIKey:      apiKey,
		Model:       model,
		ValTimeout:  valTimeout,
		ExecTimeout: execTimeout,
	}
}

func (p *GeminiInferenceProvider) Validate(ctx context.Context) error {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", p.APIKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: p.ValTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gemini validation failed with status %d", resp.StatusCode)
	}
	return nil
}

// GenerateStructured calls Gemini with native responseSchema enforcement.
// This guarantees the response is always a valid VRonOutput JSON object.
func (p *GeminiInferenceProvider) GenerateStructured(ctx context.Context, systemPrompt string, vronCtx vron.VRonContext) (*vron.VRonOutput, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("VRON_LLM_API_KEY is not set")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", p.Model, p.APIKey)

	// Build the user turn from VRonContext fields
	userPrompt := buildUserPrompt(vronCtx)

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: userPrompt}},
			},
		},
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		// Native JSON Schema enforcement — the key VRON-V1 requirement
		GenerationConfig: &geminiGenerationConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: geminiSchema{
				Type: "object",
				Properties: map[string]geminiSchema{
					"response":        {Type: "string"},
					"confidence":      {Type: "integer"},
					"need_memory":     {Type: "boolean"},
					"memory_query":    {Type: "string"},
					"need_test":       {Type: "boolean"},
					"test_query":      {Type: "string"},
					"facts":           {Type: "array", Items: &geminiSchema{Type: "string"}},
					"result":          {Type: "string"},
					"reasoning":       {Type: "string"},
					"observation":     {Type: "string"},
					"serotonin_delta": {Type: "integer"},
					"tasks":           {Type: "array", Items: &geminiSchema{Type: "string"}},
					"message":         {Type: "string"},
					"query":           {Type: "string"},
					"action":          {Type: "string"},
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

	client := &http.Client{Timeout: p.ExecTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr geminiGenerateResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return nil, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("gemini API returned non-200 status: %d", resp.StatusCode)
	}

	var geminiResp geminiGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	rawJSON := geminiResp.Candidates[0].Content.Parts[0].Text
	utils.LogDebug("LLM API Call (Gemini) | Sent: %d chars | Received: %d chars", len(jsonData), len(rawJSON))

	var output vron.VRonOutput
	if err := json.Unmarshal([]byte(rawJSON), &output); err != nil {
		return nil, fmt.Errorf("failed to parse VRonOutput from Gemini response: %w", err)
	}

	return &output, nil
}
