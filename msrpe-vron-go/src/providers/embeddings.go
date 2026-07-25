package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"msrpe-vron-go/src/utils"
)

// --- Gemini Embedding Provider ---

type GeminiEmbeddingProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func NewGeminiEmbeddingProvider(baseURL, apiKey, model string, timeout time.Duration) *GeminiEmbeddingProvider {
	if model == "" {
		model = "text-embedding-004"
	}
	return &GeminiEmbeddingProvider{BaseURL: baseURL, APIKey: apiKey, Model: model, Timeout: timeout}
}

func (p *GeminiEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	var url string
	if p.BaseURL != "" {
		url = fmt.Sprintf("%s?key=%s", p.BaseURL, p.APIKey)
	} else {
		url = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent?key=%s", p.Model, p.APIKey)
	}

	reqBody := map[string]interface{}{
		"model": "models/" + p.Model,
		"content": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": text},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: p.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini embedding API returned status %d", resp.StatusCode)
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	vec := result.Embedding.Values
	if len(vec) == 0 {
		return nil, fmt.Errorf("empty embedding returned from Gemini")
	}

	vec = normalizeVector(vec)
	utils.LogDebug("Embedding (Gemini) | Input: %d chars | Vector dim: %d", len(text), len(vec))
	return vec, nil
}

// --- OpenAI Embedding Provider ---

type OpenAIEmbeddingProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func NewOpenAIEmbeddingProvider(baseURL, apiKey, model string, timeout time.Duration) *OpenAIEmbeddingProvider {
	if model == "" {
		model = "text-embedding-3-small"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIEmbeddingProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
		Timeout: timeout,
	}
}

func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/embeddings", strings.TrimSuffix(p.BaseURL, "/"))

	reqBody := map[string]interface{}{
		"model": p.Model,
		"input": text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	client := &http.Client{Timeout: p.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embedding API returned status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned from OpenAI")
	}

	vec := normalizeVector(result.Data[0].Embedding)
	utils.LogDebug("Embedding (OpenAI) | Input: %d chars | Vector dim: %d", len(text), len(vec))
	return vec, nil
}

// normalizeVector normalizes a float32 vector to unit length.
func normalizeVector(vec []float32) []float32 {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}
