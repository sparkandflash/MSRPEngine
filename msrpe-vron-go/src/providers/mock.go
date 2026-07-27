package providers

import (
	"context"
	"fmt"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
	"strings"
)

// MockInferenceProvider provides fast, deterministic responses for offline chat testing.
type MockInferenceProvider struct{}

func NewMockInferenceProvider() *MockInferenceProvider {
	return &MockInferenceProvider{}
}

func (p *MockInferenceProvider) Validate(ctx context.Context) error {
	return nil
}

func (p *MockInferenceProvider) GenerateStructured(ctx context.Context, systemPrompt string, vronCtx vron.VRonContext) (*vron.VRonOutput, error) {
	utils.LogInfo("[MockLLM] Simulating structured output for method: %s", vronCtx.Method)

	out := &vron.VRonOutput{
		Confidence: 100,
	}

	switch vronCtx.Method {
	case vron.MethodRespond:
		if vronCtx.PassedContext != "" {
			out.Response = fmt.Sprintf("Bleep bloop! Recalled memory: %s", vronCtx.PassedContext)
		} else if strings.Contains(strings.ToLower(vronCtx.STM), "cats") || strings.Contains(strings.ToLower(vronCtx.STM), "fact") || strings.Contains(strings.ToLower(vronCtx.STM), "store") {
			out.Response = "Bleep bloop! Fact recorded into LTM memory archives."
			out.Facts = []string{"Cats say meow."}
		} else {
			out.Response = fmt.Sprintf("Bleep bloop! [Mock VRon Engine Active] Received message. MindState: %s", vronCtx.MindState)
		}

	case vron.MethodUpdateMemory:
		out.Confidence = 95
		out.Facts = []string{"Mock memory update fact."}

	case vron.MethodConsolidate:
		out.Facts = []string{"Mock consolidated summary 1", "Mock consolidated summary 2"}

	case vron.MethodReact:
		out.SerotoninDelta = 5

	case vron.MethodTest:
		out.Result = "PASS"
		out.Reasoning = "Mock verification pass."

	default:
		out.Response = "Mock response complete."
	}

	return out, nil
}

// MockEmbeddingProvider produces deterministic 768-dim normalized vectors offline.
type MockEmbeddingProvider struct{}

func NewMockEmbeddingProvider() *MockEmbeddingProvider {
	return &MockEmbeddingProvider{}
}

func (p *MockEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, 768)
	for i := range vec {
		vec[i] = float32(i+1) / 768.0
	}
	return normalizeVector(vec), nil
}
