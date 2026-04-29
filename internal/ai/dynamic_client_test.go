package ai_test

import (
	"context"
	"testing"

	"meeting-notes/internal/ai"
)

type fakeSettingsRepo struct {
	data map[string]string
}

func (f *fakeSettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	return f.data, nil
}

func TestDynamicClient_NoAPIKey_ReturnsError(t *testing.T) {
	repo := &fakeSettingsRepo{data: map[string]string{
		"ai_provider":       "anthropic",
		"anthropic_api_key": "",
		"anthropic_model":   "claude-sonnet-4-6",
	}}
	c := ai.NewDynamicAIClient(repo)
	_, _, _, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if err == nil {
		t.Fatal("expected error when API key is empty, got nil")
	}
}

func TestDynamicClient_UnknownProvider_ReturnsError(t *testing.T) {
	repo := &fakeSettingsRepo{data: map[string]string{
		"ai_provider": "gemini",
	}}
	c := ai.NewDynamicAIClient(repo)
	_, _, _, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}
