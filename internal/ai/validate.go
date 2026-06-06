package ai

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrNotConfigured is returned (wrapped) when no AI provider/key is configured.
var ErrNotConfigured = errors.New("AI not configured")

// Configured reports whether settings hold a usable AI provider + API key.
// It is a pure, network-free check suitable for hot paths (e.g. the pipeline).
func Configured(m map[string]string) bool {
	switch m["ai_provider"] {
	case "anthropic":
		return m["anthropic_api_key"] != ""
	case "openai":
		return m["openai_api_key"] != ""
	default:
		return false
	}
}

// IsAuthError reports whether err represents an API authentication failure
// (invalid/expired key), as opposed to a transient or unrelated error.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) && (apiErr.StatusCode == 401 || apiErr.StatusCode == 403) {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "authentication") ||
		strings.Contains(s, "invalid x-api-key") ||
		strings.Contains(s, "invalid api key") ||
		strings.Contains(s, "incorrect api key")
}

// Ping verifica se o provedor de IA configurado tem uma chave válida.
// Retorna (false, nil) quando nenhuma chave está configurada.
// Retorna (true, nil) quando a chave é válida.
// Retorna (true, err) quando a chave existe mas a validação falha.
func Ping(ctx context.Context, settings SettingsReader) (configured bool, err error) {
	m, err := settings.GetAll(ctx)
	if err != nil {
		return false, err
	}
	if !Configured(m) {
		return false, nil
	}
	switch m["ai_provider"] {
	case "anthropic":
		key := m["anthropic_api_key"]
		model := m["anthropic_model"]
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		c := anthropic.NewClient(option.WithAPIKey(key))
		_, pingErr := c.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(model),
			MaxTokens: 1,
			Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))},
		})
		return true, pingErr
	case "openai":
		// TODO: validate OpenAI key via API call (currently existence-check only)
		return true, nil
	default:
		return false, nil
	}
}
