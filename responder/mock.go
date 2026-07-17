package responder

import (
	"context"
	"fmt"

	"lyra/consolidator"
)

type MockResponder struct {
	config Config
}

func NewMockResponder(config Config) *MockResponder {
	return &MockResponder{config: config}
}

func (r *MockResponder) Respond(ctx context.Context, prompt string, heartRate float64, history []consolidator.Message) (string, error) {
	systemPrompt := DefaultSystemInstruction
	if r.config.SystemInstruction != "" {
		systemPrompt = r.config.SystemInstruction
	}
	return fmt.Sprintf("[Mock Response] (System Instruction: %q, Heart Rate: %.2f, History Size: %d) You said: %s", systemPrompt, heartRate, len(history), prompt), nil
}
