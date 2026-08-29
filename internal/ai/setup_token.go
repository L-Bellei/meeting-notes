package ai

import (
	"context"
	"os/exec"
)

func LaunchSetupToken() error {
	bin, err := findClaudeBinary()
	if err != nil {
		return err
	}
	return launchConsole(bin)
}

// Console visível de propósito: setup-token exige TTY interativo e imprime o
// token na tela para o usuário copiar (spike 2026-08-29 — com stdio
// redirecionado o comando trava sem sequer abrir o browser).
var launchConsole = func(bin string) error {
	cmd := exec.Command("cmd", "/c", "start", "Conectar com Claude", "cmd", "/k", bin, "setup-token")
	return cmd.Start()
}

func TestConnection(ctx context.Context, token, model string) error {
	return testConnectionWithRunner(ctx, token, model, execRunner{})
}

func testConnectionWithRunner(ctx context.Context, token, model string, r commandRunner) error {
	c := newClaudeCodeClientWithRunner(token, model, r)
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()
	_, _, _, err := c.callJSON(ctx, `Return ONLY the JSON object {"ok":true} and no extra text.`)
	return err
}
