package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPPort           string
	DatabasePath       string
	AnthropicAPIKey    string
	AnthropicModel     string
	AudioServiceURL    string
	MaxTokens          string
	WhisperModel       string
	WhisperDevice      string
	WhisperComputeType string
	WhisperLanguage    string
}

func Load() *Config {
	loadDotEnv()

	return &Config{
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		DatabasePath:       getEnv("DATABASE_PATH", "./meeting-notes.db"),
		AnthropicAPIKey:    getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:     getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-6"),
		AudioServiceURL:    getEnv("AUDIO_SERVICE_URL", "http://localhost:8765"),
		MaxTokens:          getEnv("MAX_TOKENS", "4096"),
		WhisperModel:       getEnv("WHISPER_MODEL", "medium"),
		WhisperDevice:      getEnv("WHISPER_DEVICE", "cuda"),
		WhisperComputeType: getEnv("WHISPER_COMPUTE_TYPE", "int8_float16"),
		WhisperLanguage:    getEnv("WHISPER_LANGUAGE", "pt"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv walks up from the current directory until it finds a .env file or
// reaches the filesystem root. This handles running from subdirectories like
// cmd/desktop/ during wails dev.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		candidate := filepath.Join(dir, ".env")
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
