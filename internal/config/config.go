package config

import (
	"os"

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
	_ = godotenv.Load()

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
