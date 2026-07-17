package responder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OpenAIResponder struct {
	config Config
}

func NewOpenAIResponder(config Config) *OpenAIResponder {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.cerebras.ai/v1" // Fallback to Cerebras default
	}
	if config.Model == "" {
		config.Model = "gpt-oss-120b" // Fallback to a supported Cerebras default
	}
	return &OpenAIResponder{config: config}
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []openAIChatMessage `json:"messages"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (r *OpenAIResponder) Respond(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", r.config.BaseURL)

	messages := []openAIChatMessage{}
	if r.config.SystemInstruction != "" {
		messages = append(messages, openAIChatMessage{
			Role:    "system",
			Content: r.config.SystemInstruction,
		})
	}
	messages = append(messages, openAIChatMessage{
		Role:    "user",
		Content: prompt,
	})

	reqBody := openAIChatRequest{
		Model:    r.config.Model,
		Messages: messages,
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
	if r.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.config.APIKey))
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr openAIChatResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != nil {
			return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned by model")
	}

	return chatResp.Choices[0].Message.Content, nil
}
