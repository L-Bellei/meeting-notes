package ai_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/ai"
)

type fakeSettingsRepo struct {
	data map[string]string
}

func (f *fakeSettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	return f.data, nil
}

func TestDynamicClient_NoToken_ReturnsNotConfigured(t *testing.T) {
	repo := &fakeSettingsRepo{data: map[string]string{"claude_code_token": ""}}
	c := ai.NewDynamicAIClient(repo)
	_, _, _, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if !errors.Is(err, ai.ErrNotConfigured) {
		t.Fatalf("esperava ErrNotConfigured, veio %v", err)
	}
}

func TestDynamicClient_WithToken_ResolvesClient(t *testing.T) {
	repo := &fakeSettingsRepo{data: map[string]string{
		"claude_code_token": "sk-token",
		"claude_code_model": "sonnet",
	}}
	c := ai.NewDynamicAIClient(repo)
	_, _, _, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if errors.Is(err, ai.ErrNotConfigured) {
		t.Fatalf("nao esperava ErrNotConfigured, veio %v", err)
	}
}
