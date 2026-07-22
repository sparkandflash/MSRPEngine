package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"msrpengine/src/utils"
)

// Agent represents a unified LLM client capable of communicating with various providers.
type Agent struct {
	Type         string // e.g. "gemini", "openai", "local-binary", "embedded"
	APIKey       string
	BaseURL      string
	Model        string
	SystemPrompt string
}

// NewAgent creates a new Agent instance.
func NewAgent(agentType, apiKey, baseURL, model, systemPrompt string) *Agent {
	return &Agent{
		Type:         agentType,
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Model:        model,
		SystemPrompt: systemPrompt,
	}
}

// Generate sends the userPrompt to the configured LLM and returns the raw string response.
func (a *Agent) Generate(ctx context.Context, userPrompt string, sysPromptOverride string) (string, error) {
	activeSysPrompt := a.SystemPrompt
	if sysPromptOverride != "" {
		activeSysPrompt = sysPromptOverride
	}

	switch a.Type {
	case "gemini":
		return a.generateGemini(ctx, userPrompt, activeSysPrompt)
	case "openai":
		return a.generateOpenAI(ctx, userPrompt, activeSysPrompt)

	case "mock":
		return fmt.Sprintf(`{"reply":"Mock response to: %s","useful_episode_id":""}`, userPrompt), nil
	default:
		return "", fmt.Errorf("unsupported agent type: %s", a.Type)
	}
}

// Validate pings the provider's models endpoint to verify credentials.
func (a *Agent) Validate(ctx context.Context) error {
	if a.Type == "mock" || a.Type == "" {
		return nil
	}

	var url string
	var req *http.Request
	var err error

	if a.Type == "gemini" {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", a.APIKey)
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
	} else if a.Type == "openai" {
		if a.BaseURL == "" {
			return fmt.Errorf("missing base URL")
		}
		url = fmt.Sprintf("%s/models", strings.TrimSuffix(a.BaseURL, "/"))
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err == nil && a.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+a.APIKey)
		}
	} else {
		return fmt.Errorf("unknown config type %q", a.Type)
	}

	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d from %s", resp.StatusCode, url)
	}
	return nil
}

// ─── Gemini Implementation ──────────────────────────────────────────────────

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
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason,omitempty"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (a *Agent) generateGemini(ctx context.Context, userPrompt string, activeSysPrompt string) (string, error) {
	if a.APIKey == "" {
		return "", fmt.Errorf("gemini api key is required")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", a.Model, a.APIKey)

	reqBody := geminiGenerateRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: userPrompt},
				},
			},
		},
	}

	if activeSysPrompt != "" {
		reqBody.SystemInstruction = &geminiSystemInstruction{
			Parts: []geminiPart{{Text: activeSysPrompt}},
		}
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

	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no response candidates returned by Gemini")
	}

	candidate := geminiResp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != "" && candidate.FinishReason != "STOP" {
			return "", fmt.Errorf("Gemini response blocked/terminated. Reason: %s", candidate.FinishReason)
		}
		return "", fmt.Errorf("no response candidate content returned")
	}

	result := candidate.Content.Parts[0].Text
	utils.LogMetrics("gemini", len(jsonData), len(result))
	return result, nil
}

// ─── OpenAI Implementation ──────────────────────────────────────────────────

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float32         `json:"temperature,omitempty"`
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

func (a *Agent) generateOpenAI(ctx context.Context, userPrompt string, activeSysPrompt string) (string, error) {
	if a.APIKey == "" {
		return "", fmt.Errorf("openai api key is required")
	}

	url := fmt.Sprintf("%s/chat/completions", a.BaseURL)

	reqBody := openAIRequest{
		Model:       a.Model,
		Temperature: 0.7,
		Messages:    []openAIMessage{},
	}

	if activeSysPrompt != "" {
		reqBody.Messages = append(reqBody.Messages, openAIMessage{Role: "system", Content: activeSysPrompt})
	}
	reqBody.Messages = append(reqBody.Messages, openAIMessage{Role: "user", Content: userPrompt})

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr openAIResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API returned non-200 status: %d", resp.StatusCode)
	}

	var oaiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&oaiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned by OpenAI")
	}

	result := oaiResp.Choices[0].Message.Content
	utils.LogMetrics("openai", len(jsonData), len(result))
	return result, nil
}
