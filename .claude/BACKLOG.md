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

- **Audio-service não incluído automaticamente no build** — `build.ps1` usa `-clean`, que limpa `cmd/desktop/build/bin/`. O bundle PyInstaller (`audio-service/build/dist/audio-service/`) precisa ser copiado manualmente para `build/bin/audio-service/` antes do NSIS. Automatizar adicionando etapa de cópia no `build.ps1` após o `wails build`.
- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **`frontend/dist/` duplicado** — existe `frontend/dist/` com arquivos obsoletos além do `cmd/desktop/frontend/dist/` que é o correto. Remover o diretório obsoleto (já no `.gitignore`, mas o diretório físico ainda existe).
- **`makensis` fora do PATH** — em novas sessões de terminal, adicionar `C:\Program Files (x86)\NSIS` ao PATH antes de rodar NSIS diretamente.

---

## Bugs conhecidos (sem plano)

- Nenhum no momento.
