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
Use **sempre** o `build.ps1` na raiz — ele é o caminho canônico. Não rode `wails build -nsis` direto: o `-nsis` empacota o conteúdo de `build/bin`, e sem a etapa de cópia do bundle PyInstaller o instalador sai **sem o audio-service**. O `build.ps1` faz `wails build -clean`, copia `audio-service/build/dist/audio-service` para `build/bin`, roda o NSIS e coleta o artefato em `dist/`.

```powershell
# (NSIS precisa estar no PATH; o script roda os testes Go antes)
.\build.ps1                 # usa productVersion do wails.json
.\build.ps1 -Version 2.4.0  # também persiste a versão no wails.json
```
Pré-requisitos: bundle do audio-service em `audio-service/build/dist/audio-service` (gerar com PyInstaller se ausente — o script avisa). Atualizar `productVersion` em `cmd/desktop/wails.json` (ou passar `-Version`) antes de cada release.

Release completa: bump de versão (via PR — `master` é protegido) → `build.ps1` → tag `vX.Y.Z` → GitHub Release com o `.exe` de `dist/`.
