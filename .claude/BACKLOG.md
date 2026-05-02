# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features futuras (não brainstormadas)

- **Prompts separados por tipo de geração** — `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt` no modelo Theme, em vez de um único `custom_prompt`. Ver decisão em DECISIONS.md.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.
- **Notificações de pipeline** — notificação nativa do sistema operacional quando o processamento de uma reunião termina.
- **Log persistido de erros** — tabela `app_logs` no banco para erros de pipeline e falhas de inicialização.

---

## Débitos técnicos

- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **`frontend/dist/` duplicado** — existe `frontend/dist/` com arquivos obsoletos além do `cmd/desktop/frontend/dist/` que é o correto. Remover o diretório obsoleto (já no `.gitignore`, mas o diretório físico ainda existe).
- **productVersion no wails.json** — atualizado manualmente a cada release (v2.2.3 atual). Considerar automatizar via `build.ps1`.
- **`makensis` fora do PATH** — em novas sessões de terminal, adicionar `/c/Program Files (x86)/NSIS` ao PATH antes de `wails build -nsis`. Ver DECISIONS.md.

---

## Bugs conhecidos (sem plano)

- Nenhum no momento.

---

## Planos escritos aguardando execução

- **Loading screen** — spec + plano escritos em 2026-05-02, mas a implementação **já está completa** no código. O plano `docs/superpowers/plans/2026-05-02-loading-screen.md` está **untracked** e é considerado obsoleto. Não executar.
  - Spec: `docs/superpowers/specs/2026-05-02-loading-screen-design.md`
  - Plano: `docs/superpowers/plans/2026-05-02-loading-screen.md` (untracked)
