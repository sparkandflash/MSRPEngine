package envconfig

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"msrpe-vron-go/src/utils"
)

// EnvConfig holds all absolute capacity limits loaded from .env
type EnvConfig struct {
	// API Credentials & Routing
	LLMProvider     string
	LLMAPIKey       string
	LLMModel        string
	LLMBaseURL      string
	EmbProvider     string
	EmbAPIKey       string
	EmbModel        string
	EmbBaseURL      string

	// Engine Limits & Capacities
	MaxGlobalEnergy        int
	BaseTickRateMs         time.Duration
	FatigueThresholdPct    float64
	MaxResponseChars       int
	MaxUserMessageChars    int
	LLMValidationTimeout   time.Duration
	LLMExecutionTimeout    time.Duration
	LLMTemperature         float64
	LTMMaxResults          int
	LTMDefaultWeight       int
	BaseLifetime           time.Duration
	UserIdleTimeout        time.Duration
	HibernationTimeout     time.Duration
}

var (
	configInstance *EnvConfig
	once           sync.Once
)

// Load parses the .env file and extracts absolute limits.
// It uses sync.Once to ensure the configuration is loaded exactly once,
// returning a guaranteed Singleton.
func Load() *EnvConfig {
	once.Do(func() {
		if err := godotenv.Load(".env"); err != nil {
			utils.LogInfo("[EnvConfig] Warning: could not load .env file: %v", err)
		}

		configInstance = &EnvConfig{
		LLMProvider:            os.Getenv("VRON_LLM_PROVIDER"),
		LLMAPIKey:              os.Getenv("VRON_LLM_API_KEY"),
		LLMModel:               os.Getenv("VRON_LLM_MODEL"),
		LLMBaseURL:             os.Getenv("VRON_LLM_BASE_URL"),
		EmbProvider:            os.Getenv("EMBEDDING_PROVIDER"),
		EmbAPIKey:              os.Getenv("EMBEDDING_API_KEY"),
		EmbModel:               os.Getenv("EMBEDDING_MODEL"),
		EmbBaseURL:             os.Getenv("EMBEDDING_API_URL"),
		MaxGlobalEnergy:        parseInt("MAX_GLOBAL_ENERGY", 100),
		BaseTickRateMs:         time.Duration(parseInt("BASE_TICK_RATE_MS", 4000)) * time.Millisecond,
		FatigueThresholdPct:    parseFloat("FATIGUE_THRESHOLD_PCT", 0.10),
		MaxResponseChars:       parseInt("VRON_MAX_RESPONSE_CHARS", 2000),
		MaxUserMessageChars:    parseInt("VRON_MAX_USER_MESSAGE_CHARS", 2000),
		LLMValidationTimeout:   time.Duration(parseInt("LLM_VALIDATION_TIMEOUT_SEC", 5)) * time.Second,
		LLMExecutionTimeout:    time.Duration(parseInt("LLM_EXECUTION_TIMEOUT_SEC", 60)) * time.Second,
		LLMTemperature:         parseFloat("LLM_TEMPERATURE", 0.7),
		LTMMaxResults:          parseInt("LTM_MAX_RESULTS", 5),
		LTMDefaultWeight:       parseInt("LTM_DEFAULT_WEIGHT", 70),
		BaseLifetime:           time.Duration(parseInt("VRON_BASE_LIFETIME_SEC", 300)) * time.Second,
			UserIdleTimeout:        time.Duration(parseInt("USER_IDLE_TIMEOUT_MINUTES", 5)) * time.Minute,
			HibernationTimeout:     time.Duration(parseInt("ENGINE_HIBERNATION_TIMEOUT_MINUTES", 15)) * time.Minute,
		}
	})

	return configInstance
}

func parseInt(key string, fallback int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		utils.LogInfo("[EnvConfig] Invalid integer for %s: %v. Using fallback %d.", key, err, fallback)
		return fallback
	}
	return val
}

func parseFloat(key string, fallback float64) float64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		utils.LogInfo("[EnvConfig] Invalid float for %s: %v. Using fallback %f.", key, err, fallback)
		return fallback
	}
	return val
}
