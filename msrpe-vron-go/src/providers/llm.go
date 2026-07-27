package providers

import (
	"encoding/json"
	"fmt"
	"msrpe-vron-go/src/utils"
	"msrpe-vron-go/src/vron"
)

// MockLLMProvider simulates an LLM call enforcing Native Structured Outputs (JSON Schema).
type MockLLMProvider struct {
	// In reality, this would hold OpenAI/Gemini client credentials
}

// GenerateStructuredOutput simulates a call to an LLM provider that enforces a JSON schema.
// By using native structured outputs, we guarantee the returned string perfectly unmarshals
// into the vron.VRonOutput struct, eliminating parse errors and fallback regexes.
func (p *MockLLMProvider) GenerateStructuredOutput(systemPrompt string, context vron.VRonContext) (*vron.VRonOutput, error) {
	// 1. Build the exact JSON schema definition required by OpenAI/Gemini based on vron.VRonOutput
	/*
		schema := {
			"type": "object",
			"properties": {
				"action": {"type": "string", "enum": ["respond", "spawn_child", "update_memory", "test_result"]},
				"goal": {
					"type": "string",
					"enum": ["React", "Respond", "Summarise", "Test", "Abstract"]
				},
				"query": {"type": "string"}
			},
			"required": ["action", "goal", "query"]
		}
	*/
	
	// 2. Call the external LLM API passing the schema
	// ... HTTP Request ...
	
	// 3. For the sake of this skeleton, we will mock a return response where the LLM
	// decides to spawn a child VRon because it needs more context.
	
	// Mocked response
	mockedLLMJSON := `{
		"action": "spawn_child",
		"goal": "Retrieve",
		"query": "Search memory for why the user hates apples"
	}`

	// Calculate Sent Character Count
	sentChars := len(systemPrompt) + len(context.STM) + len(context.LTM) + len(context.PassedContext)
	recvChars := len(mockedLLMJSON)

	utils.LogDebug("LLM API Call Made | Sent: %d chars | Received: %d chars", sentChars, recvChars)

	var output vron.VRonOutput
	err := json.Unmarshal([]byte(mockedLLMJSON), &output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse structured output (this should never happen with native schema): %v", err)
	}

	return &output, nil
}
