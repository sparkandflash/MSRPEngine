package responder

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"

	"lyra/consolidator"
	"lyra/prompts"
)

// EpisodeSummary is a lightweight episode view passed to the responder LLM as episodic context.
type EpisodeSummary struct {
	ID            string   `json:"id"`
	Summary       string   `json:"summary"`
	Keywords      []string `json:"keywords"`
	PeakMindState string   `json:"peak_mindstate"`
	Conclusion    string   `json:"conclusion"`
}

// Responder defines the interface for generating responses from LLMs.
// It returns:
//   - reply:           the conversational reply text to show the user
//   - usefulEpisodeID: the episode ID the model found most relevant (empty string if none)
//   - err:             any error that occurred
type Responder interface {
	Respond(ctx context.Context, prompt string, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (reply string, usefulEpisodeID string, err error)
	RespondProactive(ctx context.Context, mindState string, history []consolidator.Message, episodes []EpisodeSummary) (reply string, usefulEpisodeID string, err error)
}

// parseResponderOutput parses the structured JSON the responder LLM is expected to return.
// If the raw content is not valid JSON (e.g. legacy plain-text mode or mock), the raw
// content is returned as the reply with an empty usefulEpisodeID.
func parseResponderOutput(raw string) (string, string, error) {
	raw = strings.TrimSpace(raw)
	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}

	var out struct {
		Reply           string `json:"reply"`
		UsefulEpisodeID string `json:"useful_episode_id"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		// Graceful fallback: treat the whole response as plain reply text
		return raw, "", nil
	}
	return out.Reply, out.UsefulEpisodeID, nil
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

// LoadReactorConfigFromEnv reads reactor-specific configurations from environment variables,
// falling back to standard responder variables if the reactor-specific ones are not set.
func LoadReactorConfigFromEnv() Config {
	loadEnvFile() // Ensure the .env file is loaded before parsing config

	rType := os.Getenv("LYRA_REACTOR_TYPE")
	if rType == "" {
		rType = os.Getenv("LYRA_RESPONDER_TYPE")
	}

	apiKey := os.Getenv("LYRA_REACTOR_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LYRA_API_KEY")
	}

	baseURL := os.Getenv("LYRA_REACTOR_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("LYRA_BASE_URL")
	}

	model := os.Getenv("LYRA_REACTOR_MODEL")
	if model == "" {
		model = os.Getenv("LYRA_MODEL")
	}

	binaryPath := os.Getenv("LYRA_REACTOR_LOCAL_BINARY_PATH")
	if binaryPath == "" {
		binaryPath = os.Getenv("LYRA_LOCAL_BINARY_PATH")
	}

	sysInst := os.Getenv("LYRA_REACTOR_SYSTEM_INSTRUCTION")
	if sysInst == "" {
		sysInst = os.Getenv("LYRA_SYSTEM_INSTRUCTION")
	}
	if sysInst == "" {
		sysInst = prompts.GetReactorPrompt()
	}

	return Config{
		Type:              strings.ToLower(strings.TrimSpace(rType)),
		APIKey:            apiKey,
		BaseURL:           baseURL,
		Model:             model,
		LocalBinaryPath:   binaryPath,
		SystemInstruction: sysInst,
	}
}

// LoadSummariserConfigFromEnv reads summariser-specific configurations from environment variables,
// falling back to standard responder variables if the summariser-specific ones are not set.
func LoadSummariserConfigFromEnv() Config {
	loadEnvFile() // Ensure the .env file is loaded before parsing config

	sType := os.Getenv("LYRA_SUMMARISER_TYPE")
	if sType == "" {
		sType = os.Getenv("LYRA_RESPONDER_TYPE")
	}

	apiKey := os.Getenv("LYRA_SUMMARISER_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("LYRA_API_KEY")
	}

	baseURL := os.Getenv("LYRA_SUMMARISER_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("LYRA_BASE_URL")
	}

	model := os.Getenv("LYRA_SUMMARISER_MODEL")
	if model == "" {
		model = os.Getenv("LYRA_MODEL")
	}

	binaryPath := os.Getenv("LYRA_SUMMARISER_LOCAL_BINARY_PATH")
	if binaryPath == "" {
		binaryPath = os.Getenv("LYRA_LOCAL_BINARY_PATH")
	}

	sysInst := os.Getenv("LYRA_SUMMARISER_SYSTEM_INSTRUCTION")
	if sysInst == "" {
		sysInst = prompts.GetConsolidationPrompt()
	}

	return Config{
		Type:              strings.ToLower(strings.TrimSpace(sType)),
		APIKey:            apiKey,
		BaseURL:           baseURL,
		Model:             model,
		LocalBinaryPath:   binaryPath,
		SystemInstruction: sysInst,
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
	if config.SystemInstruction == "" {
		config.SystemInstruction = prompts.GetResponderPrompt()
	}
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
