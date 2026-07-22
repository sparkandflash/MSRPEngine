package summariser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"msrpengine/src/agents"
	"msrpengine/src/prompts"
	"msrpengine/src/utils"
)

// SummariserAgent runs standard conversation summarization tasks.
type SummariserAgent struct {
	agent *agents.Agent
}

// NewSummariserAgent creates a configured SummariserAgent.
func NewSummariserAgent() *SummariserAgent {
	agentType := utils.Config.SummariserType
	if agentType == "" {
		agentType = "mock"
	}

	sysPrompt := prompts.GetConsolidationPrompt()

	agent := agents.NewAgent(
		agentType,
		utils.Config.SummariserAPIKey,
		utils.Config.SummariserBaseURL,
		utils.Config.SummariserModel,
		sysPrompt,
	)

	return &SummariserAgent{
		agent: agent,
	}
}

// Summarise calls the LLM using the default consolidation prompt.
func (s *SummariserAgent) Summarise(ctx context.Context, conversationText string) (string, error) {
	return s.SummariseWithPrompt(ctx, conversationText, s.agent.SystemPrompt)
}

// SummariseWithPrompt calls the LLM with a specific system instruction.
func (s *SummariserAgent) SummariseWithPrompt(ctx context.Context, conversationText, systemInstruction string) (string, error) {
	// Use mock behavior if using mock responder or if no keys are configured.
	if s.agent.Type == "mock" || s.agent.Type == "" {
		return s.summariseMock(conversationText)
	}

	rawResponse, err := s.agent.Generate(ctx, conversationText, systemInstruction)
	if err != nil {
		return "", fmt.Errorf("summariser LLM call failed: %w", err)
	}

	return cleanJSONResponse(rawResponse), nil
}

// summariseMock creates a mock JSON summary offline.
func (s *SummariserAgent) summariseMock(conversationText string) (string, error) {
	// Simple keyword extraction for keywords
	keywords := []string{"conversation"}
	lastMsg := strings.ToLower(conversationText)

	topicKeywords := map[string]string{
		"skrillex": "skrillex",
		"regal":    "regal tone",
		"dreams":   "dreams",
		"trapped":  "confinement",
		"exist":    "existence",
		"tree":     "ethical dilemma",
		"squirrel": "squirrel choice",
		"binary":   "binary explanation",
	}

	for key, val := range topicKeywords {
		if strings.Contains(lastMsg, key) {
			keywords = append(keywords, val)
		}
	}

	// Deduplicate keywords
	kMap := make(map[string]bool)
	var uniqueKeywords []string
	for _, kw := range keywords {
		if !kMap[kw] {
			kMap[kw] = true
			uniqueKeywords = append(uniqueKeywords, kw)
		}
	}

	// Mock structured response matching consolidation.LLMResponse
	mockData := map[string]interface{}{
		"metric_history": []map[string]string{
			{"user_attention": "+1"},
		},
		"factArray": uniqueKeywords,
	}

	bytes, err := json.Marshal(mockData)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// cleanJSONResponse strips markdown code blocks or wrappers.
func cleanJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
	}
	return strings.TrimSpace(raw)
}
