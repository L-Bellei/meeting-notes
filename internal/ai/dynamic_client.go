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
	token := m["claude_code_token"]
	if token == "" {
		return nil, fmt.Errorf("%w (conecte com Claude nas Configurações)", ErrNotConfigured)
	}
	return NewClaudeCodeClient(token, m["claude_code_model"]), nil
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
