package responder

import (
	"context"
	"fmt"

	"lyra/consolidator"
	"lyra/prompts"
)

type MockResponder struct {
	config Config
}

func NewMockResponder(config Config) *MockResponder {
	return &MockResponder{config: config}
}

func (r *MockResponder) Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message) (string, error) {
	systemPrompt := prompts.GetResponderPrompt()
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}
	return fmt.Sprintf("[Mock Response] (System Instruction: %q, Mind State: %q, History Size: %d) You said: %s", systemPrompt, mindState, len(history), prompt), nil
}
