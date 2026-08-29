# Provider único de IA via subscription Claude (Claude Code headless) — Design

**Data:** 2026-08-29
**Status:** aprovado em brainstorm (chat), aguardando plano
**Decisões do usuário:** substituir **todos** os providers por API key (anthropic e openai saem); login iniciado pelo app via browser; seletor simples de modelo.

## Problema

O app gera resumo, pontos-chave e tasks pela Anthropic Messages API com API key, cobrando créditos de API. O usuário tem assinatura Claude (Pro/Max) e quer que esse consumo saia da assinatura. A Messages API não aceita credencial de assinatura (é explicitamente não-programática); o único caminho oficial é o **Claude Code em modo headless** (`claude -p`), autenticado por token OAuth de longa duração emitido por `claude setup-token` e cobrado da assinatura.

## Decisão de arquitetura

**Abordagem A — spawn por chamada, stateless.** Cada geração executa `claude -p <prompt> --output-format json [--model <m>]` com `CLAUDE_CODE_OAUTH_TOKEN` no ambiente e parseia o JSON do stdout. Alternativas rejeitadas: processo quente reutilizado (gestão de ciclo de vida sem retorno para poucas chamadas/hora) e sidecar com Agent SDK (runtime Node/Python inteiro por hooks que não usamos).

O overhead de boot do CLI (~2–5s por chamada) é irrelevante no pipeline (a transcrição leva minutos) e aceitável nos botões manuais.

## Componentes

### `internal/ai/claude_code_client.go` (novo)

- `ClaudeCodeClient` implementa a interface `AIClient` existente: `GenerateSummary/GenerateKeyPoints/GenerateTasks(ctx, transcript, notes, customPrompt) (resultado, tokensIn, tokensOut, error)`.
- Prompts inalterados: reutiliza `buildInstruction`/`buildContext` e os formatos `Return ONLY a JSON ...` dos clients atuais.
- `callJSON` troca HTTP por processo: `claude -p <prompt> --output-format json --model <sel>`, env com `CLAUDE_CODE_OAUTH_TOKEN`, `CREATE_NO_WINDOW` (mesmo padrão do audio-service). Saída: campo `result` + contadores de uso (zero se ausentes).
- Execução via interface interna `commandRunner` injetável — produção usa `exec.CommandContext`; testes usam fake. Nenhum teste toca o CLI real.
- Descoberta do binário a cada chamada: `exec.LookPath("claude")` com fallback para `%USERPROFILE%\.local\bin\claude.exe` (instalador nativo do Windows).
- Timeout por geração: **5 minutos** via `context.WithTimeout` no `callJSON`.

### `DynamicAIClient` (encolhe)

`resolve()` só resolve `claude-code`, lendo `claude_code_token` e `claude_code_model` das settings; a chave `ai_provider` deixa de ser lida pelo código (a migration a mantém com valor `claude-code` apenas como registro de estado do banco). `anthropic_client.go`, `openai_client.go` e os SDKs correspondentes saem do `go.mod`. `cmd/api` e `cmd/desktop` não mudam de forma.

### Login iniciado pelo app

- UI (aba IA do `SettingsModal`): status (binário encontrado + versão; token presente, mascarado), botão **"Conectar com Claude"**, campo de colagem manual (fallback), seletor de modelo, botão **"Testar conexão"**.
- `POST /api/ai/claude-login`: spawna `claude setup-token` com stdout capturado e timeout de ~5 min. O CLI abre o browser (callback localhost; sem colar código), o usuário autoriza, o token sai no stdout; o handler captura a linha com formato de token OAuth (heurística por prefixo/comprimento — não há `--json`), salva em `claude_code_token` e responde mascarado.
- Falha de captura (formato mudou, timeout, callback bloqueado) → resposta orienta o fluxo manual: rodar `claude setup-token` no terminal e colar o token no campo.
- Um login por vez: o handler rejeita spawn duplicado enquanto houver um em andamento.

**Ponto frágil assumido:** o formato do stdout do `setup-token` não é documentado. **Task 1 do plano é um spike** validando na máquina do usuário o comportamento com stdout redirecionado no Windows, antes do código definitivo.

**Restrição de distribuição:** a Anthropic não permite que produtos de terceiros ofereçam login claude.ai a seus usuários sem aprovação. Este fluxo pressupõe app pessoal do próprio assinante, na máquina dele. Distribuir o app com esse fluxo exigiria aprovação prévia.

## Validação, health e erros

- `ai.Configured` continua **puro** (hot path do orchestrator): `claude_code_token != ""`.
- `/api/ai/health` (Ping), três estados: binário ausente → `ErrNotConfigured` ("Claude Code não instalado", com instrução de instalação); token vazio → `ErrNotConfigured` ("não conectado"); ambos presentes → `claude --version` confirma o binário e responde `valid:true` **sem** validar o token (mesma semântica frouxa que o health do openai tinha; anotada como limitação conhecida).
- **"Testar conexão"** faz a validação real: um `claude -p` mínimo, distinguindo sucesso, falha de auth e falha de ambiente pela saída/exit code.
- Mapeamento para os sentinels existentes: token inválido/expirado → `ErrAIAuthFailed` (→ 502, mensagem "reconecte com Claude"); binário sumiu → `ErrNotConfigured` (→ 503); demais exit codes ≠ 0 → erro genérico (→ 502) com stderr na mensagem.
- Rate limit da assinatura estourado → erro genérico com a mensagem original do CLI preservada; **sem retry automático** (não queimar quota sozinho).
- Degradação graciosa preservada (decisão 2026-06-05): `auto_generate=true` sem configuração pula a geração e completa a reunião com a transcrição; Reprocessar explícito falha com erro claro.

## Migração e settings

- Migration `018_claude_code_provider.sql`: `ai_provider = 'claude-code'`; apaga `anthropic_api_key`, `anthropic_model`, `openai_api_key`, `openai_model` (chaves de API não sobram no banco quando nada mais as lê).
- Whitelist do `SettingsService`: entram `claude_code_token` e `claude_code_model`; saem as chaves antigas.
- **Consequência (padrão da migration 016):** downgrade impossível — banco migrado não volta a funcionar numa v2.7.x; chaves apagadas não são recuperáveis (recolar se voltar).

## Modelo

Seletor nas Configurações (`claude_code_model`) mapeado para `--model`; vazio = padrão da subscription (sem `--model`). **Decisão 2026-08-29 (mid-implementação):** o usuário pediu modelos listados "direto do Claude"; verificado que não há listagem dinâmica oficial com credencial de subscription (sem `claude models list`; `GET /v1/models` recusa token de subscription por ToS). Resolução: seletor com os aliases documentados (padrão/haiku/sonnet/opus) mais opção "outro…" com campo livre aceitando qualquer id de modelo — a whitelist do backend valida `claude_code_model` como texto livre, não enum. Modelos novos ficam usáveis sem atualizar o app.

## Testes

- Unit `internal/ai` via `commandRunner` fake: parse do JSON (feliz, `result` malformado, usage ausente), mapeamento de erros (exit code, auth, binário ausente), timeout.
- Handler do login com runner fake: captura, timeout, spawn duplicado.
- `dynamic_client_test.go` adaptado ao provider único.
- Frontend: `tsc --noEmit` + `npm run build` + exercício manual na janela nativa (reiniciar o `wails dev` — armadilha conhecida).
- Validação real do fluxo de login: spike (Task 1) + teste manual do usuário.

## Registro

- `DECISIONS.md`: decisão transversal "IA via subscription/Claude Code CLI, provider único — API keys removidas", com o trade-off de distribuição.
- BACKLOG: o débito "validação de chave OpenAI é só existência" morre com o provider; vira a nota da semântica frouxa do health do claude-code.
