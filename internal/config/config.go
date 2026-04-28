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
		DatabasePath:       getEnv("DATABASE_PATH", defaultDatabasePath()),
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

// defaultDatabasePath returns %AppData%\Meeting Notes\meeting-notes.db on Windows
// (and the OS equivalent on other platforms), falling back to the current directory
// if the user config dir cannot be determined.
func defaultDatabasePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "./meeting-notes.db"
	}
	appDir := filepath.Join(dir, "Meeting Notes")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "./meeting-notes.db"
	}
	return filepath.Join(appDir, "meeting-notes.db")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv looks for a .env file in:
//  1. The user config dir (%AppData%\Meeting Notes\ on Windows) — used in production
//  2. Walking up from the current directory — used during development
func loadDotEnv() {
	// Production: %AppData%\Meeting Notes\.env
	if dir, err := os.UserConfigDir(); err == nil {
		candidate := filepath.Join(dir, "Meeting Notes", ".env")
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
			return
		}
	}

	// Development: walk up from CWD
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
