package responder

import (
	"context"
	"fmt"
)

type MockResponder struct {
	config Config
}

func NewMockResponder(config Config) *MockResponder {
	return &MockResponder{config: config}
}

func (r *MockResponder) Respond(ctx context.Context, prompt string) (string, error) {
	systemPrompt := "I am a helpful assistant."
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}
	return fmt.Sprintf("[Mock Response] (System Instruction: %q) You said: %s", systemPrompt, prompt), nil
}
