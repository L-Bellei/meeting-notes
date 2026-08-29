package services

import (
	"context"
	"fmt"

	"meeting-notes/internal/repository"
)

var validSettings = map[string]func(string) error{
	"user_name":             func(string) error { return nil },
	"claude_code_token":     func(string) error { return nil },
	"claude_code_model":     validateEnum("", "haiku", "sonnet", "opus"),
	"auto_generate":         validateEnum("true", "false"),
	"whisper_language":      validateEnum("pt", "en", "es", "auto"),
	"whisper_model":         validateEnum("tiny", "base", "small", "medium", "large"),
	"keep_audio":            validateEnum("true", "false"),
	"recording_hotkey":      func(string) error { return nil },
	"meeting_name_template": func(string) error { return nil },
	"sidebar_pinned":        validateEnum("true", "false"),
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
