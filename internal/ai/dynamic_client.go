package ai

import (
	"context"
	"fmt"
)

// SettingsReader is satisfied by *repository.SettingsRepository.
type SettingsReader interface {
	GetAll(ctx context.Context) (map[string]string, error)
}

type DynamicAIClient struct {
	settings SettingsReader
}

func NewDynamicAIClient(settings SettingsReader) *DynamicAIClient {
	return &DynamicAIClient{settings: settings}
}

func (d *DynamicAIClient) resolve(ctx context.Context) (AIClient, error) {
	m, err := d.settings.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	provider := m["ai_provider"]
	switch provider {
	case "anthropic":
		key := m["anthropic_api_key"]
		if key == "" {
			return nil, fmt.Errorf("%w (anthropic)", ErrNotConfigured)
		}
		model := m["anthropic_model"]
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		return NewAnthropicClient(key, model), nil
	case "openai":
		key := m["openai_api_key"]
		if key == "" {
			return nil, fmt.Errorf("%w (openai)", ErrNotConfigured)
		}
		model := m["openai_model"]
		if model == "" {
			model = "gpt-4o"
		}
		return NewOpenAIClient(key, model), nil
	default:
		return nil, fmt.Errorf("%w (provider %q)", ErrNotConfigured, provider)
	}
}

func (d *DynamicAIClient) GenerateSummary(ctx context.Context, transcript, notes, customPrompt string) (string, int, int, error) {
	c, err := d.resolve(ctx)
	if err != nil {
		return "", 0, 0, err
	}
	return c.GenerateSummary(ctx, transcript, notes, customPrompt)
}

func (d *DynamicAIClient) GenerateKeyPoints(ctx context.Context, transcript, notes, customPrompt string) ([]string, int, int, error) {
	c, err := d.resolve(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	return c.GenerateKeyPoints(ctx, transcript, notes, customPrompt)
}

func (d *DynamicAIClient) GenerateTasks(ctx context.Context, transcript, notes, customPrompt string) ([]TaskSuggestion, int, int, error) {
	c, err := d.resolve(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	return c.GenerateTasks(ctx, transcript, notes, customPrompt)
}
