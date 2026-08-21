# Temas: prompt único + overhaul da aba de temas — Design

**Data:** 2026-08-20
**Tipo:** Feature + revert (reverte a estrutura de prompts por tipo entregue na v2.5.0, restaurando a decisão de 2026-04-29 "customPrompt campo único")

## Problema

Dois problemas independentes, mas que colidem no mesmo componente (`ThemeEditModal`) e por isso entram numa spec só:

1. **Prompts por tipo não se pagaram.** A v2.5.0 dividiu o prompt do tema em geral + 3 overrides (resumo / pontos-chave / tarefas). Na prática nenhum override foi usado: consulta ao banco de produção mostra os 3 campos **vazios em todos os 4 temas**; só um tema tem prompt geral (1089 chars). O resultado é um modal com 4 textareas onde 1 basta, mais `PromptFor`/`ThemePrompts` carregando complexidade que ninguém exercita.

2. **A aba de temas atrapalha em vez de ajudar.** A sidebar é uma gaveta que **fecha a cada seleção de tema** (`selectAndClose`), então trocar de filtro custa sempre abrir → clicar → fechou. Com a gaveta fechada **nada na tela indica que há filtro ativo** — dá a impressão de que reuniões desapareceram. As ações da linha (nova subcategoria / editar / excluir) estão escondidas atrás de `hover` em ícones de 11px, inalcançáveis por teclado. Excluir é "clique duas vezes" sem confirmação escrita. A cor do tema é um ponto de 8px e não comunica nada. Não há como saber, olhando a lista, quais temas têm prompt customizado ou auto-add-to-board. A hierarquia existe no modelo mas é imexível: não há como mover um tema para dentro de outro.

## Contexto atual

**Backend**
- `internal/models/models.go` → `Theme{CustomPrompt, CustomSummaryPrompt, CustomKeyPointsPrompt, CustomTasksPrompt}`, `PromptKind`, `(*Theme).PromptFor`, `ThemePrompts{General,Summary,KeyPoints,Tasks}`.
- `internal/services/theme_service.go` → `Create/Update(ctx, id, name, description, color, parentID, prompts models.ThemePrompts, autoAddToBoard)`. **Sem nenhuma validação de `parent_id`**: `Update` faz `t.ParentID = parentID` cru (auto-referência e ciclo são alcançáveis pela API hoje).
- Call sites de geração resolvem por tipo via `PromptFor`: `internal/handlers/{summary,key_point,task}_handler.go` e `internal/services/orchestrator.go` (`runAIGeneration`).
- `internal/ai` → `buildInstruction(default, customPrompt)` já faz o degrau `"" → default`. **Assinaturas dos AI clients não mencionam tipo de prompt** — nada a mudar em `internal/ai`.
- `settings` é tabela key-value (`key TEXT PRIMARY KEY, value TEXT`), com whitelist em `internal/services/settings_service.go` (`validSettings`). Adicionar preferência = 1 entrada na whitelist + seed via migration (padrão de `012_keep_audio_setting.sql`). Não altera schema.
- `internal/database/database.go` → `SetMaxOpenConns(1)` + `PRAGMA foreign_keys = ON`, então as FKs valem de fato para todas as queries.
- `meetings.theme_id` e `themes.parent_id` são `ON DELETE SET NULL`: excluir tema deixa as reuniões sem tema e promove subcategorias para raiz.
- Última migration: `015_theme_type_prompts.sql` → próxima é `016`.

**Frontend**
- `frontend/src/components/layout/Sidebar.tsx` (212 linhas) — gaveta `fixed` + backdrop com blur; `selectAndClose` fecha ao selecionar; linha é `<div onClick>` com `<button>`s aninhados; ações em `hidden group-hover:flex`; excluir por duplo clique; cor como dot `w-2 h-2`; nome em `text-muted-foreground` mesmo quando selecionado; `expanded` em `useState` (não sobrevive a restart); criação inline só com nome e cor hardcoded `#7c3aed`.
- `countForTheme` soma diretos + filhos diretos — **netos não são contados**, ou seja o código já assume 2 níveis.
- `ThemeEditModal.tsx` — 4 textareas de prompt; só edita (não cria).
- `App.tsx` → `sidebarOpen` em `useState`; atalho registrado só para Ctrl+K.
- `@dnd-kit` já é dependência (usado no board). **Não existe localStorage em uso** no projeto. **Não existe infra de teste no frontend** (sem vitest, sem testing-library).
- Board não referencia tema em lugar nenhum (`BoardView`/`BoardFilters`).

## Design

### 1. Prompt único (revert)

**Migration `016_theme_single_prompt.sql`:**
- `ALTER TABLE themes DROP COLUMN custom_summary_prompt;` (idem `custom_key_points_prompt`, `custom_tasks_prompt`) — suportado: `modernc.org/sqlite v1.50.0` embute SQLite ≥ 3.49.
- `INSERT OR IGNORE INTO settings (key, value) VALUES ('sidebar_pinned', 'false');`

**Go:**
- Saem de `models`: os 3 campos, `PromptKind`, `PromptFor`, `ThemePrompts`.
- `ThemeService.Create/Update` recebem `customPrompt string` no lugar de `prompts models.ThemePrompts`. **Não** reintroduzir 4 strings posicionais — era o motivo de `ThemePrompts` existir; com um prompt só, um parâmetro resolve.
- Request structs de `theme_handler.go` perdem os 3 campos JSON.
- Os 4 call sites de geração voltam a passar `theme.CustomPrompt`.
- `internal/ai` intocado.

**Frontend:** tipo `Theme` e payload de update perdem os 3 campos; modal fica com um textarea (reescrito na seção 4).

**Testes:** `internal/models/theme_prompt_test.go` deletado; testes de service/handler/orchestrator que fixam prompt por tipo voltam ao prompt único; round-trip do repository sem as 3 colunas.

**Sem perda de dado** — os 3 campos estão vazios no banco real.

### 2. Pin e comportamento da sidebar

**Estado:** `sidebar_pinned` em `settings`, default `"false"` (comportamento atual preservado; pin é opt-in), validado como enum `true`/`false` na whitelist.

**Modo pinado:** a sidebar sai de `fixed`/`translate-x` e entra no flex row como coluna de 256px. Sem backdrop, sem blur, sem animação de gaveta; `MeetingList`/`MeetingDetail` reflowam ao lado.

**Modo gaveta:** overlay como hoje, **menos o auto-close** — `selectAndClose` vira `onSelectTheme`. Fecha por Esc, clique no backdrop, hamburger ou X.

**Controles:** botão de pino no header da sidebar alterna dockado/gaveta (persiste em settings). O hamburger da Toolbar alterna visibilidade nos dois modos — pinada + hamburger esconde **sem despinar** (reabre dockada). `Ctrl+B` alterna, registrado junto do Ctrl+K em `App.tsx`.

**Filtro visível:** chip `Tema: <nome> ×` no header do `MeetingList`, presente sempre que `selectedThemeId != null`; o `×` limpa o filtro.

**Board:** ao entrar na view Board a sidebar recolhe e, ao voltar para Reuniões, retoma o estado anterior — o Board não filtra por tema, uma coluna inerte ali seria enganosa.

### 3. Anatomia da linha do tema

Três botões **irmãos** num flex (chevron | linha | menu `⋯`), substituindo `<div onClick>` com botões aninhados — corrige acessibilidade na mesma reescrita.

- **Cor:** barra vertical de 3px na borda esquerda, altura da linha (substitui o dot de 8px).
- **Badges** (11px, com `title`, só quando ativos): prompt customizado (`custom_prompt != ""`) e auto-add-to-board.
- **Contagem:** mantida, com contraste melhor e `tabular-nums`.
- **Menu `⋯`** sempre visível → Nova subcategoria / Editar / Excluir. Popover via `createPortal` ancorado por `getBoundingClientRect` (a linha vive dentro de um `overflow-y-auto`; convenção do DECISIONS 2026-05-07).
- **Selecionado:** nome em `text-foreground` (hoje fica `text-muted-foreground` sempre).

### 4. Hierarquia de 2 níveis + drag-and-drop

`DndContext` na lista (dep `@dnd-kit` já existe); cada linha é draggable e droppable; faixa "mover para raiz" no topo para despromover.

**Regras, validadas em `ThemeService.Update`** (violação → `ValidationError` → 422):
1. `*parentID != id` — nada é pai de si mesmo.
2. O pai escolhido não pode ter `parent_id` — teto de 2 níveis.
3. Um tema que tem filhos não pode virar filho.

O frontend também bloqueia o drop inválido (feedback `not-allowed`), sem depender do 422 para comunicar.

Com o teto de 2 níveis, `countForTheme` (que ignora netos) fica **correto por construção** — sem recursão.

**Expansão:** `localStorage["theme_expanded"]` = array de IDs, filtrado contra os temas existentes na leitura (IDs órfãos não acumulam). Primeiro uso de localStorage no projeto → decisão transversal a registrar no DECISIONS.md.

### 5. Criar, editar, excluir

**Modal unificado:** `ThemeEditModal` aceita `mode: "create" | "edit"` + `parentId` opcional. Criar tema ou subcategoria abre o form completo (nome, descrição, cor, prompt, auto-board); o input inline de criação desaparece.
**Tradeoff aceito:** perde-se o "criar rápido digitando só o nome"; em troca há um caminho só, e a cor deixa de ser `#7c3aed` hardcoded.

**Excluir:** confirmação inline explícita substituindo o duplo clique, declarando o efeito real — *"Excluir 'X'? As N reuniões continuam, sem tema. Subcategorias sobem para a raiz."* — com Cancelar / Excluir. O texto é verdade verificada (FKs `ON DELETE SET NULL` + pragma efetivo).

## Testes e verificação

**Go (TDD — é onde está a lógica):**
- Validação de parent: auto-referência, pai que já tem pai, mover tema-com-filhos → 3 casos.
- Migration 016: colunas por tipo somem, `custom_prompt` sobrevive.
- Repository round-trip sem os 3 campos.
- Settings: `sidebar_pinned` aceito no enum, valor fora do enum rejeitado.

**Frontend:** sem infra de teste no projeto — verificação é `npx tsc --noEmit` + `npm run build` + validação ao vivo (`wails dev`). Introduzir vitest fica como item de backlog, fora desta spec.

## Fora de escopo

- Notificações de pipeline (spec própria, já acordada como próximo trabalho).
- Filtro por tema no Board.
- Introdução de vitest no frontend.
- Demais itens do backlog (export, code-splitting, ping real da chave OpenAI, Silero VAD).

## Riscos

- **`DROP COLUMN` é irreversível** e migrations rodam automaticamente ao abrir o banco: depois da 016 o mesmo `.db` não volta a funcionar numa v2.5.0 — downgrade deixa de ser possível. Não há perda de conteúdo (campos vazios), mas convém copiar o `.db` antes do primeiro run.
- **dnd-kit dentro de container com `overflow-y-auto`** exige atenção a sensores/modifiers; há precedente no board para seguir.
- **Reescrita concentrada:** `Sidebar.tsx` é reescrita quase inteira. Mitigação: dividir em componentes menores e focados (linha, menu, faixa de raiz, header) em vez de um arquivo maior.
