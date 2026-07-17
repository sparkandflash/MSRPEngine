package reactor

import (
	"context"
	"testing"

	"lyra/consolidator"
)

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Clean JSON",
			input:    `{"direction": "increase", "magnitude": 0.05}`,
			expected: `{"direction": "increase", "magnitude": 0.05}`,
		},
		{
			name:     "Markdown Wrapped JSON",
			input:    "```json\n{\"direction\": \"decrease\", \"magnitude\": 0.10}\n```",
			expected: `{"direction": "decrease", "magnitude": 0.10}`,
		},
		{
			name:     "Generic Markdown Block",
			input:    "```\n{\"direction\": \"stable\", \"magnitude\": 0.0}\n```",
			expected: `{"direction": "stable", "magnitude": 0.0}`,
		},
		{
			name:     "Spaced Markdown Wrapping",
			input:    "   ```json\n{\n  \"direction\": \"increase\"\n}\n```   ",
			expected: "{\n  \"direction\": \"increase\"\n}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := cleanJSONResponse(tc.input)
			if actual != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func TestMockReactorEscalation(t *testing.T) {
	agent := NewReactorAgent()

	// 1. Test positive input
	historyPositive := []consolidator.Message{
		{Role: "user", Content: "I am so excited and happy right now!"},
	}
	resp, err := agent.React(context.Background(), historyPositive)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PositiveEmotion <= 0.5 {
		t.Errorf("expected positive emotion to increase, got %.2f", resp.PositiveEmotion)
	}

	// 2. Test negative input
	historyNegative := []consolidator.Message{
		{Role: "user", Content: "I absolutely hate this and I am extremely angry and resentful!"},
	}
	resp, err = agent.React(context.Background(), historyNegative)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.NegativeEmotion <= 0.3 {
		t.Errorf("expected negative emotion to increase, got %.2f", resp.NegativeEmotion)
	}

	// 3. Test stable/default input
	historyStable := []consolidator.Message{
		{Role: "user", Content: "Standard text."},
	}
	resp, err = agent.React(context.Background(), historyStable)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.NegativeEmotion > 0.4 || resp.PositiveEmotion > 0.6 {
		t.Errorf("expected emotions to stay near default baseline, got %+v", resp)
	}
}
