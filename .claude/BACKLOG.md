# Backlog

Itens fora do escopo das features já implementadas. Para features com plano ativo do Superpowers, apenas referência — sem duplicação de conteúdo.

---

## Features com plano pronto (não iniciadas)

- _(nenhuma no momento — loading screen concluída em 2026-06-05; guard de IA em review na PR #32)_

---

## Features futuras (não brainstormadas)

- **Prompts separados por tipo de geração** — `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt` no modelo Theme, em vez de um único `custom_prompt`. Ver decisão em DECISIONS.md.
- **Export** — exportar reunião (ou card do board) em PDF, Markdown ou Notion.
- **Busca global** — busca full-text por transcrição, resumo e tasks em todas as reuniões.
- **Notificações de pipeline** — notificação nativa do sistema operacional quando o processamento de uma reunião termina.
- **Log persistido de erros** — tabela `app_logs` no banco para erros de pipeline e falhas de inicialização.

---

## Débitos técnicos

- **Chunk size warning no build do frontend** — bundle JS de ~518 kB. Considerar code-splitting com `React.lazy` para BoardView e modais pesados.
- **`frontend/dist/` duplicado** — existe `frontend/dist/` com arquivos obsoletos além do `cmd/desktop/frontend/dist/` que é o correto. Remover o diretório obsoleto (já no `.gitignore`, mas o diretório físico ainda existe).
- **Silero VAD no PyInstaller** — `vad_filter=True` foi removido por falhar no bundle. Se quisermos reativar no futuro, os dados do modelo Silero precisam ser adicionados explicitamente ao `.spec`.
- **Validação de chave OpenAI é só existência** — `ai.Ping`/`Configured` para `openai` apenas checa se a chave não é vazia (não faz chamada à API). `/api/ai/health` retorna `valid:true` sem validar de fato. Implementar um ping real à API da OpenAI (TODO em `internal/ai/validate.go`).

---

## Bugs conhecidos (sem plano)

- **Whisper "auto" language não é detecção automática real** — quando `whisper_language = "auto"`, o orchestrator passa `""` para o audio-service, que por sua vez faz `language or default_language` em Python. Se `default_language = "pt"`, o áudio sempre é transcrito como português. Corrigir: remover o `default_language` fallback no `transcriber.py` ou garantir que `""` seja tratado como None pelo faster-whisper.
