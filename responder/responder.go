package responder

import (
	"bufio"
	"context"
	"os"
	"strings"
)

// Responder defines the interface for generating responses from LLMs.
type Responder interface {
	Respond(ctx context.Context, prompt string) (string, error)
}

// Config holds the configuration for responders loaded from environment variables.
type Config struct {
	Type              string // gemini, openai, local-binary, embedded, mock
	APIKey            string
	BaseURL           string
	Model             string
	LocalBinaryPath   string
	SystemInstruction string
}

// LoadConfigFromEnv reads configurations from environment variables.
func LoadConfigFromEnv() Config {
	return Config{
		Type:              strings.ToLower(strings.TrimSpace(os.Getenv("LYRA_RESPONDER_TYPE"))),
		APIKey:            os.Getenv("LYRA_API_KEY"),
		BaseURL:           os.Getenv("LYRA_BASE_URL"),
		Model:             os.Getenv("LYRA_MODEL"),
		LocalBinaryPath:   os.Getenv("LYRA_LOCAL_BINARY_PATH"),
		SystemInstruction: os.Getenv("LYRA_SYSTEM_INSTRUCTION"),
	}
}

// loadEnvFile parses the local .env file and sets environment variables if they are not already set.
func loadEnvFile() {
	file, err := os.Open(".env")
	if err != nil {
		return // .env file is optional
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip quotes if they surround the value
		val = strings.Trim(val, `"'`)

		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

// NewResponderFromEnv initializes a responder based on the environment config.
func NewResponderFromEnv() (Responder, error) {
	loadEnvFile()
	config := LoadConfigFromEnv()
	if config.Type == "" {
		config.Type = "mock"
	}

	switch config.Type {
	case "gemini":
		return NewGeminiResponder(config), nil
	case "openai":
		return NewOpenAIResponder(config), nil
	case "local-binary":
		return NewLocalBinaryResponder(config), nil
	case "embedded":
		return NewEmbeddedResponder(config)
	case "mock":
		fallthrough
	default:
		return NewMockResponder(config), nil
	}
}
