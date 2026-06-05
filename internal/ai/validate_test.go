package ai_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/ai"
)

func TestConfigured(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]string
		want bool
	}{
		{"anthropic with key", map[string]string{"ai_provider": "anthropic", "anthropic_api_key": "sk-ant-x"}, true},
		{"anthropic empty key", map[string]string{"ai_provider": "anthropic", "anthropic_api_key": ""}, false},
		{"anthropic missing key", map[string]string{"ai_provider": "anthropic"}, false},
		{"openai with key", map[string]string{"ai_provider": "openai", "openai_api_key": "sk-proj-x"}, true},
		{"openai empty key", map[string]string{"ai_provider": "openai", "openai_api_key": ""}, false},
		{"empty provider", map[string]string{}, false},
		{"unknown provider", map[string]string{"ai_provider": "gemini", "anthropic_api_key": "x"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ai.Configured(tc.m); got != tc.want {
				t.Fatalf("Configured(%v) = %v, want %v", tc.m, got, tc.want)
			}
		})
	}
}

func TestErrNotConfiguredIsWrapped(t *testing.T) {
	repo := &fakeSettingsRepo{data: map[string]string{
		"ai_provider":       "anthropic",
		"anthropic_api_key": "",
	}}
	c := ai.NewDynamicAIClient(repo)
	_, _, _, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if !errors.Is(err, ai.ErrNotConfigured) {
		t.Fatalf("expected error to wrap ai.ErrNotConfigured, got %v", err)
	}
}

func TestIsAuthError(t *testing.T) {
	if ai.IsAuthError(errors.New("some random failure")) {
		t.Fatal("plain error should not be an auth error")
	}
	if !ai.IsAuthError(errors.New("authentication_error: invalid x-api-key")) {
		t.Fatal("substring fallback should detect auth error")
	}
}
