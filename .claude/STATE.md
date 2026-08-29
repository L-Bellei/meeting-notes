# Estado do Projeto — 2026-08-29

## Sessão
- **Data:** 2026-08-28/29
- **`master` (`40c997e`) = v2.8.1 publicada.** Três releases nesta sessão: **v2.7.1** (hotfix — o
  audio-service empacotado das v2.6.0/v2.7.0 morria no boot; bundle regenerado do venv pinado e
  smoke test obrigatório no `build.ps1`), **v2.8.0** (IA passa a consumir a **assinatura Claude**
  via Claude Code headless; providers por API key removidos) e **v2.8.1** (ícones do produto
  restaurados e rastreados no git). Detalhes por release no CHANGELOG desta data.
- **Worktree:** nenhum ativo.

## Fase Superpowers

**Brainstorming (iniciando)** — próxima feature: **GPU/CPU na transcrição** (o app escaneia a
máquina e o usuário escolhe o device). O brainstorm foi aberto em 2026-08-28 e pausado no primeiro
passo; contexto de partida: decisão CPU-only de 2026-08-21 em DECISIONS.md e o item medido
"Instalador transcreve em CPU — empacotar GPU" no BACKLOG (ganho 3,2×, +465 MB de DLLs, timeout de
60 min em `internal/audio/client.go`, fallback GPU→CPU permanente até restart). Primeira decisão de
design em aberto: como entregar as DLLs de CUDA (instalador único ~610 MB vs download sob demanda
vs dois instaladores).

O ciclo anterior (`docs/superpowers/plans/2026-08-29-claude-code-provider.md`, spec em
`docs/superpowers/specs/2026-08-29-claude-code-provider-design.md`) foi completo: brainstorm → spec
→ plano → 7 tasks via Subagent-Driven Development (com spike de TTY que mudou o design do login) →
revisão final → merge PR #50 → release v2.8.0. Workspace do SDD apagado; o registro é o git.

## Próximo passo imediato

Retomar o brainstorm da feature GPU/CPU (`/superpowers:brainstorming`), começando pela decisão de
empacotamento das DLLs. Depois dela, o BACKLOG ainda guarda **Notificações de pipeline** e
**Export** como features acordadas/futuras.

## Estado de release

- **v2.8.1** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.8.1
  (instalador 138,4 MB; smoke test do bundle OK em 38s no build).
- **`master` está em paridade com a última release.**
- **Upgrade da v2.7.x exige reconectar a IA**: a migration 018 apaga as chaves de API do banco
  (irreversível — downgrade não funciona) e o usuário cola o token do `claude setup-token` nas
  Configurações.
- O `build.ps1` agora **falha o build** se o audio-service empacotado não responder `/health` em
  120s — a trava que faltou nas v2.6.0/v2.7.0. NSIS precisa estar no PATH
  (`C:\Program Files (x86)\NSIS`).
- **PR #48** (docs da v2.7.0) foi superado: sua única contribuição (entrada de release do CHANGELOG)
  foi portada para o master neste commit. Fechar sem merge.

## Armadilhas de ambiente

Todas no `CLAUDE.md`, seção "Rodando em dev". Novas desta sessão:
- **O bundle do PyInstaller DEVE ser gerado com o Python do `.venv`** — o global tinha uvicorn de
  outra versão sem os extras, e o bundle saiu morto (duas releases). O `.venv` foi recriado em
  2026-08-28 a partir do `requirements.txt` corrigido (pins `websockets==13.1`,
  `huggingface_hub<1.0`).
- **App instalado aberto segura o `SingleInstanceLock`** — o `wails dev` sai com exit 0 em
  silêncio; e instalar por cima com o app aberto produz instalação híbrida (débito NSIS no BACKLOG).
- Testes/gerações do provider claude-code nunca tocam o CLI real (fakes de `commandRunner`); a
  validação de verdade é o botão "Testar conexão" ou uma geração real.
