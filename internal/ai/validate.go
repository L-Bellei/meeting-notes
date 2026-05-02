package ai

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Ping verifica se o provedor de IA configurado tem uma chave válida.
// Retorna (false, nil) quando nenhuma chave está configurada.
// Retorna (true, nil) quando a chave é válida.
// Retorna (true, err) quando a chave existe mas a validação falha.
func Ping(ctx context.Context, settings SettingsReader) (configured bool, err error) {
	m, err := settings.GetAll(ctx)
	if err != nil {
		return false, err
	}
	provider := m["ai_provider"]
	switch provider {
	case "anthropic":
		key := m["anthropic_api_key"]
		if key == "" {
			return false, nil
		}
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
		key := m["openai_api_key"]
		if key == "" {
			return false, nil
		}
		// TODO: validate OpenAI key via API call (currently existence-check only)
		return true, nil
	default:
		return false, nil
	}
}
