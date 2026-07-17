package responder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"lyra/consolidator"
)

type GeminiResponder struct {
	config Config
}

func NewGeminiResponder(config Config) *GeminiResponder {
	if config.Model == "" {
		config.Model = "gemini-2.5-flash" // Default fallback model
	}
	return &GeminiResponder{config: config}
}

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

type geminiGenerateRequest struct {
	Contents          []geminiContent          `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (r *GeminiResponder) Respond(ctx context.Context, prompt string, heartRate float64, history []consolidator.Message) (string, error) {
	if r.config.APIKey == "" {
		return "", fmt.Errorf("Gemini API key is required but missing from environment variables (set LYRA_API_KEY)")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		r.config.Model, r.config.APIKey)

	// Wrap user input, heartrate value, and context history in a JSON object.
	userPayload := map[string]interface{}{
		"message":   prompt,
		"heartrate": heartRate,
		"history":   history,
	}
	payloadBytes, err := json.Marshal(userPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal user payload: %w", err)
	}

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: string(payloadBytes)},
				},
			},
		},
	}

	systemPrompt := DefaultSystemInstruction
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}

	reqBody.SystemInstruction = &geminiSystemInstruction{
		Parts: []geminiPart{
			{Text: systemPrompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr geminiGenerateResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("Gemini API returned non-200 status: %d", resp.StatusCode)
	}

	var geminiResp geminiGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response candidates returned by Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
