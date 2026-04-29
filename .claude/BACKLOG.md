# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features futuras (não brainstormadas)

- **Prompts separados por tipo de geração** — `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt` no modelo Theme, em vez de um único `custom_prompt`. Ver decisão em DECISIONS.md.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.
- **Busca global** — busca full-text por transcrição, resumo e tasks em todas as reuniões.
- **Notificações de pipeline** — notificação nativa do sistema operacional quando o processamento de uma reunião termina.
- **Atalho de teclado para gravar** — iniciar/parar gravação sem abrir a janela.

---

## Débitos técnicos

- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **`cmd/desktop/build/` e `cmd/desktop/frontend/dist/` não estão no `.gitignore`** — esses artefatos de build aparecem como untracked. Adicionar ao `.gitignore`.
- **`frontend/dist/` duplicado** — existe `frontend/dist/` com arquivos obsoletos além do `cmd/desktop/frontend/dist/` que é o correto. Remover o diretório obsoleto.
- **productVersion no wails.json** — atualmente `2.0.0`, deve ser atualizado a cada release. Considerar automatizar via script ou Makefile.

---

## Bugs conhecidos (sem plano)

- Nenhum no momento.
