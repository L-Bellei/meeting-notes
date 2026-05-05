# Estado do Projeto — 2026-05-05

## Sessão
- **Data:** 2026-05-05
- **Branch atual:** `master` (pós-merge PR #28 + PR #29 chore/bump-version-2.2.4)
- **Worktree:** nenhum ativo

## Trabalho desta sessão

**Fix single-instance + Release v2.2.4**

1. **Fix: segunda instância trava ao reabrir pelo atalho** (`fix/single-instance-lock`, PR #28, mergeado)
   - Problema: ao fechar a janela e reabrir pelo atalho do Windows, novo processo Wails colidia com a instância em background (SQLite lock, tray duplicado, porta HTTP diferente)
   - Fix: `options.SingleInstanceLock` do Wails v2 — segunda instância detecta a primeira via named pipe, traz janela ao primeiro plano, encerra limpa
   - Arquivos: `cmd/desktop/main.go`, `cmd/desktop/app.go`

2. **Checagem de auto-suficiência**
   - Go binary: 100% estático, sem CGO, sem DLLs externas
   - Audio service: PyInstaller onedir bundled no installer
   - WebView2: bootstrapper incluído (Windows 11 nativo, Windows 10 faz download ~100 MB)
   - Resultado: app completamente auto-suficiente na instalação

3. **Release v2.2.4** — installer publicado no GitHub

## Estado do repositório
- Repositório público: `https://github.com/L-Bellei/meeting-notes`
- Branch `master` protegida (PRs obrigatórios)
- PRs mergeados esta sessão: #28 (single-instance fix), #29 (bump 2.2.4)
- Release publicada: `v2.2.4`
- Versão atual: `2.2.4`

## Fase Superpowers
N/A nesta sessão — bugfix direto. Nenhum plano ativo em execução.

## Próximo passo
Ver `BACKLOG.md` para próximas features. Iniciar nova feature com `superpowers:brainstorming`.

## Worktrees paralelos
Nenhum.
