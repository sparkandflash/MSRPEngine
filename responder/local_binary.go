package responder

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type LocalBinaryResponder struct {
	config Config
}

func NewLocalBinaryResponder(config Config) *LocalBinaryResponder {
	if config.LocalBinaryPath == "" {
		config.LocalBinaryPath = "llama-cli" // Default fallback binary name
	}
	if config.Model == "" {
		config.Model = "./models/default.gguf" // Default model path fallback
	}
	return &LocalBinaryResponder{config: config}
}

func (r *LocalBinaryResponder) Respond(ctx context.Context, prompt string) (string, error) {
	// Construct the command arguments.
	// For llama-cli: -m <model> -p "<prompt>"
	// If system instruction is provided, we format it with a standard chat template
	fullPrompt := prompt
	if r.config.SystemInstruction != "" {
		fullPrompt = fmt.Sprintf("<|system|>\n%s\n<|user|>\n%s\n<|assistant|>\n", r.config.SystemInstruction, prompt)
	}

	args := []string{
		"-m", r.config.Model,
		"-p", fullPrompt,
		"--n-predict", "256", // Limit tokens to keep execution fast on CPU
		"--log-disable",      // Disable llama-cli logging output
	}

	cmd := exec.CommandContext(ctx, r.config.LocalBinaryPath, args...)

	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("local binary execution failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	output := stdout.String()

	output = strings.TrimPrefix(output, fullPrompt)

	return strings.TrimSpace(output), nil
}
