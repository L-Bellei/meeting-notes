# Temas: prompt único + overhaul da aba de temas — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reverter os prompts por tipo para um prompt único por tema e reconstruir a aba de temas (pin/dock, filtro visível, linha legível com badges e menu, hierarquia de 2 níveis com drag-and-drop, criação/exclusão claras).

**Architecture:** O revert é uma remoção atômica no Go — migration `016` derruba as 3 colunas e, no mesmo commit, modelo/repository/service/handler/call-sites voltam a `custom_prompt`; separar isso em tasks quebraria o build no meio. Depois vêm duas adições pequenas de backend (preferência `sidebar_pinned` e validação de hierarquia) e quatro tasks de frontend, que dividem o `Sidebar.tsx` atual (212 linhas fazendo tudo) em componentes focados sob `components/sidebar/`.

**Tech Stack:** Go 1.22+, chi v5, modernc/sqlite (SQLite ≥ 3.49, suporta `DROP COLUMN`), React 19 + TypeScript, Tailwind, React Query v5, `@dnd-kit` (já é dependência, usada no board), lucide-react.

**Spec:** `docs/superpowers/specs/2026-08-20-themes-single-prompt-and-sidebar-design.md`

## Global Constraints

- Sem comentários no código, salvo quando o WHY é não-óbvio (CLAUDE.md).
- Testes de repository sem mocks — SQLite real via `database.Open(t.TempDir() + "/test.db")`.
- `cmd/api` e `cmd/desktop` devem permanecer em sincronia.
- Migrations são embed e aplicadas automaticamente ao abrir o banco. Próxima é `016`.
- **Não tocar em `internal/ai`**: o degrau `"" → prompt padrão` já mora em `buildInstruction`.
- **O frontend não tem infra de teste** (sem vitest/testing-library). Verificação de frontend = `npx tsc --noEmit` + `npm run build` + checagem ao vivo. Não reportar isso como cobertura de teste.
- Textos de UI em pt-BR.
- `sidebar_pinned` default `"false"` — o comportamento atual é preservado; pin é opt-in.
- Teto de hierarquia: **2 níveis** (tema raiz → subcategoria).

---

### Task 1: Revert para prompt único (migration 016 + Go inteiro)

**Files:**
- Create: `internal/database/migrations/016_theme_single_prompt.sql`
- Delete: `internal/models/theme_prompt_test.go`
- Modify: `internal/models/models.go:5-46`
- Modify: `internal/repository/theme_repository.go:29,51-53,68,99-101,143-145`
- Modify: `internal/repository/theme_repository_test.go:215-256`
- Modify: `internal/services/theme_service.go:31-84`
- Modify: `internal/services/theme_service_test.go` (call sites + teste de prompts)
- Modify: `internal/services/orchestrator.go:301-322`
- Modify: `internal/services/orchestrator_test.go:598-624`
- Modify: `internal/handlers/theme_handler.go:23-45,59-86,102-134`
- Modify: `internal/handlers/summary_handler.go:108-113`, `key_point_handler.go:108-113`, `task_handler.go:129-134`

**Interfaces:**
- Produces: `ThemeService.Create(ctx, name, description, color string, parentID *string, customPrompt string, autoAddToBoard bool) (*models.Theme, error)` e `ThemeService.Update(ctx, id, name, description, color string, parentID *string, customPrompt string, autoAddToBoard bool) (*models.Theme, error)`. `models.Theme` fica com um único campo de prompt: `CustomPrompt string` (json `custom_prompt`). `models.ThemePrompts`, `models.PromptKind` e `(*Theme).PromptFor` deixam de existir.

- [ ] **Step 1: Criar a migration**

Criar `internal/database/migrations/016_theme_single_prompt.sql`:

```sql
ALTER TABLE themes DROP COLUMN custom_summary_prompt;
ALTER TABLE themes DROP COLUMN custom_key_points_prompt;
ALTER TABLE themes DROP COLUMN custom_tasks_prompt;

INSERT OR IGNORE INTO settings (key, value) VALUES ('sidebar_pinned', 'false');
```

- [ ] **Step 2: Rodar os testes de repository para ver a quebra**

Run: `go test ./internal/repository/ -run TestThemeRepository -v`
Expected: FAIL — as queries ainda fazem `SELECT ... custom_summary_prompt ...` numa tabela onde a coluna não existe mais (`no such column`). Essa é a falha que guia o resto da task.

- [ ] **Step 3: Deletar o teste do resolvedor por tipo**

```bash
git rm internal/models/theme_prompt_test.go
```

Esse arquivo testa exclusivamente `PromptFor`, que deixa de existir.

- [ ] **Step 4: Reescrever o round-trip do repository**

Em `internal/repository/theme_repository_test.go`, substituir `TestThemeRepository_TypePrompts_RoundTrip` (linhas 215-256) por:

```go
func TestThemeRepository_CustomPrompt_RoundTrip(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	theme := &models.Theme{
		ID:           "th-prompt",
		Name:         "Prompt",
		Color:        "#123456",
		CustomPrompt: "geral",
	}
	if err := repo.Create(ctx, theme); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(ctx, "th-prompt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CustomPrompt != "geral" {
		t.Errorf("custom prompt = %q, want %q", got.CustomPrompt, "geral")
	}

	got.CustomPrompt = "geral v2"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, err := repo.GetByID(ctx, "th-prompt")
	if err != nil {
		t.Fatalf("get reloaded: %v", err)
	}
	if reloaded.CustomPrompt != "geral v2" {
		t.Errorf("updated custom prompt = %q, want %q", reloaded.CustomPrompt, "geral v2")
	}
}
```

- [ ] **Step 5: Ajustar os testes de service**

Em `internal/services/theme_service_test.go`, trocar **todos** os `models.ThemePrompts{}` das chamadas de `Create`/`Update` por `""`. São as linhas 28, 49, 61, 72, 73, 83, 106, 107, 123, 124, 135, 149, 150. Exemplo da linha 28:

```go
theme, err := svc.Create(ctx, "Produto", "Reuniões de produto", "#8b5cf6", nil, "", false)
```

E substituir `TestThemeService_Create_PersistsTypePrompts` (linhas 164-186) por:

```go
func TestThemeService_Create_PersistsCustomPrompt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	theme, err := svc.Create(ctx, "Prompts", "", "", nil, "g", false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if theme.CustomPrompt != "g" {
		t.Errorf("custom prompt = %q, want %q", theme.CustomPrompt, "g")
	}

	updated, err := svc.Update(ctx, theme.ID, "Prompts", "", "", nil, "g2", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CustomPrompt != "g2" {
		t.Errorf("updated custom prompt = %q, want %q", updated.CustomPrompt, "g2")
	}
}
```

Se o import de `models` ficar sem uso no arquivo, removê-lo.

- [ ] **Step 6: Ajustar o teste do orchestrator**

Em `internal/services/orchestrator_test.go`, na linha 598, trocar o seed do tema e as três asserções finais (linhas ~612-624):

```go
	theme := &models.Theme{ID: "th-1", Name: "T", Color: "#111111", CustomPrompt: "GERAL"}
```

```go
	if fai.lastSummaryPrompt != "GERAL" {
		t.Errorf("summary prompt = %q, want %q", fai.lastSummaryPrompt, "GERAL")
	}
	if fai.lastKeyPointsPrompt != "GERAL" {
		t.Errorf("key points prompt = %q, want %q", fai.lastKeyPointsPrompt, "GERAL")
	}
	if fai.lastTasksPrompt != "GERAL" {
		t.Errorf("tasks prompt = %q, want %q", fai.lastTasksPrompt, "GERAL")
	}
```

Renomear o teste para `TestOrchestrator_UsesThemeCustomPrompt` se o nome atual citar "type prompts". Os campos `lastSummaryPrompt`/`lastKeyPointsPrompt`/`lastTasksPrompt` do `fakeAI` continuam existindo e sendo úteis — não removê-los.

- [ ] **Step 7: Rodar os testes para confirmar que agora quebram na compilação**

Run: `go build ./... && go test ./internal/... 2>&1 | head -30`
Expected: FAIL de compilação — os testes já usam a assinatura nova (`customPrompt string`) e o `models.ThemePrompts` ainda existe / o service ainda espera o struct.

- [ ] **Step 8: Enxugar o modelo**

Em `internal/models/models.go`, remover o struct `ThemePrompts` (linhas 5-10), os 3 campos de `Theme` (linhas 19-21) e todo o bloco `PromptKind`/constantes/`PromptFor` (linhas 25-46). O `Theme` fica:

```go
type Theme struct {
	ID             string    `json:"id"`
	ParentID       *string   `json:"parent_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Color          string    `json:"color"`
	CustomPrompt   string    `json:"custom_prompt"`
	AutoAddToBoard bool      `json:"auto_add_to_board"`
	CreatedAt      time.Time `json:"created_at"`
}
```

- [ ] **Step 9: Ajustar o repository**

Em `internal/repository/theme_repository.go`, tirar as 3 colunas das 4 queries e do scan:

- Linha 29 (`List`) e linha 68 (`GetByID`): `SELECT id, parent_id, name, description, color, custom_prompt, auto_add_to_board, created_at FROM themes ...`
- Linha 51 (`Create`): `INSERT INTO themes (id, parent_id, name, description, color, custom_prompt, auto_add_to_board, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)` — e o slice de argumentos perde `theme.CustomSummaryPrompt, theme.CustomKeyPointsPrompt, theme.CustomTasksPrompt` (linha 53).
- Linha 99 (`Update`): `UPDATE themes SET parent_id = ?, name = ?, description = ?, color = ?, custom_prompt = ?, auto_add_to_board = ? WHERE id = ?` — argumentos sem os 3 campos (linha 101).
- `scanTheme` (linhas 143-145):

```go
	if err := row.Scan(&t.ID, &parentID, &t.Name, &t.Description, &t.Color, &t.CustomPrompt,
		&t.AutoAddToBoard, &createdAt); err != nil {
		return nil, err
	}
```

Contagem de `?` tem que casar com a de colunas — errar aqui dá erro só em runtime.

- [ ] **Step 10: Ajustar o service**

Em `internal/services/theme_service.go`, as duas assinaturas passam a receber `customPrompt string`:

```go
func (s *ThemeService) Create(ctx context.Context, name, description, color string, parentID *string, customPrompt string, autoAddToBoard bool) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if color == "" {
		color = "#6366f1"
	}
	t := &models.Theme{
		ID:             uuid.New().String(),
		ParentID:       parentID,
		Name:           name,
		Description:    description,
		Color:          color,
		CustomPrompt:   customPrompt,
		AutoAddToBoard: autoAddToBoard,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
```

E em `Update`, trocar as 4 atribuições de prompt (linhas 75-78) por uma:

```go
	t.CustomPrompt = customPrompt
```

- [ ] **Step 11: Ajustar o handler de temas**

Em `internal/handlers/theme_handler.go`, os dois request structs perdem os 3 campos:

```go
type createThemeRequest struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Color          string  `json:"color"`
	ParentID       *string `json:"parent_id"`
	CustomPrompt   string  `json:"custom_prompt"`
	AutoAddToBoard bool    `json:"auto_add_to_board"`
}

type updateThemeRequest struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Color          string  `json:"color"`
	ParentID       *string `json:"parent_id"`
	CustomPrompt   string  `json:"custom_prompt"`
	AutoAddToBoard bool    `json:"auto_add_to_board"`
}
```

Remover os dois blocos `prompts := models.ThemePrompts{...}` (linhas 65-70 e 109-114) e passar `req.CustomPrompt` direto:

```go
	theme, err := h.svc.Create(r.Context(), req.Name, req.Description, req.Color, req.ParentID, req.CustomPrompt, req.AutoAddToBoard)
```

```go
	theme, err := h.svc.Update(r.Context(), id, req.Name, req.Description, req.Color, req.ParentID, req.CustomPrompt, req.AutoAddToBoard)
```

O import de `models` **continua sendo usado** neste arquivo (`[]models.Theme{}` na linha 54) — mantê-lo.

- [ ] **Step 12: Ajustar os 4 call sites de geração**

Em `internal/services/orchestrator.go`, substituir o closure `promptFor` (linhas 308-313) e as 3 chamadas:

```go
	customPrompt := ""
	if theme != nil {
		customPrompt = theme.CustomPrompt
	}
	if _, err := o.summarySvc.Generate(ctx, m, customPrompt); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	if _, err := o.keyPointSvc.Generate(ctx, m, customPrompt); err != nil {
		return fmt.Errorf("key_points: %w", err)
	}
	if _, err := o.taskSvc.Generate(ctx, m, customPrompt); err != nil {
		return fmt.Errorf("tasks: %w", err)
	}
```

Nos 3 handlers de geração, a linha de resolução passa a ser `customPrompt = theme.CustomPrompt`:

- `internal/handlers/summary_handler.go:111`
- `internal/handlers/key_point_handler.go:111`
- `internal/handlers/task_handler.go:132`

**Atenção:** nesses três arquivos o pacote `models` era usado **exclusivamente** para o `PromptKind`. Remover o import `"meeting-notes/internal/models"` dos três, senão não compila.

- [ ] **Step 13: Escrever o teste da migration**

Adicionar em `internal/database/database_test.go`:

```go
func TestOpen_ThemesHasSingleCustomPrompt(t *testing.T) {
	path := t.TempDir() + "/test.db"

	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA table_info(themes)")
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		cols[name] = true
	}

	if !cols["custom_prompt"] {
		t.Error("custom_prompt column missing")
	}
	for _, gone := range []string{"custom_summary_prompt", "custom_key_points_prompt", "custom_tasks_prompt"} {
		if cols[gone] {
			t.Errorf("column %q should have been dropped by migration 016", gone)
		}
	}
}
```

Run: `go test ./internal/database/ -run SingleCustomPrompt -v`
Expected: PASS (a migration 016 do Step 1 já derrubou as colunas). Se falhar, a migration não está sendo aplicada — conferir que o arquivo entrou no `embed` (nome no padrão `NNN_*.sql` dentro de `internal/database/migrations/`).

- [ ] **Step 14: Verificar tudo verde**

Run: `go vet ./... && go test ./... 2>&1 | tail -25`
Expected: PASS em todos os pacotes. `grep -rn "PromptFor\|ThemePrompts\|CustomSummaryPrompt" --include=*.go .` deve retornar vazio.

- [ ] **Step 15: Commit**

```bash
git add -A internal/ docs/
git commit -m "refactor: revert theme prompts to a single custom_prompt (migration 016)"
```

---

### Task 2: Preferência `sidebar_pinned`

**Files:**
- Modify: `internal/services/settings_service.go:10-23`
- Modify: `internal/services/settings_service_test.go`

**Interfaces:**
- Consumes: a migration `016` da Task 1, que já semeia `('sidebar_pinned', 'false')`.
- Produces: chave `sidebar_pinned` aceita por `PUT /api/settings` com valores `"true"`/`"false"`; qualquer outro valor → `*services.ValidationError` → HTTP 422.

- [ ] **Step 1: Escrever os testes que falham**

Adicionar em `internal/services/settings_service_test.go`:

```go
func TestSettingsService_Update_SidebarPinned(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"sidebar_pinned": "true"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := svc.GetAll(context.Background())
	if m["sidebar_pinned"] != "true" {
		t.Errorf("sidebar_pinned = %q, want true", m["sidebar_pinned"])
	}
}

func TestSettingsService_Update_InvalidSidebarPinned(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"sidebar_pinned": "yes"})
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}
```

- [ ] **Step 2: Rodar para ver falhar**

Run: `go test ./internal/services/ -run SidebarPinned -v`
Expected: FAIL no primeiro teste com `unknown setting key: "sidebar_pinned"`.

- [ ] **Step 3: Adicionar a chave à whitelist**

Em `internal/services/settings_service.go`, dentro de `validSettings`, depois de `"meeting_name_template"`:

```go
	"sidebar_pinned":         validateEnum("true", "false"),
```

- [ ] **Step 4: Rodar para ver passar**

Run: `go test ./internal/services/ -run SidebarPinned -v`
Expected: PASS nos dois.

- [ ] **Step 5: Commit**

```bash
git add internal/services/settings_service.go internal/services/settings_service_test.go
git commit -m "feat: sidebar_pinned setting"
```

---

### Task 3: Validação de hierarquia de 2 níveis

**Files:**
- Modify: `internal/services/theme_service.go` (`Create` e `Update`)
- Modify: `internal/services/theme_service_test.go`

**Interfaces:**
- Consumes: assinaturas da Task 1 (`customPrompt string`); `ThemeRepository.ChildIDs(ctx, parentID string) ([]string, error)`, que **já existe** em `internal/repository/theme_repository.go:80`.
- Produces: `Create`/`Update` retornam `*ValidationError` (→ 422 pelos handlers, que já tratam `errors.As`) quando o pai é inválido. Mensagens exatas: `"theme cannot be its own parent"`, `"parent theme cannot be a subcategory"`, `"theme with subcategories cannot become a subcategory"`, `"parent theme not found"`.

- [ ] **Step 1: Escrever os testes que falham**

Adicionar em `internal/services/theme_service_test.go`:

```go
func TestThemeService_Update_RejectsSelfAsParent(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Solo", "", "", nil, "", false)
	_, err := svc.Update(ctx, created.ID, "Solo", "", "", &created.ID, "", false)

	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Update_RejectsThreeLevels(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	root, _ := svc.Create(ctx, "Raiz", "", "", nil, "", false)
	child, _ := svc.Create(ctx, "Filho", "", "", &root.ID, "", false)
	grand, _ := svc.Create(ctx, "Neto", "", "", nil, "", false)

	_, err := svc.Update(ctx, grand.ID, "Neto", "", "", &child.ID, "", false)

	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Update_RejectsMovingParentWithChildren(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	root, _ := svc.Create(ctx, "Raiz", "", "", nil, "", false)
	_, _ = svc.Create(ctx, "Filho", "", "", &root.ID, "", false)
	other, _ := svc.Create(ctx, "Outro", "", "", nil, "", false)

	_, err := svc.Update(ctx, root.ID, "Raiz", "", "", &other.ID, "", false)

	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Update_AcceptsValidReparent(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	root, _ := svc.Create(ctx, "Raiz", "", "", nil, "", false)
	leaf, _ := svc.Create(ctx, "Folha", "", "", nil, "", false)

	updated, err := svc.Update(ctx, leaf.ID, "Folha", "", "", &root.ID, "", false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ParentID == nil || *updated.ParentID != root.ID {
		t.Errorf("parent = %v, want %q", updated.ParentID, root.ID)
	}
}

func TestThemeService_Create_RejectsSubcategoryOfSubcategory(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	root, _ := svc.Create(ctx, "Raiz", "", "", nil, "", false)
	child, _ := svc.Create(ctx, "Filho", "", "", &root.ID, "", false)

	_, err := svc.Create(ctx, "Neto", "", "", &child.ID, "", false)

	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}
```

Garantir que o arquivo importa `"errors"` e `"meeting-notes/internal/services"` (já usados nos testes existentes de duplicidade/validação).

- [ ] **Step 2: Rodar para ver falhar**

Run: `go test ./internal/services/ -run "TestThemeService_(Update_Rejects|Update_Accepts|Create_Rejects)" -v`
Expected: FAIL — hoje `Update` faz `t.ParentID = parentID` sem checar nada, então os 3 testes de rejeição passam batido (nenhum erro retornado).

- [ ] **Step 3: Implementar a validação no service**

Em `internal/services/theme_service.go`, adicionar o helper e chamá-lo nos dois pontos:

```go
func (s *ThemeService) validateParent(ctx context.Context, id string, parentID *string) error {
	if parentID == nil {
		return nil
	}
	if *parentID == id {
		return &ValidationError{"theme cannot be its own parent"}
	}
	parent, err := s.repo.GetByID(ctx, *parentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &ValidationError{"parent theme not found"}
		}
		return err
	}
	if parent.ParentID != nil {
		return &ValidationError{"parent theme cannot be a subcategory"}
	}
	if id != "" {
		children, err := s.repo.ChildIDs(ctx, id)
		if err != nil {
			return err
		}
		if len(children) > 0 {
			return &ValidationError{"theme with subcategories cannot become a subcategory"}
		}
	}
	return nil
}
```

Em `Create`, logo depois do check de `name`, com `id` vazio (o tema ainda não existe, então a regra de "tem filhos" não se aplica):

```go
	if err := s.validateParent(ctx, "", parentID); err != nil {
		return nil, err
	}
```

Em `Update`, depois do `GetByID` do tema:

```go
	if err := s.validateParent(ctx, id, parentID); err != nil {
		return nil, err
	}
```

Adicionar os imports `"errors"` e `"meeting-notes/internal/repository"` — este último já está importado (o campo `repo` é `*repository.ThemeRepository`), então só `"errors"` é novo.

- [ ] **Step 4: Rodar para ver passar**

Run: `go test ./internal/services/ -run TestThemeService -v`
Expected: PASS em todos, incluindo os pré-existentes.

- [ ] **Step 5: Verificação completa de backend**

Run: `go vet ./... && go test ./...`
Expected: PASS. Este é o último toque no Go — daqui para frente é só frontend.

- [ ] **Step 6: Commit**

```bash
git add internal/services/theme_service.go internal/services/theme_service_test.go
git commit -m "feat: validate theme hierarchy at two levels"
```

---

### Task 4: Frontend — tipos e modal unificado de tema

**Files:**
- Modify: `frontend/src/hooks/useThemes.ts`
- Modify: `frontend/src/hooks/useSettings.ts`
- Create: `frontend/src/components/sidebar/ThemeEditModal.tsx`
- Delete: `frontend/src/components/layout/ThemeEditModal.tsx`

**Interfaces:**
- Consumes: `custom_prompt` como único campo de prompt (Task 1); `sidebar_pinned` em settings (Task 2).
- Produces: `ThemeEditModal` com props `{ mode: "create" | "edit", theme: Theme | null, parentId?: string | null, onClose: () => void }`. `useCreateTheme` aceita `custom_prompt` e `useUpdateTheme` deixa de exigir os 3 campos por tipo.

- [ ] **Step 1: Enxugar os tipos e payloads**

Em `frontend/src/hooks/useThemes.ts`, remover as 3 linhas de prompt por tipo da interface `Theme` (linhas 11-13) e ajustar os dois payloads:

```ts
export interface Theme {
  id: string
  parent_id: string | null
  name: string
  description: string
  color: string
  custom_prompt: string
  auto_add_to_board: boolean
  created_at: string
}
```

```ts
export function useCreateTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description: string; color: string; parent_id?: string | null; custom_prompt?: string; auto_add_to_board?: boolean }) =>
      api<Theme>("/api/themes", { method: "POST", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["themes"] }),
  })
}

export function useUpdateTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { id: string; name: string; description: string; color: string; parent_id?: string | null; custom_prompt: string; auto_add_to_board?: boolean }) =>
      api<Theme>(`/api/themes/${data.id}`, { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["themes"] }),
  })
}
```

Em `frontend/src/hooks/useSettings.ts`, adicionar à interface `Settings`:

```ts
  sidebar_pinned: string
```

- [ ] **Step 2: Criar o modal unificado**

Criar `frontend/src/components/sidebar/ThemeEditModal.tsx` com o conteúdo completo abaixo. Ele cria **e** edita, tem um textarea de prompt só, e o `parentId` vem de quem abriu (a criação de subcategoria):

```tsx
import { useEffect, useState } from "react"
import { createPortal } from "react-dom"
import { X } from "lucide-react"
import { useCreateTheme, useUpdateTheme, type Theme } from "../../hooks/useThemes"
import { useAIConfigured } from "../../hooks/useAIConfigured"
import { Button } from "../ui/button"

interface Props {
  mode: "create" | "edit"
  theme: Theme | null
  parentId?: string | null
  onClose: () => void
}

const DEFAULT_COLOR = "#7c3aed"

export function ThemeEditModal({ mode, theme, parentId = null, onClose }: Props) {
  const createTheme = useCreateTheme()
  const updateTheme = useUpdateTheme()
  const { configured: aiConfigured } = useAIConfigured()
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [color, setColor] = useState(DEFAULT_COLOR)
  const [customPrompt, setCustomPrompt] = useState("")
  const [autoAddToBoard, setAutoAddToBoard] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (mode === "edit" && theme) {
      setName(theme.name)
      setDescription(theme.description)
      setColor(theme.color)
      setCustomPrompt(theme.custom_prompt)
      setAutoAddToBoard(theme.auto_add_to_board)
    } else {
      setName("")
      setDescription("")
      setColor(DEFAULT_COLOR)
      setCustomPrompt("")
      setAutoAddToBoard(false)
    }
    setError("")
  }, [mode, theme])

  const pending = createTheme.isPending || updateTheme.isPending

  async function handleSave() {
    if (!name.trim()) return
    setError("")
    try {
      if (mode === "edit" && theme) {
        await updateTheme.mutateAsync({
          id: theme.id,
          name: name.trim(),
          description,
          color,
          parent_id: theme.parent_id,
          custom_prompt: customPrompt,
          auto_add_to_board: autoAddToBoard,
        })
      } else {
        await createTheme.mutateAsync({
          name: name.trim(),
          description,
          color,
          parent_id: parentId,
          custom_prompt: customPrompt,
          auto_add_to_board: autoAddToBoard,
        })
      }
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : "Não foi possível salvar o tema.")
    }
  }

  const title = mode === "edit"
    ? "Editar tema"
    : parentId
      ? "Nova subcategoria"
      : "Novo tema"

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative z-10 w-full max-w-md mx-4 bg-[#1a1a1a] border border-border rounded-2xl p-6 shadow-xl">
        <div className="flex items-center justify-between mb-5">
          <h2 className="font-semibold text-sm text-foreground">{title}</h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X size={16} />
          </button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Nome</label>
            <input
              autoFocus
              value={name}
              onChange={e => setName(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleSave() }}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary"
              placeholder="Nome do tema"
            />
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Descrição</label>
            <input
              value={description}
              onChange={e => setDescription(e.target.value)}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary"
              placeholder="Descrição opcional"
            />
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Cor</label>
            <div className="flex items-center gap-2">
              <input
                type="color"
                value={color}
                onChange={e => setColor(e.target.value)}
                className="w-8 h-8 rounded cursor-pointer border-0 bg-transparent"
              />
              <span className="text-xs text-muted-foreground">{color}</span>
            </div>
          </div>

          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt personalizado</label>
            <textarea
              value={customPrompt}
              onChange={e => setCustomPrompt(e.target.value)}
              rows={4}
              disabled={!aiConfigured}
              title={!aiConfigured ? "Disponível quando a IA estiver configurada" : undefined}
              className="w-full text-sm rounded-lg px-3 py-2 bg-[#111] border border-border focus:outline-none focus:ring-1 focus:ring-primary resize-none disabled:opacity-50 disabled:cursor-not-allowed"
              placeholder="Ex: Foque em oportunidades comerciais, objeções e próximos passos."
            />
            <p className="text-[11px] text-muted-foreground mt-1">Vale para resumo, pontos-chave e tarefas. Vazio → usa o prompt padrão.</p>
          </div>

          {!aiConfigured && (
            <p className="text-[10px] text-amber-500">Prompt disponível quando a IA estiver configurada (Configurações → IA).</p>
          )}

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="auto_add_to_board"
              checked={autoAddToBoard}
              onChange={e => setAutoAddToBoard(e.target.checked)}
              className="w-4 h-4 rounded border-border accent-primary cursor-pointer"
            />
            <label htmlFor="auto_add_to_board" className="text-xs text-muted-foreground cursor-pointer select-none">
              Adicionar ao board automaticamente após processamento
            </label>
          </div>

          {error && <p className="text-[11px] text-destructive">{error}</p>}
        </div>

        <div className="flex justify-end gap-2 mt-6">
          <Button variant="ghost" size="sm" onClick={onClose}>Cancelar</Button>
          <Button size="sm" onClick={handleSave} disabled={!name.trim() || pending}>
            {pending ? "Salvando…" : "Salvar"}
          </Button>
        </div>
      </div>
    </div>,
    document.body
  )
}
```

- [ ] **Step 3: Remover o modal antigo**

```bash
git rm frontend/src/components/layout/ThemeEditModal.tsx
```

O `Sidebar.tsx` atual importa esse caminho e vai quebrar de propósito — a Task 5 reescreve a sidebar e conserta o import. Para manter o repo compilável **neste** commit, ajustar já o import em `frontend/src/components/layout/Sidebar.tsx` para `../sidebar/ThemeEditModal` e passar as props novas na linha 209:

```tsx
      <ThemeEditModal mode="edit" theme={editingTheme} onClose={() => setEditingTheme(null)} />
```

- [ ] **Step 4: Typecheck e build**

Run: `cd frontend && npx tsc --noEmit`
Expected: sem erros. Se acusar `custom_summary_prompt` em algum lugar, é resíduo do modal antigo — confirmar que ele foi removido.

Run: `cd frontend && npm run build`
Expected: build conclui.

- [ ] **Step 5: Commit**

```bash
git add -A frontend/src
git commit -m "feat: unified theme modal with a single custom prompt"
```

---

### Task 5: Frontend — pin/dock, sem auto-close e chip de filtro

**Files:**
- Create: `frontend/src/hooks/useSidebarPinned.ts`
- Create: `frontend/src/components/sidebar/Sidebar.tsx`
- Delete: `frontend/src/components/layout/Sidebar.tsx`
- Modify: `frontend/src/App.tsx:7,119-135,180-220`
- Modify: `frontend/src/components/layout/MeetingList.tsx:11-17,44,102-126`

**Interfaces:**
- Consumes: `sidebar_pinned` (Task 2), `ThemeEditModal` de `components/sidebar/` (Task 4).
- Produces: `useSidebarPinned(): { pinned: boolean; toggle: () => void }`. `Sidebar` com props `{ open, onClose, selectedThemeId, onSelectTheme }` — **`onSelectTheme` não fecha mais a sidebar**. `MeetingList` ganha a prop `onClearTheme: () => void`.

- [ ] **Step 1: Criar o hook de pin**

Criar `frontend/src/hooks/useSidebarPinned.ts`:

```ts
import { useSettings, useUpdateSettings } from "./useSettings"

export function useSidebarPinned() {
  const { data: settings } = useSettings()
  const updateSettings = useUpdateSettings()
  const pinned = settings?.sidebar_pinned === "true"

  return {
    pinned,
    toggle: () => updateSettings.mutate({ sidebar_pinned: pinned ? "false" : "true" }),
  }
}
```

- [ ] **Step 2: Mover a sidebar e aplicar pin + sem auto-close**

Criar `frontend/src/components/sidebar/Sidebar.tsx` a partir do arquivo atual (`components/layout/Sidebar.tsx`), com estas mudanças e **nada além delas** (a linha do tema é a Task 6; o drag é a Task 7):

1. Import do modal passa a ser `./ThemeEditModal`, usado com `mode`:

```tsx
  const [editingTheme, setEditingTheme] = useState<Theme | null>(null)
  const [creating, setCreating] = useState<{ parentId: string | null } | null>(null)
```

```tsx
      {editingTheme && (
        <ThemeEditModal mode="edit" theme={editingTheme} onClose={() => setEditingTheme(null)} />
      )}
      {creating && (
        <ThemeEditModal mode="create" theme={null} parentId={creating.parentId} onClose={() => setCreating(null)} />
      )}
```

Isso substitui o estado `creating: "root" | string | null`, o `newName`, o `handleCreate` e os dois blocos de input inline (linhas 129-144 e 190-206 do arquivo atual). O botão do rodapé passa a ser:

```tsx
        <div className="p-3 border-t border-border">
          <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => setCreating({ parentId: null })}>
            <Plus size={14} className="mr-1" /> Novo tema
          </Button>
        </div>
```

E o `+` da linha abre o modal de subcategoria: `onClick={e => { e.stopPropagation(); setCreating({ parentId: theme.id }) }}`.

2. `selectAndClose` sai. Todos os `onClick` de seleção passam a chamar `onSelectTheme(id)` direto — selecionar tema **não fecha mais** a sidebar.

3. A raiz do componente recebe o modo pinado:

```tsx
export function Sidebar({ open, onClose, selectedThemeId, onSelectTheme }: SidebarProps) {
  const { pinned, toggle: togglePinned } = useSidebarPinned()
```

```tsx
  if (!open) return <>{modals}</>

  return (
    <>
      {!pinned && (
        <div className="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm" onClick={onClose} />
      )}
      <div
        className={cn(
          "w-64 flex flex-col bg-[#161616] border-r border-border",
          pinned
            ? "h-full flex-shrink-0"
            : "fixed left-0 top-0 h-full z-40 rounded-r-2xl"
        )}
      >
        <div className="h-14 flex items-center justify-between px-4 border-b border-border flex-shrink-0">
          <span className="font-semibold text-sm text-foreground">Temas</span>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              onClick={togglePinned}
              title={pinned ? "Desafixar painel" : "Fixar painel"}
            >
              {pinned ? <PinOff size={15} /> : <Pin size={15} />}
            </Button>
            {!pinned && (
              <Button variant="ghost" size="icon" onClick={onClose} title="Fechar (Esc)">
                <X size={16} />
              </Button>
            )}
          </div>
        </div>
        {/* aqui entram, sem alteração de estilo, a lista e o rodapé:
            linhas 172-207 do arquivo original components/layout/Sidebar.tsx.
            As linhas 165-171 (header "Meeting Notes" + label "Temas") são
            substituídas pelo header acima, que já nomeia o painel. */}
      </div>
      {modals}
    </>
  )
}
```

Onde `modals` é o JSX dos dois `ThemeEditModal` do item 1, extraído para uma const — os modais precisam continuar montados mesmo com a sidebar fechada (o modal usa portal e a sidebar pode ser escondida enquanto ele está aberto).

Importar `Pin` e `PinOff` de `lucide-react`, e `useSidebarPinned` de `../../hooks/useSidebarPinned`. A animação `transform transition-transform` e o `translate-x-full` saem: a sidebar agora é montada/desmontada por `open`, sem gaveta animada em nenhum dos modos (evita o pinado nascer deslizando).

4. Fechar com Esc no modo gaveta:

```tsx
  useEffect(() => {
    if (pinned || !open) return
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose() }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [pinned, open, onClose])
```

Depois: `git rm frontend/src/components/layout/Sidebar.tsx`.

- [ ] **Step 3: Ligar no App — layout, Ctrl+B e Board**

Em `frontend/src/App.tsx`:

Trocar o import da linha 7 para `import { Sidebar } from "./components/sidebar/Sidebar"`.

Adicionar o atalho, junto do handler de Ctrl+K (linhas 126-135):

```tsx
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "k" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault()
        setSearchOpen(true)
      }
      if (e.key === "b" && (e.ctrlKey || e.metaKey)) {
        e.preventDefault()
        setSidebarOpen(o => !o)
      }
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [])
```

Mover o `<Sidebar>` de fora (linhas 194-199) para **dentro** do flex row, antes do conteúdo, e não renderizá-lo no Board:

```tsx
          <div className="flex flex-1 overflow-hidden">
            {activeView === "board" ? (
              <BoardView />
            ) : (
              <>
                <Sidebar
                  open={sidebarOpen}
                  onClose={() => setSidebarOpen(false)}
                  selectedThemeId={selectedThemeId}
                  onSelectTheme={setSelectedThemeId}
                />
                <MeetingList
                  themeId={selectedThemeId}
                  selectedMeetingId={selectedMeetingId}
                  onSelectMeeting={id => { setSelectedMeetingId(id); setHighlightQuery(undefined) }}
                  onMeetingDeleted={id => { if (selectedMeetingId === id) setSelectedMeetingId(null) }}
                  onOpenSearch={() => setSearchOpen(true)}
                  onClearTheme={() => setSelectedThemeId(null)}
                />
                <MeetingDetail
                  meetingId={selectedMeetingId}
                  onDeleted={() => setSelectedMeetingId(null)}
                  highlightQuery={highlightQuery}
                  onOpenSettings={() => setSettingsOpen(true)}
                />
              </>
            )}
          </div>
```

No modo gaveta a sidebar é `fixed`, então a posição no flex não a afeta; no modo pinado ela ocupa a coluna. Como o Board não renderiza a sidebar, `sidebarOpen` e `sidebar_pinned` sobrevivem à ida e volta — é o "recolhe e retoma" da spec.

- [ ] **Step 4: Chip de filtro ativo no MeetingList**

Em `frontend/src/components/layout/MeetingList.tsx`, adicionar `onClearTheme: () => void` à interface de props (linhas 11-17) e ao destructuring (linha 44). Depois do header (`</div>` da linha 126), inserir:

```tsx
      {activeTheme && (
        <div className="px-3 py-2 border-b border-border flex-shrink-0">
          <button
            onClick={onClearTheme}
            title="Limpar filtro de tema"
            className="inline-flex items-center gap-1.5 max-w-full rounded-full pl-2 pr-1.5 py-1 bg-accent border border-border text-xs text-foreground hover:bg-muted transition-colors"
          >
            <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: activeTheme.color }} />
            <span className="truncate">{activeTheme.name}</span>
            <X size={12} className="flex-shrink-0 text-muted-foreground" />
          </button>
        </div>
      )}
```

E resolver o tema junto dos filtros (o componente já chama `useThemes()`):

```tsx
  const activeTheme = themes.find(t => t.id === themeId) ?? null
```

Conferir o nome da variável que recebe `useThemes()` no arquivo e reusá-lo; `X` já está importado de `lucide-react` na linha 2.

- [ ] **Step 5: Typecheck e build**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros.

- [ ] **Step 6: Verificar ao vivo**

Com `wails dev` rodando: fixar o painel (a lista deve reflowar, sem backdrop), selecionar um tema (a sidebar **não** fecha, o chip aparece no header de Reuniões), clicar no `×` do chip (filtro limpa), `Ctrl+B` (esconde/mostra), ir ao Board (sidebar sai) e voltar (volta pinada), reiniciar o app (segue pinada).

- [ ] **Step 7: Commit**

```bash
git add -A frontend/src
git commit -m "feat: pinnable themes sidebar with visible active filter"
```

---

### Task 6: Frontend — linha do tema legível

**Files:**
- Create: `frontend/src/components/sidebar/ThemeRow.tsx`
- Create: `frontend/src/components/sidebar/ThemeRowMenu.tsx`
- Modify: `frontend/src/components/sidebar/Sidebar.tsx`

**Interfaces:**
- Consumes: `Theme` de `hooks/useThemes`.
- Produces: `ThemeRow` com props exatamente `{ theme: Theme; depth: number; count: number; selected: boolean; expanded: boolean; hasChildren: boolean; onSelect: () => void; onToggleExpand: () => void; onCreateChild: () => void; onEdit: () => void; onDelete: () => void }` — os indicadores de prompt e de board são derivados de `theme.custom_prompt` e `theme.auto_add_to_board` dentro do componente, não passados por prop. A Task 7 acrescenta a esta interface `draggable: boolean` e `droppable: boolean`. `ThemeRowMenu` com `{ anchor: HTMLElement | null; canAddChild: boolean; onAddChild: () => void; onEdit: () => void; onDelete: () => void; onClose: () => void }`.

- [ ] **Step 1: Criar o menu em portal**

Criar `frontend/src/components/sidebar/ThemeRowMenu.tsx`. Ele é ancorado por `getBoundingClientRect` porque a linha vive dentro de um `overflow-y-auto` — mesma convenção dos widgets flutuantes (DECISIONS 2026-05-07):

```tsx
import { useEffect, useRef } from "react"
import { createPortal } from "react-dom"
import { Plus, Pencil, Trash2 } from "lucide-react"

interface Props {
  anchor: HTMLElement | null
  canAddChild: boolean
  onAddChild: () => void
  onEdit: () => void
  onDelete: () => void
  onClose: () => void
}

export function ThemeRowMenu({ anchor, canAddChild, onAddChild, onEdit, onDelete, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("mousedown", onDown)
    document.addEventListener("keydown", onKey)
    return () => {
      document.removeEventListener("mousedown", onDown)
      document.removeEventListener("keydown", onKey)
    }
  }, [onClose])

  if (!anchor) return null
  const r = anchor.getBoundingClientRect()

  return createPortal(
    <div
      ref={ref}
      style={{ top: Math.min(r.bottom + 4, window.innerHeight - 130), left: r.left - 150 }}
      className="fixed z-50 w-44 py-1 bg-[#1a1a1a] border border-border rounded-xl shadow-xl"
    >
      {canAddChild && (
        <button onClick={onAddChild} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground">
          <Plus size={12} /> Nova subcategoria
        </button>
      )}
      <button onClick={onEdit} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-muted-foreground hover:bg-accent hover:text-foreground">
        <Pencil size={12} /> Editar tema
      </button>
      <button onClick={onDelete} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-destructive hover:bg-destructive/10">
        <Trash2 size={12} /> Excluir tema
      </button>
    </div>,
    document.body
  )
}
```

`canAddChild` é `false` para subcategorias — teto de 2 níveis, coerente com a validação da Task 3.

- [ ] **Step 2: Criar a linha**

Criar `frontend/src/components/sidebar/ThemeRow.tsx` — três botões **irmãos** (chevron, seleção, menu), barra de cor de 3px, badges:

```tsx
import { useRef, useState } from "react"
import { ChevronRight, MoreHorizontal, Sparkles, LayoutGrid } from "lucide-react"
import type { Theme } from "../../hooks/useThemes"
import { ThemeRowMenu } from "./ThemeRowMenu"
import { cn } from "../../lib/utils"

interface Props {
  theme: Theme
  depth: number
  count: number
  selected: boolean
  expanded: boolean
  hasChildren: boolean
  onSelect: () => void
  onToggleExpand: () => void
  onCreateChild: () => void
  onEdit: () => void
  onDelete: () => void
}

export function ThemeRow({
  theme, depth, count, selected, expanded, hasChildren,
  onSelect, onToggleExpand, onCreateChild, onEdit, onDelete,
}: Props) {
  const [menuOpen, setMenuOpen] = useState(false)
  const menuBtn = useRef<HTMLButtonElement>(null)
  const hasPrompt = theme.custom_prompt.trim() !== ""

  return (
    <div
      className={cn(
        "group relative flex items-center gap-1 rounded-xl pr-1 mt-0.5 hover:bg-accent transition-colors",
        selected && "bg-accent",
        depth > 0 && "ml-4"
      )}
    >
      <span
        aria-hidden
        className="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-full"
        style={{ backgroundColor: theme.color }}
      />

      <button
        type="button"
        onClick={onToggleExpand}
        aria-label={expanded ? "Recolher subcategorias" : "Expandir subcategorias"}
        className={cn(
          "ml-1.5 w-4 h-4 flex items-center justify-center flex-shrink-0 text-muted-foreground",
          !hasChildren && "invisible"
        )}
      >
        <ChevronRight size={12} className={cn("transition-transform", expanded && "rotate-90")} />
      </button>

      <button
        type="button"
        onClick={onSelect}
        aria-current={selected ? "true" : undefined}
        title={theme.description || theme.name}
        className="flex-1 min-w-0 flex items-center gap-1.5 py-2 text-left text-sm"
      >
        <span className={cn("truncate", selected ? "text-foreground font-medium" : "text-muted-foreground")}>
          {theme.name}
        </span>
        {hasPrompt && <Sparkles size={11} className="flex-shrink-0 text-muted-foreground" title="Prompt personalizado" />}
        {theme.auto_add_to_board && <LayoutGrid size={11} className="flex-shrink-0 text-muted-foreground" title="Adiciona ao board automaticamente" />}
      </button>

      <span className="text-[11px] tabular-nums text-muted-foreground flex-shrink-0">{count}</span>

      <button
        type="button"
        ref={menuBtn}
        onClick={() => setMenuOpen(v => !v)}
        aria-label={`Ações de ${theme.name}`}
        className="p-1 rounded-md flex-shrink-0 text-muted-foreground hover:bg-primary/20 hover:text-primary"
      >
        <MoreHorizontal size={14} />
      </button>

      {menuOpen && (
        <ThemeRowMenu
          anchor={menuBtn.current}
          canAddChild={depth === 0}
          onAddChild={() => { setMenuOpen(false); onCreateChild() }}
          onEdit={() => { setMenuOpen(false); onEdit() }}
          onDelete={() => { setMenuOpen(false); onDelete() }}
          onClose={() => setMenuOpen(false)}
        />
      )}
    </div>
  )
}
```

- [ ] **Step 3: Usar a linha nova e a confirmação de exclusão**

Em `Sidebar.tsx`, apagar o `ThemeRow` interno (função aninhada) e renderizar o componente novo, com o estado de confirmação de exclusão substituindo o duplo clique. O `confirmDelete` deixa de ser "primeiro clique arma" e passa a abrir uma confirmação escrita:

```tsx
  const [confirmDelete, setConfirmDelete] = useState<Theme | null>(null)
```

```tsx
  function renderRow(theme: Theme, depth = 0) {
    const children = childrenOf(theme.id)
    return (
      <div key={theme.id}>
        <ThemeRow
          theme={theme}
          depth={depth}
          count={countForTheme(theme.id)}
          selected={selectedThemeId === theme.id}
          expanded={!!expanded[theme.id]}
          hasChildren={children.length > 0}
          onSelect={() => onSelectTheme(theme.id)}
          onToggleExpand={() => toggleExpand(theme.id)}
          onCreateChild={() => setCreating({ parentId: theme.id })}
          onEdit={() => setEditingTheme(theme)}
          onDelete={() => setConfirmDelete(theme)}
        />
        {expanded[theme.id] && children.map(c => renderRow(c, depth + 1))}
      </div>
    )
  }
```

`toggleExpand` perde o parâmetro de evento (`stopPropagation` não é mais necessário: os botões são irmãos, não aninhados).

A confirmação, renderizada logo abaixo da lista, dizendo o efeito real (as FKs são `ON DELETE SET NULL`):

```tsx
      {confirmDelete && (
        <div className="mx-2 mb-2 p-3 rounded-xl bg-destructive/10 border border-destructive/30">
          <p className="text-xs text-foreground">
            Excluir <span className="font-medium">{confirmDelete.name}</span>?
          </p>
          <p className="text-[11px] text-muted-foreground mt-1">
            As {countForTheme(confirmDelete.id)} reuniões continuam, sem tema.
            {childrenOf(confirmDelete.id).length > 0 && " As subcategorias sobem para a raiz."}
          </p>
          <div className="flex justify-end gap-2 mt-2">
            <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(null)}>Cancelar</Button>
            <Button
              size="sm"
              onClick={async () => {
                const id = confirmDelete.id
                await deleteTheme.mutateAsync(id)
                if (selectedThemeId === id) onSelectTheme(null)
                setConfirmDelete(null)
              }}
              disabled={deleteTheme.isPending}
            >
              Excluir
            </Button>
          </div>
        </div>
      )}
```

O `handleDelete` antigo (linhas 51-60 do arquivo original) sai por completo.

- [ ] **Step 4: Typecheck e build**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros.

- [ ] **Step 5: Verificar ao vivo**

Barra de cor visível em cada linha; `Daily` mostra o badge de prompt e o de board; menu `⋯` visível sem hover e fechando com Esc/clique fora; "Nova subcategoria" ausente nas subcategorias; excluir mostra a frase com a contagem certa e só apaga no segundo botão; navegação por Tab alcança chevron, nome e menu.

- [ ] **Step 6: Commit**

```bash
git add -A frontend/src
git commit -m "feat: readable theme row with color bar, badges and actions menu"
```

---

### Task 7: Frontend — reparent por drag-and-drop e expansão persistida

**Files:**
- Create: `frontend/src/hooks/useThemeExpanded.ts`
- Modify: `frontend/src/components/sidebar/Sidebar.tsx`
- Modify: `frontend/src/components/sidebar/ThemeRow.tsx`

**Interfaces:**
- Consumes: validação de 2 níveis da Task 3 (422 com mensagem); `ThemeRow` da Task 6.
- Produces: `useThemeExpanded(themeIds: string[]): { expanded: Record<string, boolean>; toggle: (id: string) => void }`, persistido em `localStorage["theme_expanded"]`. `ThemeRow` ganha as props `{ draggable: boolean; droppable: boolean }`.

- [ ] **Step 1: Criar o hook de expansão persistida**

Criar `frontend/src/hooks/useThemeExpanded.ts`. A leitura filtra IDs que não existem mais, para não acumular lixo de temas excluídos:

```ts
import { useCallback, useEffect, useState } from "react"

const KEY = "theme_expanded"

function read(): string[] {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === "string") : []
  } catch {
    return []
  }
}

export function useThemeExpanded(themeIds: string[]) {
  const [ids, setIds] = useState<string[]>(read)

  useEffect(() => {
    if (themeIds.length === 0) return
    setIds(prev => {
      const pruned = prev.filter(id => themeIds.includes(id))
      return pruned.length === prev.length ? prev : pruned
    })
  }, [themeIds])

  useEffect(() => {
    try {
      localStorage.setItem(KEY, JSON.stringify(ids))
    } catch { /* modo privado / cota — expansão volta a ser efêmera */ }
  }, [ids])

  const toggle = useCallback((id: string) => {
    setIds(prev => prev.includes(id) ? prev.filter(v => v !== id) : [...prev, id])
  }, [])

  const expanded: Record<string, boolean> = {}
  for (const id of ids) expanded[id] = true

  return { expanded, toggle }
}
```

Em `Sidebar.tsx`, substituir o `useState<Record<string, boolean>>` e o `toggleExpand` por:

```tsx
  const { expanded, toggle: toggleExpand } = useThemeExpanded(themes.map(t => t.id))
```

- [ ] **Step 2: Tornar a linha arrastável e alvo de drop**

Em `ThemeRow.tsx`, envolver a linha com os hooks do dnd-kit. Um tema **raiz sem filhos** é arrastável para dentro de outro raiz; um tema **raiz** é alvo válido; subcategorias não são alvo (teto de 2 níveis):

```tsx
import { useDraggable, useDroppable } from "@dnd-kit/core"
```

Acrescentar à interface `Props` da Task 6:

```tsx
  draggable: boolean
  droppable: boolean
```

E no destructuring do componente, incluir `draggable, droppable`. Dentro do componente:

```tsx
  const drag = useDraggable({ id: theme.id, disabled: !draggable })
  const drop = useDroppable({ id: `drop-${theme.id}`, disabled: !droppable })
```

Aplicar no container externo (mantendo as classes já existentes):

```tsx
    <div
      ref={node => { drag.setNodeRef(node); drop.setNodeRef(node) }}
      {...drag.attributes}
      {...drag.listeners}
      className={cn(
        "group relative flex items-center gap-1 rounded-xl pr-1 mt-0.5 hover:bg-accent transition-colors",
        selected && "bg-accent",
        depth > 0 && "ml-4",
        drag.isDragging && "opacity-40",
        drop.isOver && droppable && "ring-1 ring-primary"
      )}
    >
```

Os `listeners` do dnd-kit no container não roubam o clique dos botões filhos (o sensor exige movimento para iniciar o drag), mas para garantir que um clique acidental não vire drag, configurar o sensor com distância mínima no passo seguinte.

- [ ] **Step 3: Ligar o DndContext e a faixa "mover para raiz"**

Em `Sidebar.tsx`, envolver a lista e tratar o drop:

```tsx
import { DndContext, PointerSensor, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core"
```

```tsx
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 5 } }))
  const [dragError, setDragError] = useState("")
  const [activeId, setActiveId] = useState<string | null>(null)
  const updateTheme = useUpdateTheme()

  async function handleDragEnd(e: DragEndEvent) {
    const themeId = String(e.active.id)
    const overId = e.over ? String(e.over.id) : ""
    if (!overId) return

    const moved = themes.find(t => t.id === themeId)
    if (!moved) return

    const parentId = overId === "drop-root" ? null : overId.replace("drop-", "")
    if (parentId === themeId) return
    if ((moved.parent_id ?? null) === parentId) return

    setDragError("")
    try {
      await updateTheme.mutateAsync({
        id: moved.id,
        name: moved.name,
        description: moved.description,
        color: moved.color,
        parent_id: parentId,
        custom_prompt: moved.custom_prompt,
        auto_add_to_board: moved.auto_add_to_board,
      })
    } catch (err) {
      setDragError(err instanceof Error ? err.message : "Não foi possível mover o tema.")
    }
  }
```

`useUpdateTheme` precisa entrar no import de `../../hooks/useThemes`. O backend é a autoridade — se a regra de 2 níveis for violada, vem 422 e a mensagem aparece; o `droppable` do frontend já evita o caso comum.

A faixa de raiz, renderizada acima da lista e visível só durante o arraste de uma subcategoria:

```tsx
  const { setNodeRef: setRootRef, isOver: rootIsOver } = useDroppable({ id: "drop-root" })
```

```tsx
        <div
          ref={setRootRef}
          className={cn(
            "mx-2 mb-1 px-3 py-1.5 rounded-lg border border-dashed text-[11px] text-muted-foreground text-center transition-colors",
            rootIsOver ? "border-primary text-primary" : "border-border"
          )}
        >
          Solte aqui para mover para a raiz
        </div>
```

Renderizar essa faixa apenas quando houver arraste ativo — usar `onDragStart` para guardar `activeId` em estado e condicionar `{activeId && ...}`.

E o erro, abaixo da lista:

```tsx
      {dragError && <p className="mx-2 mb-2 text-[11px] text-destructive">{dragError}</p>}
```

Envolver a lista com `<DndContext sensors={sensors} onDragStart={e => setActiveId(String(e.active.id))} onDragEnd={e => { setActiveId(null); handleDragEnd(e) }}>`.

Passar as props novas em `renderRow`:

```tsx
          draggable={depth === 0 && children.length === 0}
          droppable={depth === 0 && theme.id !== activeId}
```

- [ ] **Step 4: Typecheck e build**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: sem erros.

- [ ] **Step 5: Verificar ao vivo**

Arrastar um tema raiz sem filhos para outro raiz → vira subcategoria e a lista atualiza. Arrastar de volta para a faixa de raiz → volta a raiz. Tentar arrastar um tema que tem filhos → não é aceito. Expandir uma subcategoria, fechar e reabrir o app → segue expandida. Clicar (sem arrastar) continua selecionando o tema.

- [ ] **Step 6: Commit**

```bash
git add -A frontend/src
git commit -m "feat: drag-and-drop theme reparenting with persisted expansion"
```

---

## Final verification

- [ ] `go vet ./...` limpo; `go test ./...` verde.
- [ ] `cd frontend && npx tsc --noEmit` limpo; `npm run build` conclui.
- [ ] `grep -rn "PromptFor\|ThemePrompts\|custom_summary_prompt\|custom_key_points_prompt\|custom_tasks_prompt" --include=*.go --include=*.ts --include=*.tsx .` → vazio.
- [ ] Migration 016 aplica num banco novo **e** num banco existente (copiar o `.db` real para um temp e abrir).
- [ ] Validação ao vivo com `wails dev`: prompt único salva e é usado na geração; pin persiste entre reinícios; chip limpa o filtro; hierarquia respeita 2 níveis; exclusão diz a verdade sobre reuniões e subcategorias.
- [ ] Registrar no `.claude/DECISIONS.md`: (a) revert para prompt único, revisitando a decisão de 2026-07-21; (b) `localStorage` como lugar de estado efêmero de UI, com settings no banco para preferências de verdade; (c) teto de 2 níveis na hierarquia de temas.
