# Changelog de Sessões

---

## [2026-04-29] Kanban Board — v2.0.0

**Feature:** Global Kanban Board com drag-and-drop, colunas configuráveis, CardDetailModal, filtros e auto-add por tema.

**Plano Superpowers:** `C:\Users\leo_b\.claude\plans\functional-honking-moler.md` (7 tasks, todas concluídas)

**Spec:** `docs/superpowers/specs/2026-04-29-kanban-board-design.md`

**Fase final:** `finishing` — mergeado em `master`, tag `v2.0.0` criada, installer gerado em `dist/meeting-notes-2.0.0-windows-amd64-installer.exe`.

**O que foi entregue:**
- Migration 007: tabelas `board_columns` (seed: Backlog / Em Andamento / Concluído) e `board_cards`
- Repositories: `BoardColumnRepository`, `BoardCardRepository` com rebalanceamento automático de posições
- Services: `BoardColumnService`, `BoardCardService`
- Handler: `BoardHandler` com rotas CRUD de colunas e cards + PATCH `/move`
- Frontend: `BoardView`, `KanbanColumn`, `KanbanCard` (drag-and-drop @dnd-kit), `CardDetailModal`, `BoardFilters`, `ColumnSettingsPanel`
- Hook: `useBoard.ts`, `useBoardColumns.ts`
- Navegação: botão "Board" na Toolbar, `activeView` state em App.tsx
- MeetingDetail: botão "Adicionar ao Board" + badge de card existente
- Theme: campo `auto_add_to_board` + hook no orchestrator para auto-criar card após processamento

**Decisões transversais registradas em DECISIONS.md:**
- Float positions + rebalanceamento
- customPrompt campo único
- Board global, numeração imutável
- Seed de colunas padrão com IDs fixos
- Processo de build do installer (NSIS path)

**Bloqueios encontrados:**
- `makensis` não estava no PATH do bash; resolvido adicionando `/c/Program Files (x86)/NSIS` temporariamente
- Primeiro installer copiado para dist era de build anterior (sem o board); corrigido após identificar a causa
