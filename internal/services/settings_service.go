package services

import (
	"context"
	"fmt"

	"meeting-notes/internal/repository"
)

var validSettings = map[string]func(string) error{
	"user_name":         func(string) error { return nil },
	"ai_provider":       validateEnum("anthropic", "openai"),
	"anthropic_api_key": func(string) error { return nil },
	"anthropic_model":   validateEnum("claude-sonnet-4-6", "claude-opus-4-7", "claude-haiku-4-5"),
	"openai_api_key":    func(string) error { return nil },
	"openai_model":      validateEnum("gpt-4o", "gpt-4o-mini", "gpt-4-turbo"),
	"auto_generate":     validateEnum("true", "false"),
	"whisper_language":  validateEnum("pt", "en", "es", "auto"),
	"whisper_model":     validateEnum("tiny", "base", "small", "medium", "large"),
	"recording_hotkey": func(string) error { return nil },
}

func validateEnum(allowed ...string) func(string) error {
	set := make(map[string]bool, len(allowed))
	for _, v := range allowed {
		set[v] = true
	}
	return func(v string) error {
		if !set[v] {
			return fmt.Errorf("invalid value %q (allowed: %v)", v, allowed)
		}
		return nil
	}
}

type SettingsService struct {
	repo *repository.SettingsRepository
}

func NewSettingsService(repo *repository.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) GetAll(ctx context.Context) (map[string]string, error) {
	return s.repo.GetAll(ctx)
}

func (s *SettingsService) Update(ctx context.Context, updates map[string]string) error {
	for key, value := range updates {
		validate, ok := validSettings[key]
		if !ok {
			return &ValidationError{fmt.Sprintf("unknown setting key: %q", key)}
		}
		if err := validate(value); err != nil {
			return &ValidationError{fmt.Sprintf("setting %q: %v", key, err)}
		}
	}
	for key, value := range updates {
		if err := s.repo.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}
