# Estado do Projeto — 2026-08-29 (fim de sessão)

## Sessão
- **Data:** 2026-08-28/29 (sessão longa — quatro releases)
- **`master` (`7d5d872`) = v2.9.0 publicada.** Nesta sessão: **v2.7.1** (hotfix — audio-service
  empacotado morria no boot; smoke test obrigatório no `build.ps1`), **v2.8.0** (IA pela
  **assinatura Claude** via Claude Code headless; API keys removidas), **v2.8.1** (ícones do
  produto restaurados e rastreados) e **v2.9.0** (transcrição em **GPU** com scan da máquina e
  seletor Auto/GPU/CPU). Detalhes por release no CHANGELOG desta data.
- **Worktree:** nenhum ativo.

## Fase Superpowers

**Nenhum ciclo em andamento.** O último (`docs/superpowers/plans/2026-08-29-gpu-cpu-transcription.md`,
spec `docs/superpowers/specs/2026-08-29-gpu-cpu-transcription-design.md`) foi completo: 9 tasks via
Subagent-Driven Development, experimento de corte das DLLs validado com transcrição real em CUDA,
homologação do usuário na janela nativa, merge (PRs #53/#54) e release v2.9.0. Workspaces do SDD
apagados; o registro é o git.

## Próximo passo imediato

Nenhum acordado. Candidatas no BACKLOG (Features futuras): **Notificações de pipeline** e
**Export** — ambas começam por `/superpowers:brainstorming`. O argumento antigo do backlog
(**vitest no frontend**) segue válido.

## Estado de release

- **v2.9.0** publicada: https://github.com/L-Bellei/meeting-notes/releases/tag/v2.9.0
  — instalador de **631,3 MB** (CUDA embarcada; bundle podado de 1,85→1,07 GB). A estimativa
  intermediária de ~390 MB era otimista: a razão de compressão medida numa amostra de cudnn
  (0,27) não representa a cublas, que comprime mal. 631 MB está dentro do envelope (~610 MB)
  aprovado na decisão do instalador único.
- **`master` está em paridade com a última release.**
- Upgrade da v2.8.x: migration 019 só adiciona `whisper_device` (default auto) — sem quebra;
  quem vem da v2.7.x ainda precisa reconectar a IA (migration 018, irreversível).
- Armadilha nova de release: a corrida entre `git push` do bump e `gh pr merge --delete-branch`
  deixou o bump fora do merge do PR #53 (corrigido via PR #54). Nas próximas: commitar o bump
  ANTES de abrir o PR, ou conferir `git show HEAD:cmd/desktop/wails.json` pós-merge.

## Armadilhas de ambiente

Todas no `CLAUDE.md` ("Rodando em dev") e nas entradas de 2026-08-28/29 do DECISIONS. Extras:
- Bundle do PyInstaller SEMPRE com o Python do `.venv`; a poda das DLLs de cuDNN é **pós-Analysis**
  no `.spec` (hooks do hooks-contrib re-coletam nvidia.* por fora das listas de entrada).
- Ícone "W" aparecendo em atalho/barra com o produto correto no exe = **cache de ícones do
  Windows** (limpar `iconcache_*.db` + reiniciar Explorer; atalho fixado guarda cópia própria).
  Não é bug do app — episódio de 2026-08-29 diagnosticado com o ícone extraído do binário.
- Testes Python nunca carregam WhisperModel real (suíte em ~2s); provider claude-code nunca toca
  o CLI real em teste.
