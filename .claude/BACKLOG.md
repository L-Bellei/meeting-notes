# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features futuras (não brainstormadas)

- **Prompts separados por tipo de geração** — `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt` no modelo Theme, em vez de um único `custom_prompt`. Ver decisão em DECISIONS.md.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.
- **Busca global** — busca full-text por transcrição, resumo e tasks em todas as reuniões.
- **Notificações de pipeline** — notificação nativa do sistema operacional quando o processamento de uma reunião termina.

---

## Débitos técnicos

- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **`frontend/dist/` duplicado** — existe `frontend/dist/` com arquivos obsoletos além do `cmd/desktop/frontend/dist/` que é o correto. Remover o diretório obsoleto (já no `.gitignore`, mas o diretório físico ainda existe).
- **productVersion no wails.json** — atualmente `2.0.0`, deve ser atualizado a cada release. Considerar automatizar via `build.ps1`.
- **`makensis` fora do PATH** — em novas sessões de terminal, adicionar `/c/Program Files (x86)/NSIS` ao PATH antes de `wails build -nsis`. Ver DECISIONS.md.

---

## Bugs conhecidos (sem plano)

- Nenhum no momento.
