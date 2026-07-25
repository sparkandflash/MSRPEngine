package providers

import (
	"context"

	"msrpe-vron-go/src/envconfig"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
)

// InferenceProvider defines the interface all LLM backends must implement for VRon execution.
// The critical difference from V2: GenerateStructured enforces a JSON schema response,
// not a raw string. This is non-negotiable for VRon output parsing.
type InferenceProvider interface {
	// GenerateStructured calls the LLM with a native JSON schema enforcement,
	// returning a parsed VRonOutput directly. No regex fallback, no string parsing.
	GenerateStructured(ctx context.Context, systemPrompt string, vronCtx vron.VRonContext) (*vron.VRonOutput, error)

	// Validate pings the provider's endpoint to verify credentials before boot.
	Validate(ctx context.Context) error
}

// EmbeddingProvider defines the interface all embedding backends must implement.
type EmbeddingProvider interface {
	// Embed converts a text string into a normalized float32 vector.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// NewInferenceProvider is a factory that returns the appropriate LLM provider.
// It independently extracts credentials from the environment and self-validates.
func NewInferenceProvider() (InferenceProvider, error) {
	config := envconfig.Load()
	
	var provider InferenceProvider
	switch config.LLMProvider {
	case "openai":
		provider = NewOpenAIInferenceProvider(config.LLMBaseURL, config.LLMAPIKey, config.LLMModel, config.LLMValidationTimeout, config.LLMExecutionTimeout, config.LLMTemperature)
	case "gemini":
		provider = NewGeminiInferenceProvider(config.LLMAPIKey, config.LLMModel, config.LLMValidationTimeout, config.LLMExecutionTimeout)
	default:
		provider = NewGeminiInferenceProvider(config.LLMAPIKey, config.LLMModel, config.LLMValidationTimeout, config.LLMExecutionTimeout) // Gemini as default
	}

	// Self-validate credentials before boot
	if err := provider.Validate(context.Background()); err != nil {
		return nil, err
	}
	utils.LogInfo("[Provider] LLM provider OK (%s / %s)", config.LLMProvider, config.LLMModel)
	return provider, nil
}

// NewEmbeddingProvider is a factory that returns the appropriate embedding provider.
func NewEmbeddingProvider() EmbeddingProvider {
	config := envconfig.Load()
	
	switch config.EmbProvider {
	case "openai":
		return NewOpenAIEmbeddingProvider(config.EmbBaseURL, config.EmbAPIKey, config.EmbModel, config.LLMExecutionTimeout)
	case "gemini":
		return NewGeminiEmbeddingProvider(config.EmbBaseURL, config.EmbAPIKey, config.EmbModel, config.LLMExecutionTimeout)
	default:
		return NewGeminiEmbeddingProvider(config.EmbBaseURL, config.EmbAPIKey, config.EmbModel, config.LLMExecutionTimeout) // Gemini as default
	}
}
