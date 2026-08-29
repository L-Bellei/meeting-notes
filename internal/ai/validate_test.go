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
		{"with token", map[string]string{"claude_code_token": "sk-x"}, true},
		{"empty token", map[string]string{"claude_code_token": ""}, false},
		{"missing token", map[string]string{}, false},
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
		"claude_code_token": "",
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
	if !ai.IsAuthError(errors.New("OAuth token expired. Please run /login")) {
		t.Fatal("oauth token expired message should be detected as auth error")
	}
}
