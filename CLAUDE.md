# Meeting Notes — Guia para o Agente

## O que é este projeto
Aplicação desktop local (Wails v2) que grava reuniões, transcreve com Whisper e gera resumo, pontos-chave e tasks pela assinatura Claude (Claude Code headless). Single-user, sem autenticação.

## Stack
- **Backend:** Go 1.22+, chi v5, modernc/sqlite (sem CGO), uuid
- **Desktop:** Wails v2 (WebView2 no Windows)
- **Frontend:** React 19 + TypeScript, Tailwind CSS, React Query v5, @dnd-kit
- **AI:** Claude via subscription — Claude Code headless (`internal/ai/claude_code_client.go`)
- **Áudio:** componente Python separado (Whisper + loopback de sistema)

## Arquitetura
```
cmd/
  api/          → servidor HTTP standalone
  desktop/      → entry point Wails
internal/
  ai/           → cliente Claude Code (subscription)
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

## Rodando em dev

`wails dev` a partir de `cmd/desktop`. Três armadilhas que já custaram tempo:
- **O watcher só observa `cmd/desktop`.** Mudança em `internal/**` não rebuilda o Go — o app segue com o binário antigo, e depois de uma migration nova isso aparece como erro 500 de código velho contra banco já migrado. Reinicie o `wails dev` após mexer no backend.
- **O HMR do vite não chega à janela nativa.** Mudança em `frontend/**` é aplicada na aba do navegador em `localhost:34115`, mas a janela do WebView2 continua com o código carregado no boot. Reinicie o `wails dev` após mexer no frontend também. O log do `wails dev` mostra `hmr update <arquivo>` de qualquer forma — isso é o vite *emitindo*, não o webview *aplicando*, e não serve como prova de que a janela nativa atualizou. Custou uma correção reportada como "não funcionou" quando o código já estava certo (2026-08-22).
- **`SingleInstanceLock`**: subir um segundo `wails dev` com o primeiro rodando faz o novo sair com exit 0, silenciosamente. Mate o processo antes de reiniciar.

Em dev o audio-service sobe via `audio-service/.venv/Scripts/uvicorn.exe`; sem esse venv o auto-start é pulado (o app funciona, gravação não).

## Build do installer (Windows)
Use **sempre** o `build.ps1` na raiz — ele é o caminho canônico. Não rode `wails build -nsis` direto: o `-nsis` empacota o conteúdo de `build/bin`, e sem a etapa de cópia do bundle PyInstaller o instalador sai **sem o audio-service**. O `build.ps1` faz `wails build -clean`, copia `audio-service/build/dist/audio-service` para `build/bin`, roda o NSIS e coleta o artefato em `dist/`.

```powershell
# (NSIS precisa estar no PATH; o script roda os testes Go antes)
.\build.ps1                 # usa productVersion do wails.json
.\build.ps1 -Version 2.4.0  # também persiste a versão no wails.json
```
Pré-requisitos: bundle do audio-service em `audio-service/build/dist/audio-service`. Se ausente, gerar com o spec **rastreado no git** (`audio-service/build/pyinstaller/audio-service.spec` — o resto de `audio-service/build/` é ignorado de propósito):

O bundle **precisa** ser gerado com o Python do `.venv` (criado a partir de `requirements.txt`) — o bundle da v2.6.0/v2.7.0 saiu morto porque foi gerado com o Python global, que tinha um uvicorn de outra versão e sem os extras de `uvicorn[standard]`.

```powershell
cd audio-service
.venv\Scripts\python.exe -m PyInstaller build\pyinstaller\audio-service.spec --distpath build\dist --workpath build\work --noconfirm
```

O bundle embarca CUDA desde 2026-08-29 (ver DECISIONS.md); o device é escolha do usuário nas Configurações (`whisper_device`), com fallback GPU→CPU por chamada. GPU não-NVIDIA usa whisper.cpp/Vulkan (DECISIONS 2026-08-30). Atualizar `productVersion` em `cmd/desktop/wails.json` (ou passar `-Version`) antes de cada release.

Segundo pré-requisito desde 2026-08-30: o binário do whisper.cpp (Vulkan) em `audio-service/vendor/whispercpp/` — compilado com `.\audio-service\build\fetch-whispercpp.ps1` (versão pinada em `audio-service/build/whispercpp.version`; exige CMake, VS Build Tools 2022 C++ e Vulkan SDK). O `.spec` e o `build.ps1` abortam sem ele.

Release completa: bump de versão (via PR — `master` é protegido) → `build.ps1` → tag `vX.Y.Z` → GitHub Release com o `.exe` de `dist/`.
