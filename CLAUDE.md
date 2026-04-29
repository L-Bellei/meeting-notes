# Meeting Notes — Guia para o Agente

## O que é este projeto
Aplicação desktop local (Wails v2) que grava reuniões, transcreve com Whisper e gera resumo, pontos-chave e tasks via Claude API. Single-user, sem autenticação.

## Stack
- **Backend:** Go 1.22+, chi v5, modernc/sqlite (sem CGO), uuid
- **Desktop:** Wails v2 (WebView2 no Windows)
- **Frontend:** React 19 + TypeScript, Tailwind CSS, React Query v5, @dnd-kit
- **AI:** Anthropic Claude Sonnet 4.6 (via `internal/ai/anthropic_client.go`)
- **Áudio:** componente Python separado (Whisper + loopback de sistema)

## Arquitetura
```
cmd/
  api/          → servidor HTTP standalone
  desktop/      → entry point Wails
internal/
  ai/           → clientes Anthropic/OpenAI
  audio/        → cliente para o serviço Python de áudio
  database/     → abertura do SQLite + migrations automáticas
  handlers/     → handlers HTTP (chi)
  models/       → structs de domínio
  repository/   → acesso ao SQLite
  services/     → lógica de negócio + Orchestrator
frontend/src/
  components/   → React components
  hooks/        → React Query hooks
```

## Convenções
- Sem comentários no código, salvo quando o WHY é não-óbvio
- Sem mocks em testes de repositório — usar SQLite em memória via `t.TempDir()`
- Dois entry points (`cmd/api` e `cmd/desktop`) devem permanecer em sincronia
- Migrations são embed e aplicadas automaticamente ao abrir o banco

## Workflow com Superpowers
Este projeto usa o plugin **Superpowers**. Ao iniciar qualquer nova feature:
1. Verificar `.claude/STATE.md` e `.claude/BACKLOG.md` para contexto
2. Seguir o workflow: **brainstorm → plan → implement → review → finish**
3. Não duplicar conteúdo dos planos do Superpowers nos arquivos `.claude/`

Specs em: `docs/superpowers/specs/`
Planos em: `docs/superpowers/plans/`

## Build do installer (Windows)
```bash
cd cmd/desktop
PATH="$PATH:/c/Program Files (x86)/NSIS" wails build -nsis
cp "build/bin/Meeting Notes-amd64-installer.exe" "../../dist/meeting-notes-X.Y.Z-windows-amd64-installer.exe"
```
Atualizar `productVersion` em `cmd/desktop/wails.json` antes de cada release.
