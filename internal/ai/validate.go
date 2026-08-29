package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotConfigured is returned (wrapped) when no AI provider/key is configured.
var ErrNotConfigured = errors.New("AI not configured")

// Configured reports whether settings hold a usable claude-code token.
// It is a pure, network-free check suitable for hot paths (e.g. the pipeline).
func Configured(m map[string]string) bool {
	return m["claude_code_token"] != ""
}

// IsAuthError reports whether err represents an API authentication failure
// (invalid/expired key), as opposed to a transient or unrelated error.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "authentication") ||
		strings.Contains(s, "invalid x-api-key") ||
		strings.Contains(s, "invalid api key") ||
		strings.Contains(s, "incorrect api key") ||
		strings.Contains(s, "oauth token") ||
		strings.Contains(s, "token expired") ||
		strings.Contains(s, "please run /login") ||
		strings.Contains(s, "invalid bearer")
}

// Ping verifica se o token do claude-code está configurado e o binário funciona.
// Retorna (false, nil) quando nenhum token está configurado.
// Retorna (true, nil) quando `claude --version` roda com sucesso.
// Retorna (true, err) quando o token existe mas o binário está ausente ou falha.
func Ping(ctx context.Context, settings SettingsReader) (configured bool, err error) {
	m, err := settings.GetAll(ctx)
	if err != nil {
		return false, err
	}
	if !Configured(m) {
		return false, nil
	}
	bin, err := findClaudeBinary()
	if err != nil {
		return true, err
	}
	_, stderr, err := execRunner{}.Run(ctx, bin, []string{"--version"}, "", nil)
	if err != nil {
		return true, fmt.Errorf("claude --version falhou: %w (stderr: %s)", err, stderr)
	}
	return true, nil
}
