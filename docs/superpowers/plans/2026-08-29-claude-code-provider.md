# Provider único via subscription (Claude Code headless) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir os providers anthropic/openai por API key por um único provider `claude-code` que spawna `claude -p` autenticado pela subscription (CLAUDE_CODE_OAUTH_TOKEN), com login iniciado pelo app.

**Architecture:** `ClaudeCodeClient` implementa a interface `AIClient` existente trocando HTTP por spawn de processo (prompt via stdin, saída `--output-format json`). `DynamicAIClient` encolhe para um provider. Login: handler spawna `claude setup-token`, captura o token do stdout e salva nas settings. Execução injetável via interface `commandRunner` (fake nos testes; nenhum teste toca o CLI real).

**Tech Stack:** Go 1.22 (os/exec, chi), SQLite (migration 018), React 19 + TS (SettingsModal), React Query v5.

**Spec:** `docs/superpowers/specs/2026-08-29-claude-code-provider-design.md`

## Global Constraints

- Os dois entry points (`cmd/api/main.go` e `cmd/desktop/app.go`) registram as MESMAS rotas novas — manter em sincronia (convenção do CLAUDE.md).
- Sem comentários no código, salvo WHY não-óbvio (convenção do CLAUDE.md).
- Prompt do CLI vai via **stdin** (transcrições longas estouram o limite de linha de comando do Windows, ~32K chars); os args carregam só flags.
- Spawn de processo sempre com `CreationFlags: 0x08000000` (CREATE_NO_WINDOW), padrão já usado em `cmd/desktop/app.go:312`.
- Timeout por geração: 5 minutos (`generateTimeout`). Timeout do login (setup-token): 5 minutos.
- Settings novas: `claude_code_token` (string livre) e `claude_code_model` (enum `""`, `"haiku"`, `"sonnet"`, `"opus"`).
- Interface `AIClient` e assinaturas `Generate*` NÃO mudam (consumidores em services/orchestrator intocados).
- `ai.Configured` continua puro (sem I/O).
- Prompts de geração (textos `def`/`jsonFmt`) copiados byte a byte dos clients atuais.

---

### Task 1: Spike — captura do `claude setup-token` no Windows (REQUER O USUÁRIO)

**Files:**
- Create: `C:\Users\leo_b\AppData\Local\Temp\claude\...\scratchpad\spike-setup-token.ps1` (throwaway — NÃO commitar)

**Interfaces:**
- Produces: fatos para as Tasks 2 e 4 — (a) prefixo/formato exato do token no stdout (esperado: linha única começando com `sk-ant-oat`); (b) se o comando funciona com stdout redirecionado e sem TTY; (c) exit code no sucesso e no cancelamento; (d) caminho do binário nesta máquina (`(Get-Command claude).Source`).

Este task é interativo: o browser abre e o USUÁRIO precisa autorizar. Não despachar para subagent — executar na sessão principal com o usuário presente. O token capturado é real: NÃO imprimir inteiro em log/transcript (mostrar só prefixo + comprimento) e NÃO commitar.

- [ ] **Step 1: Localizar o binário e a versão**

Run: `(Get-Command claude -ErrorAction SilentlyContinue).Source; claude --version`
Expected: caminho + versão. Se ausente, PARAR e instalar antes de seguir.

- [ ] **Step 2: Rodar setup-token com stdout capturado**

```powershell
$p = Start-Process -FilePath "claude" -ArgumentList "setup-token" -RedirectStandardOutput "$env:TEMP\st-out.txt" -RedirectStandardError "$env:TEMP\st-err.txt" -PassThru -NoNewWindow
# usuário autoriza no browser
$p.WaitForExit(300000)
"exit=$($p.ExitCode)"
Get-Content "$env:TEMP\st-out.txt" | ForEach-Object { if ($_ -match '^sk-ant-') { "TOKEN LINE: $($_.Substring(0,12))... (len=$($_.Length))" } else { $_ } }
Get-Content "$env:TEMP\st-err.txt"
```

Expected: browser abre; após autorizar, uma linha de token aparece no stdout (ou stderr — registrar onde), exit 0.

- [ ] **Step 3: Registrar os fatos**

Anotar no ledger do SDD: prefixo real do token, stream onde ele sai, exit codes, se `-NoNewWindow`/sem TTY funcionou. Se a captura NÃO funcionar sem TTY, a Task 4 degrada para: o botão abre instruções + campo de colagem manual apenas (decisão já prevista no spec como fallback). Apagar `st-out.txt`/`st-err.txt` (contêm token real).

---

### Task 2: `internal/ai/client.go` + `claude_code_client.go` com testes

**Files:**
- Create: `internal/ai/client.go` (tipos/helpers movidos de `anthropic_client.go` — o arquivo de origem ainda não é apagado; a remoção é da Task 3)
- Create: `internal/ai/claude_code_client.go`
- Create: `internal/ai/claude_code_client_test.go`

**Interfaces:**
- Consumes: `AIClient`, `TaskSuggestion`, `buildInstruction`, `buildContext`, `stripJSONFence`, `ErrNotConfigured` (existentes).
- Produces: `NewClaudeCodeClient(token, model string) *ClaudeCodeClient` (satisfaz `AIClient`); interface interna `commandRunner` com `Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (stdout, stderr string, err error)`; `findClaudeBinary() (string, error)`; `newClaudeCodeClientWithRunner(token, model string, r commandRunner) *ClaudeCodeClient` (seam de teste). Task 4 reutiliza `commandRunner` e `findClaudeBinary`.

- [ ] **Step 1: Criar `internal/ai/client.go` movendo (recortar/colar, sem alterar) de `anthropic_client.go`:** `TaskSuggestion`, `AIClient`, `buildInstruction`, `buildContext`, `stripJSONFence`. Rodar `go build ./...` — deve compilar (mesma package).

- [ ] **Step 2: Escrever os testes que falham** em `claude_code_client_test.go`:

```go
package ai

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	stdout, stderr string
	err            error
	gotArgs        []string
	gotStdin       string
	gotEnv         []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (string, string, error) {
	f.gotArgs = args
	f.gotStdin = stdin
	f.gotEnv = extraEnv
	return f.stdout, f.stderr, f.err
}

func TestClaudeCode_GenerateSummary_ParsesResult(t *testing.T) {
	r := &fakeRunner{stdout: `{"type":"result","is_error":false,"result":"{\"summary\":\"resumo aqui\"}","usage":{"input_tokens":10,"output_tokens":5}}`}
	c := newClaudeCodeClientWithRunner("tok", "sonnet", r)
	got, in, out, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "resumo aqui" || in != 10 || out != 5 {
		t.Fatalf("got %q in=%d out=%d", got, in, out)
	}
	if f := strings.Join(r.gotArgs, " "); !strings.Contains(f, "--output-format json") || !strings.Contains(f, "--model sonnet") {
		t.Fatalf("args: %v", r.gotArgs)
	}
	if !strings.Contains(r.gotStdin, "transcript") {
		t.Fatalf("prompt deve ir via stdin, foi: %q", r.gotStdin)
	}
	found := false
	for _, e := range r.gotEnv {
		if e == "CLAUDE_CODE_OAUTH_TOKEN=tok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("token ausente do env: %v", r.gotEnv)
	}
}

func TestClaudeCode_NoModel_OmitsModelFlag(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":false,"result":"[\"p1\"]","usage":{}}`}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	if _, _, _, err := c.GenerateKeyPoints(context.Background(), "t", "", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(r.gotArgs, " "), "--model") {
		t.Fatalf("--model não deveria aparecer: %v", r.gotArgs)
	}
}

func TestClaudeCode_CLIError_WrapsStderr(t *testing.T) {
	r := &fakeRunner{stderr: "boom", err: context.DeadlineExceeded}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	_, _, _, err := c.GenerateSummary(context.Background(), "t", "", "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("stderr deve entrar no erro, veio: %v", err)
	}
}

func TestClaudeCode_IsErrorTrue_ReturnsResultAsError(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":true,"result":"OAuth token expired. Please run /login","usage":{}}`}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	_, _, _, err := c.GenerateSummary(context.Background(), "t", "", "")
	if err == nil || !IsAuthError(err) {
		t.Fatalf("erro de auth do CLI deve ser IsAuthError, veio: %v", err)
	}
}

func TestClaudeCode_MalformedCLIOutput_Errors(t *testing.T) {
	r := &fakeRunner{stdout: "not json"}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	if _, _, _, err := c.GenerateSummary(context.Background(), "t", "", ""); err == nil {
		t.Fatal("esperava erro de parse")
	}
}

func TestClaudeCode_GenerateTasks_ParsesArray(t *testing.T) {
	r := &fakeRunner{stdout: `{"is_error":false,"result":"[{\"description\":\"d\",\"assignee\":\"a\",\"priority\":\"high\"}]","usage":{"input_tokens":1,"output_tokens":2}}`}
	c := newClaudeCodeClientWithRunner("tok", "", r)
	tasks, _, _, err := c.GenerateTasks(context.Background(), "t", "", "")
	if err != nil || len(tasks) != 1 || tasks[0].Priority != "high" {
		t.Fatalf("tasks=%v err=%v", tasks, err)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./internal/ai/ -run TestClaudeCode -v`
Expected: FAIL (`newClaudeCodeClientWithRunner` undefined).

- [ ] **Step 4: Implementar `claude_code_client.go`**

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const generateTimeout = 5 * time.Minute

type commandRunner interface {
	Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (stdout, stderr string, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args []string, stdin string, extraEnv []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

func findClaudeBinary() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		// Instalador nativo do Windows coloca o binário fora do PATH de processos GUI.
		p := filepath.Join(home, ".local", "bin", "claude.exe")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w (binário claude não encontrado — instale o Claude Code)", ErrNotConfigured)
}

type ClaudeCodeClient struct {
	token  string
	model  string
	runner commandRunner
}

func NewClaudeCodeClient(token, model string) *ClaudeCodeClient {
	return newClaudeCodeClientWithRunner(token, model, execRunner{})
}

func newClaudeCodeClientWithRunner(token, model string, r commandRunner) *ClaudeCodeClient {
	return &ClaudeCodeClient{token: token, model: model, runner: r}
}

type cliResult struct {
	Result  string `json:"result"`
	IsError bool   `json:"is_error"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *ClaudeCodeClient) callJSON(ctx context.Context, prompt string) (string, int, int, error) {
	bin, err := findClaudeBinary()
	if err != nil {
		return "", 0, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()
	args := []string{"-p", "--output-format", "json",
		"--append-system-prompt", "You are a JSON-only API. Output only valid JSON. No prose, no markdown fences, no extra text."}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}
	stdout, stderr, err := c.runner.Run(ctx, bin, args, prompt, []string{"CLAUDE_CODE_OAUTH_TOKEN=" + c.token})
	if err != nil {
		return "", 0, 0, fmt.Errorf("claude cli: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	var res cliResult
	if jsonErr := json.Unmarshal([]byte(stdout), &res); jsonErr != nil {
		return "", 0, 0, fmt.Errorf("parse claude cli output: %w (raw: %.200s)", jsonErr, stdout)
	}
	if res.IsError {
		return "", 0, 0, fmt.Errorf("claude cli error: %s", res.Result)
	}
	return res.Result, res.Usage.InputTokens, res.Usage.OutputTokens, nil
}
```

Os três `Generate*` copiam os de `anthropic_client.go` byte a byte (mesmos `def`, `jsonFmt`, `fmt.Sprintf` e parses com `stripJSONFence`), trocando apenas `c.callJSON(ctx, prompt, 1024)` por `c.callJSON(ctx, prompt)`.

- [ ] **Step 5: Atualizar `IsAuthError`** em `validate.go` para reconhecer as frases do CLI — acrescentar às substrings existentes: `"oauth token"`, `"token expired"`, `"please run /login"`, `"invalid bearer"`. (A remoção do ramo `*anthropic.Error` fica para a Task 3, junto com a dependência.)

- [ ] **Step 6: Rodar e ver passar**

Run: `go test ./internal/ai/ -v`
Expected: PASS (novos + existentes).

- [ ] **Step 7: Commit**

```bash
git add internal/ai/client.go internal/ai/claude_code_client.go internal/ai/claude_code_client_test.go internal/ai/validate.go internal/ai/anthropic_client.go
git commit -m "feat: ClaudeCodeClient — geração via claude -p com runner injetável"
```

---

### Task 3: DynamicAIClient encolhe; anthropic/openai removidos; Configured/Ping novos

**Files:**
- Modify: `internal/ai/dynamic_client.go` (resolve)
- Modify: `internal/ai/validate.go` (Configured, Ping, IsAuthError sem SDK)
- Modify: `internal/ai/dynamic_client_test.go`, `internal/ai/validate_test.go`
- Delete: `internal/ai/anthropic_client.go`, `internal/ai/openai_client.go`
- Modify: `go.mod` (via `go mod tidy` — saem `anthropic-sdk-go` e o SDK da openai)

**Interfaces:**
- Consumes: `NewClaudeCodeClient`, `findClaudeBinary` (Task 2).
- Produces: `resolve()` lê `claude_code_token`/`claude_code_model`; `Configured(m) = m["claude_code_token"] != ""`; `Ping` com semântica: token vazio → `(false, nil)`; binário ausente → `(true, err descritivo)`; `claude --version` ok → `(true, nil)`.

- [ ] **Step 1: Adaptar os testes primeiro** — em `dynamic_client_test.go`, os cenários viram: token vazio → erro `ErrNotConfigured`; token presente → resolve sem erro (verificado via chamada com runner… o resolve retorna `*ClaudeCodeClient`, então o teste de sucesso apenas garante que `GenerateSummary` NÃO retorna `ErrNotConfigured` — vai falhar adiante no binário/exec, então o teste de sucesso usa `errors.Is(err, ErrNotConfigured) == false`). Em `validate_test.go`: `Configured` com/sem `claude_code_token`; `IsAuthError` com `"OAuth token expired. Please run /login"`.

```go
func TestDynamicClient_NoToken_ReturnsNotConfigured(t *testing.T) {
	repo := &fakeSettingsRepo{data: map[string]string{"claude_code_token": ""}}
	c := ai.NewDynamicAIClient(repo)
	_, _, _, err := c.GenerateSummary(context.Background(), "transcript", "", "")
	if !errors.Is(err, ai.ErrNotConfigured) {
		t.Fatalf("esperava ErrNotConfigured, veio %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/ai/ -v` (chaves antigas ainda no resolve).

- [ ] **Step 3: Implementar** — `resolve()` vira:

```go
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
```

`validate.go`: remover imports do SDK; `Configured` = `m["claude_code_token"] != ""`; `Ping`:

```go
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
```

`IsAuthError`: apagar o ramo `*anthropic.Error`, manter só as substrings (com as da Task 2).

- [ ] **Step 4: Apagar `anthropic_client.go` e `openai_client.go`; `go mod tidy`; `go build ./...`** — ajustar qualquer referência restante que o build apontar.

- [ ] **Step 5: Rodar a suíte inteira** — `go test ./...` — Expected: PASS em todos os pacotes (services/handlers usam a interface, não os tipos removidos).

- [ ] **Step 6: Commit**

```bash
git add -A internal/ai go.mod go.sum
git commit -m "feat: provider único claude-code — anthropic/openai por API key removidos"
```

---

### Task 4: Login pelo app + testar conexão (backend)

**Files:**
- Create: `internal/ai/setup_token.go`
- Create: `internal/ai/setup_token_test.go`
- Create: `internal/handlers/claude_login_handler.go`
- Create: `internal/handlers/claude_login_handler_test.go`
- Modify: `cmd/api/main.go` (~linha 181) e `cmd/desktop/app.go` (~linha 224): registrar `POST /api/ai/claude-login` e `POST /api/ai/test`

**Interfaces:**
- Consumes: `commandRunner`, `execRunner`, `findClaudeBinary` (Task 2); `SettingsService.Update` (existente); padrão de resposta de `internal/handlers/respond.go`.
- Produces: `ai.CaptureSetupToken(ctx context.Context) (string, error)` e seam `captureSetupTokenWithRunner(ctx, r commandRunner) (string, error)`; `ai.TestConnection(ctx context.Context, token, model string) error`; `handlers.NewClaudeLoginHandler(login func(ctx context.Context) (string, error), save func(ctx context.Context, token string) error) *ClaudeLoginHandler` com métodos `Login` e um `NewAITestHandler(test func(ctx context.Context) error)` com método `Test`.

- [ ] **Step 1: Testes de `setup_token.go`** (fake runner):

```go
func TestCaptureSetupToken_FindsTokenLine(t *testing.T) {
	r := &fakeRunner{stdout: "Opening browser...\n\nsk-ant-oat01-abc123DEF_ghi\n"}
	tok, err := captureSetupTokenWithRunner(context.Background(), r)
	if err != nil || tok != "sk-ant-oat01-abc123DEF_ghi" {
		t.Fatalf("tok=%q err=%v", tok, err)
	}
}

func TestCaptureSetupToken_NoToken_Errors(t *testing.T) {
	r := &fakeRunner{stdout: "Opening browser...\n"}
	if _, err := captureSetupTokenWithRunner(context.Background(), r); err == nil {
		t.Fatal("esperava erro de captura")
	}
}
```

(Ajustar o prefixo `sk-ant-oat` ao fato registrado pelo spike da Task 1 — se o spike viu outro prefixo/stream, o regex e a fonte mudam aqui.)

- [ ] **Step 2: Ver falhar** — `go test ./internal/ai/ -run TestCaptureSetupToken -v`.

- [ ] **Step 3: Implementar `setup_token.go`**

```go
package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const setupTokenTimeout = 5 * time.Minute

var tokenLine = regexp.MustCompile(`^sk-ant-[A-Za-z0-9_-]{20,}$`)

func CaptureSetupToken(ctx context.Context) (string, error) {
	return captureSetupTokenWithRunner(ctx, execRunner{})
}

func captureSetupTokenWithRunner(ctx context.Context, r commandRunner) (string, error) {
	bin, err := findClaudeBinary()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, setupTokenTimeout)
	defer cancel()
	stdout, stderr, err := r.Run(ctx, bin, []string{"setup-token"}, "", nil)
	if err != nil {
		return "", fmt.Errorf("claude setup-token: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	for _, line := range strings.Split(stdout+"\n"+stderr, "\n") {
		if l := strings.TrimSpace(line); tokenLine.MatchString(l) {
			return l, nil
		}
	}
	return "", fmt.Errorf("token não encontrado na saída do setup-token — rode `claude setup-token` no terminal e cole o token manualmente")
}

func TestConnection(ctx context.Context, token, model string) error {
	c := NewClaudeCodeClient(token, model)
	ctx, cancel := context.WithTimeout(ctx, generateTimeout)
	defer cancel()
	_, _, _, err := c.callJSON(ctx, `Return ONLY the JSON object {"ok":true} and no extra text.`)
	return err
}
```

- [ ] **Step 4: Testes do handler** — seguir a forma de `ai_health_handler_test.go` (handler recebe closures; sem servidor real): sucesso salva token e responde `{"token_masked":"sk-ant-oa...( n chars)"}` 200; login em andamento responde 409; falha responde 502 com a mensagem. Handler mantém `sync.Mutex` + flag `inFlight` para o 409.

- [ ] **Step 5: Ver falhar, implementar `claude_login_handler.go`, ver passar** — `Login` chama `login(ctx)`, depois `save(ctx, token)`, responde mascarado (`primeiros 10 chars + "…" + len`). `Test` chama `test(ctx)`: nil → `{"ok":true}`; `IsAuthError` → 502 com "reconecte com Claude"; outro erro → 502 com a mensagem.

- [ ] **Step 6: Registrar rotas nos DOIS entry points**, ao lado do `/api/ai/health` existente:

```go
claudeLoginHandler := handlers.NewClaudeLoginHandler(
	ai.CaptureSetupToken,
	func(ctx context.Context, token string) error {
		return settingsService.Update(ctx, map[string]string{"claude_code_token": token})
	},
)
r.Post("/api/ai/claude-login", claudeLoginHandler.Login)
aiTestHandler := handlers.NewAITestHandler(func(ctx context.Context) error {
	m, err := settingsRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	return ai.TestConnection(ctx, m["claude_code_token"], m["claude_code_model"])
})
r.Post("/api/ai/test", aiTestHandler.Test)
```

(Conferir os nomes reais das variáveis `settingsService`/`settingsRepo` em cada entry point e usar os existentes.)

- [ ] **Step 7: Suíte + build** — `go test ./... && go build ./...` — PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/ai internal/handlers cmd
git commit -m "feat: login pelo app (claude setup-token) e endpoint de teste de conexão"
```

---

### Task 5: Migration 018 + whitelist de settings

**Files:**
- Create: `internal/database/migrations/018_claude_code_provider.sql`
- Modify: `internal/services/settings_service.go:10-24` (mapa `validSettings`)
- Test: `internal/services/settings_service_test.go` (se não existir, criar seguindo o padrão dos testes de services — SQLite em memória via `t.TempDir()`, sem mocks)

**Interfaces:**
- Produces: chaves válidas `claude_code_token` (livre) e `claude_code_model` (`validateEnum("", "haiku", "sonnet", "opus")`); chaves `anthropic_*`/`openai_*`/`ai_provider` deixam de ser aceitas no Update.

- [ ] **Step 1: Teste que falha** — `Update` com `claude_code_token` aceita; com `anthropic_api_key` retorna `ValidationError`; com `claude_code_model: "opus"` aceita e `"gpt-4o"` recusa.

- [ ] **Step 2: Ver falhar; editar o mapa:**

```go
var validSettings = map[string]func(string) error{
	"user_name":              func(string) error { return nil },
	"claude_code_token":      func(string) error { return nil },
	"claude_code_model":      validateEnum("", "haiku", "sonnet", "opus"),
	"auto_generate":          validateEnum("true", "false"),
	"whisper_language":       validateEnum("pt", "en", "es", "auto"),
	"whisper_model":          validateEnum("tiny", "base", "small", "medium", "large"),
	"keep_audio":             validateEnum("true", "false"),
	"recording_hotkey":       func(string) error { return nil },
	"meeting_name_template":  func(string) error { return nil },
	"sidebar_pinned":         validateEnum("true", "false"),
}
```

- [ ] **Step 3: Migration** `018_claude_code_provider.sql`:

```sql
-- Provider único via subscription: chaves de API removidas do banco (irreversível;
-- downgrade para v2.7.x deixa de funcionar — ver DECISIONS.md 2026-08-29).
UPDATE settings SET value = 'claude-code' WHERE key = 'ai_provider';
DELETE FROM settings WHERE key IN ('anthropic_api_key', 'anthropic_model', 'openai_api_key', 'openai_model');
INSERT OR IGNORE INTO settings (key, value) VALUES ('claude_code_token', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('claude_code_model', '');
```

(Conferir na migration `005_settings.sql` se a tabela tem PK em `key` — `INSERT OR IGNORE` depende disso; se não tiver, usar o padrão `INSERT ... SELECT WHERE NOT EXISTS`.)

- [ ] **Step 4: Suíte** — `go test ./internal/services/ ./internal/database/ -v` — PASS (migrations aplicam ao abrir o banco nos testes de repository).

- [ ] **Step 5: Commit**

```bash
git add internal/database/migrations/018_claude_code_provider.sql internal/services
git commit -m "feat: migration 018 e whitelist — settings do provider claude-code"
```

---

### Task 6: Frontend — aba IA do SettingsModal

**Files:**
- Modify: `frontend/src/components/settings/SettingsModal.tsx` (bloco do provider de IA)
- Modify: `frontend/src/hooks/useSettings.ts` (nenhuma mudança de forma esperada — conferir se tipa as chaves) e criar mutation `useClaudeLogin` + `useAITest` (no hook onde `useAIConfigured`/health vivem — conferir `useAIConfigured.ts`)

**Interfaces:**
- Consumes: `POST /api/ai/claude-login` → `{token_masked: string}` | erro `{error: string}`; `POST /api/ai/test` → `{ok: true}` | erro; settings `claude_code_token`/`claude_code_model` via `useSettings`/`useUpdateSettings` existentes.
- Produces: UI descrita no spec (status, Conectar, colagem manual, seletor, Testar conexão).

- [ ] **Step 1: Hooks** — em `useAIConfigured.ts` (ou arquivo irmão novo `useClaudeCode.ts`):

```ts
export function useClaudeLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const res = await fetch("/api/ai/claude-login", { method: "POST" });
      if (!res.ok) throw new Error((await res.json()).error ?? "login falhou");
      return res.json() as Promise<{ token_masked: string }>;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["settings"] });
      qc.invalidateQueries({ queryKey: ["ai-health"] });
    },
  });
}

export function useAITest() {
  return useMutation({
    mutationFn: async () => {
      const res = await fetch("/api/ai/test", { method: "POST" });
      if (!res.ok) throw new Error((await res.json()).error ?? "teste falhou");
      return res.json() as Promise<{ ok: boolean }>;
    },
  });
}
```

(Seguir o padrão real de fetch/baseURL dos hooks existentes — conferir `useApi.ts` e usar o helper de lá se houver.)

- [ ] **Step 2: SettingsModal** — remover: select de `ai_provider`, campos `anthropic_api_key`/`openai_api_key` e selects de modelo antigos. Adicionar, no mesmo lugar: linha de status ("Claude Code: conectado (sk-ant-oa…)" ou "não conectado"), botão "Conectar com Claude" (estado `isPending` do `useClaudeLogin`: "Aguardando autorização no navegador…"), input de colagem manual gravando `claude_code_token` via `useUpdateSettings`, select de `claude_code_model` (`Padrão da assinatura` = `""`, `haiku`, `sonnet`, `opus`), botão "Testar conexão" mostrando sucesso/erro do `useAITest`. Textos em pt-BR como o resto do modal.

- [ ] **Step 3: Verificar** — `cd frontend && npx tsc --noEmit && npm run build` — Expected: sem erros.

- [ ] **Step 4: Exercício manual** — reiniciar o `wails dev` (armadilha do HMR: a janela nativa não recebe HMR) e percorrer: conectar (browser abre → autoriza → status muda), testar conexão, gerar resumo numa reunião existente.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat: aba IA por subscription — conectar com Claude, modelo e teste de conexão"
```

---

### Task 7: Documentação e registro

**Files:**
- Modify: `CLAUDE.md` (linha do stack: "**AI:** Claude via subscription — Claude Code headless (`internal/ai/claude_code_client.go`)")
- Modify: `.claude/DECISIONS.md` (nova entrada no topo)
- Modify: `.claude/BACKLOG.md` (débito da validação openai sai; entra a nota da semântica frouxa do health do claude-code)

**Interfaces:** nenhuma — só texto.

- [ ] **Step 1: DECISIONS.md** — entrada `[2026-08-29] IA via subscription (Claude Code CLI), provider único — API keys removidas`: contexto (Messages API não aceita credencial de assinatura; único caminho oficial é o CLI headless), escolha (spawn por chamada, prompt via stdin, login pelo app via setup-token), justificativa e trade-offs explícitos (dependência do binário `claude` instalado; rate limits da assinatura; migration 018 irreversível; restrição de distribuição — oferecer login claude.ai a terceiros exige aprovação da Anthropic).

- [ ] **Step 2: BACKLOG.md** — remover o item "Validação de chave OpenAI é só existência"; adicionar "Health do claude-code valida binário, não o token — validação real só no botão Testar conexão; `/api/ai/health` responde `valid:true` com token expirado."

- [ ] **Step 3: CLAUDE.md** — atualizar a linha de stack citada acima e a menção a `anthropic_client.go` na seção de arquitetura, se houver.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md .claude
git commit -m "docs: registrar o provider único via subscription"
```
