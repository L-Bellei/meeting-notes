# Per-Type Theme Prompts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a Theme carry a general custom prompt plus per-type overrides (summary / key points / tasks), applied with precedence specific → general → built-in default.

**Architecture:** Add 3 nullable-defaulted columns to `themes`; resolve precedence in a `Theme.PromptFor(kind)` helper so AI-client and generation-service signatures stay unchanged (call sites just pass the resolved string). Theme CRUD threads the 4 prompt fields through a small `models.ThemePrompts` struct to avoid a 4-adjacent-string-arg signature.

**Tech Stack:** Go 1.22+ (modernc/sqlite), React 19 + TypeScript.

## Global Constraints

- Sem comentários no código, salvo quando o WHY é não-óbvio.
- Sem mocks em testes de repositório — SQLite via `t.TempDir()` (helper `openThemeTestDB`).
- Dois entry points (`cmd/api`, `cmd/desktop`) compartilham services/repository.
- Migrations embed, aplicadas automaticamente; numeração sequencial (próxima: `015`).
- Precedência ao gerar cada tipo: **específico → geral (`custom_prompt`) → default embutido** (o último degrau já é feito por `buildInstruction` nos AI clients).
- Novas colunas: `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt`, `TEXT NOT NULL DEFAULT ''`.
- `Theme` struct fica em `internal/models/models.go` (não `meeting.go`).

---

### Task 1: Model + migration + PromptFor + repository

**Files:**
- Create: `internal/database/migrations/015_theme_type_prompts.sql`
- Modify: `internal/models/models.go:5-14` (Theme struct) + add `PromptKind`/`PromptFor`
- Modify: `internal/repository/theme_repository.go` (SELECTs at :29 and :67, INSERT :50-55, UPDATE :97-100, `scanTheme` :136-152)
- Test: `internal/models/theme_prompt_test.go` (new), `internal/repository/theme_repository_test.go`

**Interfaces:**
- Produces: `models.Theme.CustomSummaryPrompt`, `CustomKeyPointsPrompt`, `CustomTasksPrompt` (string, json `custom_summary_prompt` etc.); `models.PromptKind` with `PromptSummary`/`PromptKeyPoints`/`PromptTasks`; `func (t *Theme) PromptFor(kind PromptKind) string`.

- [ ] **Step 1: Write the failing PromptFor test**

Create `internal/models/theme_prompt_test.go`:

```go
package models_test

import (
	"testing"

	"meeting-notes/internal/models"
)

func TestTheme_PromptFor(t *testing.T) {
	theme := &models.Theme{
		CustomPrompt:          "GERAL",
		CustomSummaryPrompt:   "RESUMO",
		CustomKeyPointsPrompt: "",
		CustomTasksPrompt:     "TAREFAS",
	}

	if got := theme.PromptFor(models.PromptSummary); got != "RESUMO" {
		t.Errorf("summary: got %q, want specific 'RESUMO'", got)
	}
	if got := theme.PromptFor(models.PromptKeyPoints); got != "GERAL" {
		t.Errorf("key points: got %q, want fallback to general 'GERAL'", got)
	}
	if got := theme.PromptFor(models.PromptTasks); got != "TAREFAS" {
		t.Errorf("tasks: got %q, want specific 'TAREFAS'", got)
	}

	empty := &models.Theme{}
	if got := empty.PromptFor(models.PromptSummary); got != "" {
		t.Errorf("all-empty: got %q, want \"\" (AI client falls back to default)", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/models/ -run TestTheme_PromptFor -v`
Expected: FAIL — compile error (`CustomSummaryPrompt`, `PromptFor`, `PromptSummary` undefined).

- [ ] **Step 3: Add fields + PromptFor to the model**

In `internal/models/models.go`, extend the `Theme` struct (add after `CustomPrompt`):

```go
type Theme struct {
	ID                    string    `json:"id"`
	ParentID              *string   `json:"parent_id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Color                 string    `json:"color"`
	CustomPrompt          string    `json:"custom_prompt"`
	CustomSummaryPrompt   string    `json:"custom_summary_prompt"`
	CustomKeyPointsPrompt string    `json:"custom_key_points_prompt"`
	CustomTasksPrompt     string    `json:"custom_tasks_prompt"`
	AutoAddToBoard        bool      `json:"auto_add_to_board"`
	CreatedAt             time.Time `json:"created_at"`
}

type PromptKind int

const (
	PromptSummary PromptKind = iota
	PromptKeyPoints
	PromptTasks
)

func (t *Theme) PromptFor(kind PromptKind) string {
	var specific string
	switch kind {
	case PromptSummary:
		specific = t.CustomSummaryPrompt
	case PromptKeyPoints:
		specific = t.CustomKeyPointsPrompt
	case PromptTasks:
		specific = t.CustomTasksPrompt
	}
	if specific != "" {
		return specific
	}
	return t.CustomPrompt
}
```

- [ ] **Step 4: Run PromptFor test to verify it passes**

Run: `go test ./internal/models/ -run TestTheme_PromptFor -v`
Expected: PASS.

- [ ] **Step 5: Write the failing repository round-trip test**

Add to `internal/repository/theme_repository_test.go`:

```go
func TestThemeRepository_TypePrompts_RoundTrip(t *testing.T) {
	repo := openThemeTestDB(t)
	ctx := context.Background()

	theme := &models.Theme{
		ID:                  "th-prompts",
		Name:                "Prompts",
		Color:               "#123456",
		CustomPrompt:        "geral",
		CustomSummaryPrompt: "resumo custom",
		CustomTasksPrompt:   "tarefas custom",
	}
	if err := repo.Create(ctx, theme); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByID(ctx, "th-prompts")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CustomSummaryPrompt != "resumo custom" {
		t.Errorf("summary prompt = %q", got.CustomSummaryPrompt)
	}
	if got.CustomKeyPointsPrompt != "" {
		t.Errorf("key points prompt = %q, want empty (default)", got.CustomKeyPointsPrompt)
	}
	if got.CustomTasksPrompt != "tarefas custom" {
		t.Errorf("tasks prompt = %q", got.CustomTasksPrompt)
	}

	got.CustomKeyPointsPrompt = "pontos custom"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reloaded, err := repo.GetByID(ctx, "th-prompts")
	if err != nil {
		t.Fatalf("get reloaded: %v", err)
	}
	if reloaded.CustomKeyPointsPrompt != "pontos custom" {
		t.Errorf("updated key points prompt = %q", reloaded.CustomKeyPointsPrompt)
	}
}
```

(`openThemeTestDB(t)` is the existing helper at the top of the file.)

- [ ] **Step 6: Run repo test to verify it fails**

Run: `go test ./internal/repository/ -run TestThemeRepository_TypePrompts_RoundTrip -v`
Expected: FAIL — compile error (new fields) and/or missing column.

- [ ] **Step 7: Create migration 015**

Create `internal/database/migrations/015_theme_type_prompts.sql`:

```sql
ALTER TABLE themes ADD COLUMN custom_summary_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN custom_key_points_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN custom_tasks_prompt TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 8: Thread the columns through the repository**

In `internal/repository/theme_repository.go`:

`List` (line 29) — change the SELECT column list:
```go
		`SELECT id, parent_id, name, description, color, custom_prompt, custom_summary_prompt, custom_key_points_prompt, custom_tasks_prompt, auto_add_to_board, created_at FROM themes ORDER BY name`)
```

`GetByID` (line 67) — same column list:
```go
		`SELECT id, parent_id, name, description, color, custom_prompt, custom_summary_prompt, custom_key_points_prompt, custom_tasks_prompt, auto_add_to_board, created_at FROM themes WHERE id = ?`, id)
```

`Create` (lines 50-55) — extend INSERT:
```go
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO themes (id, parent_id, name, description, color, custom_prompt, custom_summary_prompt, custom_key_points_prompt, custom_tasks_prompt, auto_add_to_board, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		theme.ID, theme.ParentID, theme.Name, theme.Description, theme.Color, theme.CustomPrompt,
		theme.CustomSummaryPrompt, theme.CustomKeyPointsPrompt, theme.CustomTasksPrompt,
		theme.AutoAddToBoard,
		theme.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
```

`Update` (lines 97-100) — extend UPDATE:
```go
	result, err := r.db.ExecContext(ctx,
		`UPDATE themes SET parent_id = ?, name = ?, description = ?, color = ?, custom_prompt = ?, custom_summary_prompt = ?, custom_key_points_prompt = ?, custom_tasks_prompt = ?, auto_add_to_board = ? WHERE id = ?`,
		theme.ParentID, theme.Name, theme.Description, theme.Color, theme.CustomPrompt,
		theme.CustomSummaryPrompt, theme.CustomKeyPointsPrompt, theme.CustomTasksPrompt,
		theme.AutoAddToBoard, theme.ID,
	)
```

`scanTheme` (line 140) — add the 3 fields to the Scan in the matching position (after `CustomPrompt`, before `AutoAddToBoard`):
```go
	if err := row.Scan(&t.ID, &parentID, &t.Name, &t.Description, &t.Color, &t.CustomPrompt,
		&t.CustomSummaryPrompt, &t.CustomKeyPointsPrompt, &t.CustomTasksPrompt,
		&t.AutoAddToBoard, &createdAt); err != nil {
		return nil, err
	}
```

- [ ] **Step 9: Run both tests + full packages**

Run: `go test ./internal/models/ ./internal/repository/`
Expected: PASS (round-trip + PromptFor + no regressions).

- [ ] **Step 10: Commit**

```bash
git add internal/database/migrations/015_theme_type_prompts.sql internal/models/models.go internal/models/theme_prompt_test.go internal/repository/theme_repository.go internal/repository/theme_repository_test.go
git commit -m "feat: theme per-type prompt fields + PromptFor resolver (migration 015)"
```

---

### Task 2: Persist per-type prompts through service + handler

**Files:**
- Modify: `internal/models/models.go` (add `ThemePrompts` struct)
- Modify: `internal/services/theme_service.go` (`Create` :31, `Update` :58)
- Modify: `internal/services/theme_service_test.go` (8 call sites)
- Modify: `internal/handlers/theme_handler.go` (request structs :23-39, `Create` :59, `Update` :97)

**Interfaces:**
- Consumes: `models.Theme` fields from Task 1.
- Produces: `models.ThemePrompts{General, Summary, KeyPoints, Tasks string}`; `ThemeService.Create(ctx, name, description, color, parentID, prompts models.ThemePrompts, autoAddToBoard)` and `Update(ctx, id, name, description, color, parentID, prompts models.ThemePrompts, autoAddToBoard)`.

- [ ] **Step 1: Write the failing service test**

Add to `internal/services/theme_service_test.go`:

```go
func TestThemeService_Create_PersistsTypePrompts(t *testing.T) {
	svc := newThemeTestService(t)
	ctx := context.Background()

	theme, err := svc.Create(ctx, "Prompts", "", "", nil,
		models.ThemePrompts{General: "g", Summary: "s", KeyPoints: "k", Tasks: "t"}, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if theme.CustomPrompt != "g" || theme.CustomSummaryPrompt != "s" ||
		theme.CustomKeyPointsPrompt != "k" || theme.CustomTasksPrompt != "t" {
		t.Errorf("prompts not mapped: %+v", theme)
	}

	updated, err := svc.Update(ctx, theme.ID, "Prompts", "", "", nil,
		models.ThemePrompts{General: "g2", Summary: "s2"}, false)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.CustomPrompt != "g2" || updated.CustomSummaryPrompt != "s2" ||
		updated.CustomKeyPointsPrompt != "" || updated.CustomTasksPrompt != "" {
		t.Errorf("update did not map prompts: %+v", updated)
	}
}
```

Note: the test helper for building a `ThemeService` — reuse whatever the neighbouring tests use to construct `svc` (inspect the top of `theme_service_test.go`; if there is no shared helper, build it inline the same way an existing test does: open a DB via `database.Open(t.TempDir()+"/test.db")`, `repository.NewThemeRepository(db)`, `services.NewThemeService(repo)`). Name it `newThemeTestService` if you add a helper.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/ -run TestThemeService_Create_PersistsTypePrompts -v`
Expected: FAIL — compile error (`models.ThemePrompts` undefined; `Create` signature mismatch).

- [ ] **Step 3: Add the ThemePrompts struct**

In `internal/models/models.go`, add near the `Theme` type:

```go
type ThemePrompts struct {
	General   string
	Summary   string
	KeyPoints string
	Tasks     string
}
```

- [ ] **Step 4: Change ThemeService.Create/Update to accept ThemePrompts**

In `internal/services/theme_service.go`, `Create` (line 31): replace the `customPrompt string` parameter with `prompts models.ThemePrompts`, and map all four fields onto the new `Theme`:

```go
func (s *ThemeService) Create(ctx context.Context, name, description, color string, parentID *string, prompts models.ThemePrompts, autoAddToBoard bool) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if color == "" {
		color = "#6366f1"
	}
	t := &models.Theme{
		ID:                    uuid.New().String(),
		ParentID:              parentID,
		Name:                  name,
		Description:           description,
		Color:                 color,
		CustomPrompt:          prompts.General,
		CustomSummaryPrompt:   prompts.Summary,
		CustomKeyPointsPrompt: prompts.KeyPoints,
		CustomTasksPrompt:     prompts.Tasks,
		AutoAddToBoard:        autoAddToBoard,
		CreatedAt:             time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
```

`Update` (line 58): same parameter swap, map all four onto the loaded theme:

```go
func (s *ThemeService) Update(ctx context.Context, id, name, description, color string, parentID *string, prompts models.ThemePrompts, autoAddToBoard bool) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Name = name
	t.Description = description
	if color != "" {
		t.Color = color
	}
	t.ParentID = parentID
	t.CustomPrompt = prompts.General
	t.CustomSummaryPrompt = prompts.Summary
	t.CustomKeyPointsPrompt = prompts.KeyPoints
	t.CustomTasksPrompt = prompts.Tasks
	t.AutoAddToBoard = autoAddToBoard
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}
```

- [ ] **Step 5: Update the 8 existing service test call sites**

In `internal/services/theme_service_test.go`, replace the `""` customPrompt argument with `models.ThemePrompts{}` in each existing `svc.Create(...)`/`svc.Update(...)` call (lines ~27, 48, 60, 71, 72, 82, 105, 106, 122, 123 — every call that currently passes `..., nil, "", false)` or `..., nil, "", true)`). Example — line 27:

```go
	theme, err := svc.Create(ctx, "Produto", "Reuniões de produto", "#8b5cf6", nil, models.ThemePrompts{}, false)
```

and line 106:

```go
	updated, err := svc.Update(ctx, created.ID, "Novo Nome", "nova desc", "#ff0000", nil, models.ThemePrompts{}, false)
```

Apply the same `"" → models.ThemePrompts{}` swap to all remaining call sites. Ensure `models` is imported in the test file (it is used by other tests; if not, add `"meeting-notes/internal/models"`).

- [ ] **Step 6: Thread the fields through the handler**

In `internal/handlers/theme_handler.go`, add the 3 fields to BOTH request structs (`createThemeRequest` :23-30 and `updateThemeRequest` :32-39):

```go
	CustomPrompt          string  `json:"custom_prompt"`
	CustomSummaryPrompt   string  `json:"custom_summary_prompt"`
	CustomKeyPointsPrompt string  `json:"custom_key_points_prompt"`
	CustomTasksPrompt     string  `json:"custom_tasks_prompt"`
	AutoAddToBoard        bool    `json:"auto_add_to_board"`
```

`Create` (line 59) — build `ThemePrompts` from the request:

```go
	prompts := models.ThemePrompts{
		General:   req.CustomPrompt,
		Summary:   req.CustomSummaryPrompt,
		KeyPoints: req.CustomKeyPointsPrompt,
		Tasks:     req.CustomTasksPrompt,
	}
	theme, err := h.svc.Create(r.Context(), req.Name, req.Description, req.Color, req.ParentID, prompts, req.AutoAddToBoard)
```

`Update` (line 97) — same construction, then:

```go
	prompts := models.ThemePrompts{
		General:   req.CustomPrompt,
		Summary:   req.CustomSummaryPrompt,
		KeyPoints: req.CustomKeyPointsPrompt,
		Tasks:     req.CustomTasksPrompt,
	}
	theme, err := h.svc.Update(r.Context(), id, req.Name, req.Description, req.Color, req.ParentID, prompts, req.AutoAddToBoard)
```

- [ ] **Step 7: Run tests + full packages**

Run: `go test ./internal/services/ ./internal/handlers/`
Expected: PASS (new test + all existing theme/handler tests compile and pass).

- [ ] **Step 8: Commit**

```bash
git add internal/models/models.go internal/services/theme_service.go internal/services/theme_service_test.go internal/handlers/theme_handler.go
git commit -m "feat: persist per-type theme prompts via ThemePrompts"
```

---

### Task 3: Generation call sites use PromptFor

**Files:**
- Modify: `internal/handlers/summary_handler.go:107-113`, `internal/handlers/key_point_handler.go` (analogous block ~107-113), `internal/handlers/task_handler.go` (analogous block ~128-134)
- Modify: `internal/services/orchestrator.go:301-318` (`runAIGeneration`)
- Test: `internal/services/orchestrator_test.go`

**Interfaces:**
- Consumes: `Theme.PromptFor` (Task 1).
- Produces: each generation call receives the type-resolved prompt.

- [ ] **Step 1a: Add per-method prompt capture to fakeAI**

In `internal/services/orchestrator_test.go`, add three fields to the `fakeAI` struct:

```go
	lastSummaryPrompt   string
	lastKeyPointsPrompt string
	lastTasksPrompt     string
```

Then, at the TOP of `fakeAI`'s existing `GenerateSummary`/`GenerateKeyPoints`/`GenerateTasks` methods, record the received prompt (the 4th parameter — named `customPrompt` or similar in the signature). For example, in `GenerateSummary(ctx, transcript, notes, customPrompt string) (...)` add as the first line:

```go
	f.lastSummaryPrompt = customPrompt
```

and analogously `f.lastKeyPointsPrompt = customPrompt` in `GenerateKeyPoints`, `f.lastTasksPrompt = customPrompt` in `GenerateTasks`. (Match the actual 4th parameter name in each method signature.)

- [ ] **Step 1b: Write the failing self-contained orchestrator test**

Add this self-contained test (it builds its own DB + repos + services so it can seed a theme on the same DB — it does NOT reuse `newOrchTest`). All constructor signatures below match existing usage in this file (`services.NewOrchestrator(mr, thr, summarySvc, keyPointSvc, taskSvc, audioClient, settings, nil)`):

```go
func TestOrchestrator_AIGeneration_PerTypePrompts(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mr := repository.NewMeetingRepository(db)
	thr := repository.NewThemeRepository(db)
	sr := repository.NewSummaryRepository(db)
	kpr := repository.NewKeyPointRepository(db)
	tr := repository.NewTaskRepository(db)

	fai := &fakeAI{summaryText: "s", keyPoints: []string{"k"}, tasks: []ai.TaskSuggestion{{Description: "d", Priority: "medium"}}}
	summarySvc := services.NewSummaryService(sr, fai)
	keyPointSvc := services.NewKeyPointService(kpr, fai)
	taskSvc := services.NewTaskService(tr, fai)

	wavPath := t.TempDir() + "/rec.wav"
	if err := os.WriteFile(wavPath, fakeWAVBytes(), 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{Path: wavPath, DurationSeconds: 5.0},
		transcribeResp: &audio.TranscribeResponse{Transcript: "x", Language: "pt", DurationSeconds: 5.0},
	}
	settings := map[string]string{"ai_provider": "anthropic", "anthropic_api_key": "sk-test"}
	orch := services.NewOrchestrator(mr, thr, summarySvc, keyPointSvc, taskSvc, fa, &fakeSettings{data: settings}, nil)

	theme := &models.Theme{ID: "th-1", Name: "T", Color: "#111111", CustomPrompt: "GERAL", CustomSummaryPrompt: "RESUMO"}
	if err := thr.Create(context.Background(), theme); err != nil {
		t.Fatalf("seed theme: %v", err)
	}
	tid := "th-1"
	now := time.Now().UTC()
	m := &models.Meeting{ID: "m-1", Title: "R", ThemeID: &tid, StartedAt: &now, Status: models.StatusRecording}
	if err := mr.Create(context.Background(), m); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}

	if err := orch.RunCapturePipeline(context.Background(), "m-1"); err != nil {
		t.Fatalf("RunCapturePipeline: %v", err)
	}

	if fai.lastSummaryPrompt != "RESUMO" {
		t.Errorf("summary prompt = %q, want specific 'RESUMO'", fai.lastSummaryPrompt)
	}
	if fai.lastKeyPointsPrompt != "GERAL" {
		t.Errorf("key points prompt = %q, want general fallback 'GERAL'", fai.lastKeyPointsPrompt)
	}
	if fai.lastTasksPrompt != "GERAL" {
		t.Errorf("tasks prompt = %q, want general fallback 'GERAL'", fai.lastTasksPrompt)
	}
}
```

If any constructor name/arg differs from the above when you read the file, adjust to the real signature — but do NOT invent helpers; the wiring mirrors `newOrchTestSettings` in the same file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/services/ -run TestOrchestrator_AIGeneration_PerTypePrompts -v`
Expected: FAIL — currently `runAIGeneration` passes the single general `customPrompt` to all three, so `lastSummaryPrompt` would be `"GERAL"`, not `"RESUMO"`.

- [ ] **Step 3: Resolve per-type prompts in the orchestrator**

In `internal/services/orchestrator.go`, `runAIGeneration` (lines 301-318): replace the single `customPrompt` with per-type resolution via the theme:

```go
func (o *Orchestrator) runAIGeneration(ctx context.Context, m *models.Meeting) error {
	var theme *models.Theme
	if m.ThemeID != nil {
		if t, err := o.themeRepo.GetByID(ctx, *m.ThemeID); err == nil {
			theme = t
		}
	}
	promptFor := func(kind models.PromptKind) string {
		if theme == nil {
			return ""
		}
		return theme.PromptFor(kind)
	}
	if _, err := o.summarySvc.Generate(ctx, m, promptFor(models.PromptSummary)); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	if _, err := o.keyPointSvc.Generate(ctx, m, promptFor(models.PromptKeyPoints)); err != nil {
		return fmt.Errorf("key_points: %w", err)
	}
	if _, err := o.taskSvc.Generate(ctx, m, promptFor(models.PromptTasks)); err != nil {
		return fmt.Errorf("tasks: %w", err)
	}
	if theme != nil && o.boardCardSvc != nil && theme.AutoAddToBoard {
```

(Leave the rest of the function — the `theme != nil && ... AutoAddToBoard` block onward — unchanged.)

- [ ] **Step 4: Resolve per-type prompts in the 3 handlers**

In `internal/handlers/summary_handler.go` (lines 107-113), replace:

```go
	customPrompt := ""
	if meeting.ThemeID != nil {
		if theme, err := h.themeRepo.GetByID(r.Context(), *meeting.ThemeID); err == nil {
			customPrompt = theme.PromptFor(models.PromptSummary)
		}
	}
	s, err := h.svc.Generate(r.Context(), meeting, customPrompt)
```

In `internal/handlers/key_point_handler.go` (analogous block), use `theme.PromptFor(models.PromptKeyPoints)`.

In `internal/handlers/task_handler.go` (analogous block), use `theme.PromptFor(models.PromptTasks)`.

Ensure `"meeting-notes/internal/models"` is imported in each handler (add it if the handler doesn't already import it).

- [ ] **Step 5: Run the failing test + full services/handlers packages**

Run: `go test ./internal/services/ ./internal/handlers/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services/orchestrator.go internal/services/orchestrator_test.go internal/handlers/summary_handler.go internal/handlers/key_point_handler.go internal/handlers/task_handler.go
git commit -m "feat: generation resolves per-type theme prompt via PromptFor"
```

---

### Task 4: Frontend — type + ThemeEditModal fields

**Files:**
- Modify: `frontend/src/hooks/useThemes.ts` (Theme interface :4-13, create payload :22, update payload :31)
- Modify: `frontend/src/components/layout/ThemeEditModal.tsx`

**Interfaces:**
- Consumes: backend JSON fields `custom_summary_prompt`, `custom_key_points_prompt`, `custom_tasks_prompt`.

> **Note:** the frontend has no test runner (no `test` script / vitest / `*.test.*`). Coverage is `tsc --noEmit` + `npm run build` (consistent with the project).

- [ ] **Step 1: Add the fields to the Theme type and payloads**

In `frontend/src/hooks/useThemes.ts`, extend the `Theme` interface (after `custom_prompt`):

```ts
  custom_prompt: string
  custom_summary_prompt: string
  custom_key_points_prompt: string
  custom_tasks_prompt: string
  auto_add_to_board: boolean
```

Extend the update mutation payload type (line 31) to include the 3 fields:

```ts
    mutationFn: (data: { id: string; name: string; description: string; color: string; parent_id?: string | null; custom_prompt: string; custom_summary_prompt: string; custom_key_points_prompt: string; custom_tasks_prompt: string; auto_add_to_board?: boolean }) =>
```

(The create payload at line 22 doesn't currently include prompts; leave it as-is unless theme creation also needs prompts — out of scope here, editing is done via update.)

- [ ] **Step 2: Add state + populate + save for the 3 fields**

In `frontend/src/components/layout/ThemeEditModal.tsx`:

Add state (after `customPrompt`):

```tsx
  const [customPrompt, setCustomPrompt] = useState("")
  const [summaryPrompt, setSummaryPrompt] = useState("")
  const [keyPointsPrompt, setKeyPointsPrompt] = useState("")
  const [tasksPrompt, setTasksPrompt] = useState("")
```

Populate in the `useEffect` (after `setCustomPrompt(theme.custom_prompt)`):

```tsx
      setCustomPrompt(theme.custom_prompt)
      setSummaryPrompt(theme.custom_summary_prompt)
      setKeyPointsPrompt(theme.custom_key_points_prompt)
      setTasksPrompt(theme.custom_tasks_prompt)
```

Include them in the `handleSave` payload (after `custom_prompt: customPrompt,`):

```tsx
      custom_prompt: customPrompt,
      custom_summary_prompt: summaryPrompt,
      custom_key_points_prompt: keyPointsPrompt,
      custom_tasks_prompt: tasksPrompt,
```

- [ ] **Step 3: Add the 3 textareas + hint**

In `ThemeEditModal.tsx`, find the existing "Prompt personalizado" block (around line 94). Rename its label to "Prompt geral" and add three textareas after it. Match the existing textarea's classes (copy them from the existing `custom prompt` textarea in this file — do not invent new styling). Structure:

```tsx
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt geral</label>
            <textarea
              value={customPrompt}
              onChange={e => setCustomPrompt(e.target.value)}
              rows={3}
              className={/* same className as the current custom-prompt textarea */ ""}
            />
            <p className="text-[11px] text-muted-foreground mt-1">Vazio nos campos abaixo → usa o prompt geral; geral vazio → usa o padrão.</p>
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt do resumo</label>
            <textarea value={summaryPrompt} onChange={e => setSummaryPrompt(e.target.value)} rows={2} className={/* same className */ ""} />
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt dos pontos-chave</label>
            <textarea value={keyPointsPrompt} onChange={e => setKeyPointsPrompt(e.target.value)} rows={2} className={/* same className */ ""} />
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">Prompt das tarefas</label>
            <textarea value={tasksPrompt} onChange={e => setTasksPrompt(e.target.value)} rows={2} className={/* same className */ ""} />
          </div>
```

Read the existing textarea in this file first and reuse its exact `className` string for all four (the current custom-prompt textarea is at ~line 95-98). Keep the four textareas inside the same container the current one lives in.

- [ ] **Step 4: Typecheck**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 5: Build (sanity)**

Run: `cd frontend && npm run build`
Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/hooks/useThemes.ts frontend/src/components/layout/ThemeEditModal.tsx
git commit -m "feat: per-type prompt fields in ThemeEditModal"
```

---

## Final verification

- [ ] `go vet ./...` clean; `go test ./...` green.
- [ ] `cd frontend && npx tsc --noEmit` clean; `npm run build` succeeds.
- [ ] Manual (via `wails dev`): edit a theme, set only the "Prompt do resumo", attach the theme to a meeting, reprocess → summary uses the specific prompt; key points and tasks use the general (or built-in default when general is empty).
