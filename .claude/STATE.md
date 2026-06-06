# Estado do Projeto — 2026-06-05

## Sessão
- **Data:** 2026-06-05
- **Branch atual:** `feat/ai-config-guard-and-resilience` (pushed, PR #32 aberta)
- **Worktree:** nenhum ativo

## Trabalho desta sessão

1. **Fix do audio-service em dev** (commit `f4aad63`): o app usava o bundle de release stale em `build/bin/` em vez do uvicorn; `findAudioServiceDir` achava o diretório errado. Corrigido com skip do `-dev.exe` + exigência de `main.py`.

2. **Guard de IA não-configurada + avisos/contramedidas** (commit `f1e673b`, feature principal):
   - Desabilita UI dependente de IA (geração, reprocessar, custom_prompt, auto_generate) com banner/tooltip quando não há provider/chave.
   - Pipeline degrada graciosamente (preserva transcrição, status `completed`) em vez de marcar `FAILED`.
   - Sentinels de erro (`ai.ErrNotConfigured`, `ErrAIAuthFailed`) → mensagens HTTP claras; corrige caminho 503 que era código morto.
   - Resiliência do audio-service mid-session (`monitorAudioHealth` + `useAudioStatus`).

## Fase Superpowers

**Finishing** — porém esta feature foi conduzida via **plan-mode ad-hoc**, não pelo fluxo brainstorm→plan do Superpowers. Não há plano Superpowers correspondente em `docs/superpowers/plans/`. O plano de execução ficou em `~/.claude/plans/agora-fa-a-uma-analise-*.md` (artefato do plan-mode, fora do repo).

## Próximo passo imediato

- Revisar/mergear **PR #32**: https://github.com/L-Bellei/meeting-notes/pull/32
- Após merge, `master` recebe o fix do audio-service + a feature de guard de IA.

## Pendências menores

- Processo `uvicorn` órfão pode ter ficado da sessão de testes (relançado manualmente na porta 8765). Reaproveitado no próximo start do app; reiniciar o `wails dev` para estado limpo.

## Features com plano Superpowers

- **Loading screen** (`docs/superpowers/plans/2026-05-02-loading-screen.md`): verificada como **completa** nesta sessão (já estava implementada; validada via wails dev). Pode ser marcada como entregue.

## Worktrees paralelos

Nenhum.

## Estado de release

- **v2.3.0** publicada. A feature de guard de IA ainda não foi versionada/empacotada (aguardando merge da PR #32).
