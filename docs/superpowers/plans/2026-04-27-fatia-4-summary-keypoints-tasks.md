# Fatia 4 — Summary, Key Points & Tasks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement CRUD for `summary`, `key_points` and `tasks` as sub-resources of meetings, with AI generation via Anthropic API. Update `GET /api/meetings/{id}` to return real nested data.

**Architecture:** Same 3-layer pattern as Fatia 2/3 (repository → service → handler). New `internal/ai/` package with `AIClient` interface and `AnthropicClient` implementation. Services that generate via AI receive an `AIClient` via dependency injection — tests use a `fakeAIClient`. `MeetingHandler` receives the three new repositories to populate `MeetingDetailResponse`.

**Tech Stack:** Go 1.26, chi v5, modernc.org/sqlite (pure Go), `github.com/anthropics/anthropic-sdk-go`, `github.com/google/uuid`.

---

## Project conventions reminder

- Tests use real SQLite via `database.Open(t.TempDir() + "/test.db")`. No mocks for DB.
- FK enforcement is active in SQLite — pre-seed parent rows (themes, meetings) in test helpers.
- Sentinel errors: `repository.ErrNotFound`, `services.ValidationError` (pointer receiver, defined in `theme_service.go`).
- `parseTime` helper lives in `theme_repository.go` (package `repository`) — reuse it in new repositories, do NOT duplicate.
- Test helpers `withChiID` and `newTestMeetingAndThemeHandlers` are in `theme_handler_test.go` / `meeting_handler_test.go` (package `handlers_test`) — reuse, do NOT redefine.
- Config (`internal/config/config.go`) already has `AnthropicAPIKey` and `AnthropicModel` fields. No config changes required.
- Handler helpers `writeJSON` / `writeError` in `internal/handlers/respond.go`.

---

## Task 1: AI Client (Anthropic)

**Files:**
- Create: `internal/ai/anthropic_client.go`
- Modify: `go.mod` (add `github.com/anthropics/anthropic-sdk-go`)

- [ ] **Step 1: Add Anthropic SDK dependency**

Run from `F:\dev\meeting-notes`:
```bash
go get github.com/anthropics/anthropic-sdk-go@latest
```

Expected: `go.mod` and `go.sum` updated with the SDK and its transitive deps.

- [ ] **Step 2: Create AI client file**

Write `internal/ai/anthropic_client.go`:

```go
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type TaskSuggestion struct {
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	Priority    string `json:"priority"`
}

type AIClient interface {
	GenerateSummary(ctx context.Context, transcript string) (content string, inputTokens, outputTokens int, err error)
	GenerateKeyPoints(ctx context.Context, transcript string) (points []string, inputTokens, outputTokens int, err error)
	GenerateTasks(ctx context.Context, transcript string) (tasks []TaskSuggestion, inputTokens, outputTokens int, err error)
}

type AnthropicClient struct {
	client anthropic.Client
	model  string
}

func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	c := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicClient{client: c, model: model}
}

func (c *AnthropicClient) Model() string { return c.model }

func (c *AnthropicClient) GenerateSummary(ctx context.Context, transcript string) (string, int, int, error) {
	prompt := fmt.Sprintf(`Summarize the following meeting transcript in 2-3 paragraphs, in the same language as the transcript. Return ONLY a JSON object with the shape {"summary":"..."} and no extra text.

Transcript:
%s`, transcript)

	text, in, out, err := c.callJSON(ctx, prompt, 1024)
	if err != nil {
		return "", 0, 0, err
	}
	var result struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return "", 0, 0, fmt.Errorf("parse summary response: %w (raw: %s)", err, text)
	}
	return result.Summary, in, out, nil
}

func (c *AnthropicClient) GenerateKeyPoints(ctx context.Context, transcript string) ([]string, int, int, error) {
	prompt := fmt.Sprintf(`Extract the key points discussed in the following meeting transcript, in the same language as the transcript. Return ONLY a JSON array of strings: ["point 1","point 2",...] and no extra text.

Transcript:
%s`, transcript)

	text, in, out, err := c.callJSON(ctx, prompt, 1024)
	if err != nil {
		return nil, 0, 0, err
	}
	var points []string
	if err := json.Unmarshal([]byte(text), &points); err != nil {
		return nil, 0, 0, fmt.Errorf("parse key points response: %w (raw: %s)", err, text)
	}
	return points, in, out, nil
}

func (c *AnthropicClient) GenerateTasks(ctx context.Context, transcript string) ([]TaskSuggestion, int, int, error) {
	prompt := fmt.Sprintf(`Extract action items from the following meeting transcript, in the same language as the transcript. Return ONLY a JSON array with the shape [{"description":"...","assignee":"name or empty string","priority":"low|medium|high"},...] and no extra text.

Transcript:
%s`, transcript)

	text, in, out, err := c.callJSON(ctx, prompt, 2048)
	if err != nil {
		return nil, 0, 0, err
	}
	var tasks []TaskSuggestion
	if err := json.Unmarshal([]byte(text), &tasks); err != nil {
		return nil, 0, 0, fmt.Errorf("parse tasks response: %w (raw: %s)", err, text)
	}
	return tasks, in, out, nil
}

func (c *AnthropicClient) callJSON(ctx context.Context, prompt string, maxTokens int64) (string, int, int, error) {
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: maxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("anthropic call: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", 0, 0, fmt.Errorf("anthropic returned no content")
	}
	return msg.Content[0].Text, int(msg.Usage.InputTokens), int(msg.Usage.OutputTokens), nil
}
```

**Note for implementer:** the exact API shape of `github.com/anthropics/anthropic-sdk-go` may differ slightly from the code above (e.g., field-wrapping helper `anthropic.F()` in older SDK versions). After running `go get`, verify the actual SDK API by reading the README in `~/go/pkg/mod/github.com/anthropics/anthropic-sdk-go@*` or by running `go doc github.com/anthropics/anthropic-sdk-go MessageNewParams`. Adjust the `callJSON` body if needed; keep the public interface (`AIClient`) and the JSON parsing logic identical.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/ai/...`
Expected: success, no errors. If the SDK API differs, fix the `callJSON` body and try again.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/ai/anthropic_client.go
git commit -m "feat(ai): add Anthropic AIClient interface and implementation"
```

---

## Task 2: Summary Repository

**Files:**
- Create: `internal/repository/summary_repository.go`
- Create: `internal/repository/summary_repository_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/repository/summary_repository_test.go`:

```go
package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func openSummaryTestDB(t *testing.T) (*repository.SummaryRepository, *repository.MeetingRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	now := time.Now().UTC()
	if err := mr.Create(context.Background(), &models.Meeting{
		ID: "m-1", Title: "Reunião", StartedAt: &now, Status: models.StatusPending,
	}); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return repository.NewSummaryRepository(db), mr
}

func TestSummaryRepository_GetByMeetingID_NotFound(t *testing.T) {
	repo, _ := openSummaryTestDB(t)
	_, err := repo.GetByMeetingID(context.Background(), "m-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSummaryRepository_Upsert_Insert(t *testing.T) {
	repo, _ := openSummaryTestDB(t)
	ctx := context.Background()
	s := &models.Summary{
		ID: "s-1", MeetingID: "m-1", Content: "Resumo", ModelUsed: "claude-haiku-4-5",
		InputTokens: 100, OutputTokens: 50,
	}
	if err := repo.Upsert(ctx, s); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := repo.GetByMeetingID(ctx, "m-1")
	if err != nil {
		t.Fatalf("GetByMeetingID: %v", err)
	}
	if got.Content != "Resumo" {
		t.Errorf("Content = %q", got.Content)
	}
	if got.InputTokens != 100 || got.OutputTokens != 50 {
		t.Errorf("Tokens = %d/%d", got.InputTokens, got.OutputTokens)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestSummaryRepository_Upsert_Replaces(t *testing.T) {
	repo, _ := openSummaryTestDB(t)
	ctx := context.Background()
	repo.Upsert(ctx, &models.Summary{ID: "s-1", MeetingID: "m-1", Content: "Original", ModelUsed: "x"})
	repo.Upsert(ctx, &models.Summary{ID: "s-2", MeetingID: "m-1", Content: "Substituído", ModelUsed: "y"})
	got, err := repo.GetByMeetingID(ctx, "m-1")
	if err != nil {
		t.Fatalf("GetByMeetingID: %v", err)
	}
	if got.Content != "Substituído" {
		t.Errorf("Content = %q, want Substituído", got.Content)
	}
}

func TestSummaryRepository_Delete(t *testing.T) {
	repo, _ := openSummaryTestDB(t)
	ctx := context.Background()
	repo.Upsert(ctx, &models.Summary{ID: "s-1", MeetingID: "m-1", Content: "Resumo", ModelUsed: "x"})
	if err := repo.Delete(ctx, "m-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByMeetingID(ctx, "m-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSummaryRepository_Delete_NotFound(t *testing.T) {
	repo, _ := openSummaryTestDB(t)
	err := repo.Delete(context.Background(), "m-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repository/ -run TestSummaryRepository -v`
Expected: FAIL with "undefined: repository.SummaryRepository" or similar.

- [ ] **Step 3: Implement repository**

Create `internal/repository/summary_repository.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"meeting-notes/internal/models"
)

type SummaryRepository struct {
	db *sql.DB
}

func NewSummaryRepository(db *sql.DB) *SummaryRepository {
	return &SummaryRepository{db: db}
}

func (r *SummaryRepository) GetByMeetingID(ctx context.Context, meetingID string) (*models.Summary, error) {
	var s models.Summary
	var createdAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, content, model_used, input_tokens, output_tokens, created_at FROM summaries WHERE meeting_id = ?`,
		meetingID,
	).Scan(&s.ID, &s.MeetingID, &s.Content, &s.ModelUsed, &s.InputTokens, &s.OutputTokens, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}
	if s.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SummaryRepository) Upsert(ctx context.Context, s *models.Summary) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO summaries (id, meeting_id, content, model_used, input_tokens, output_tokens, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(meeting_id) DO UPDATE SET
		   id = excluded.id,
		   content = excluded.content,
		   model_used = excluded.model_used,
		   input_tokens = excluded.input_tokens,
		   output_tokens = excluded.output_tokens,
		   created_at = excluded.created_at`,
		s.ID, s.MeetingID, s.Content, s.ModelUsed, s.InputTokens, s.OutputTokens,
		s.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert summary: %w", err)
	}
	return nil
}

func (r *SummaryRepository) Delete(ctx context.Context, meetingID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM summaries WHERE meeting_id = ?`, meetingID)
	if err != nil {
		return fmt.Errorf("delete summary: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete summary rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

**Note:** the `ON CONFLICT(meeting_id)` clause requires a unique index on `summaries.meeting_id`. The current schema does NOT have this index (only `idx_summaries_meeting` which is non-unique). Add a migration in the next step.

- [ ] **Step 4: Add unique index migration**

Append to `internal/database/migrations/001_initial.sql` — change the `idx_summaries_meeting` line to a UNIQUE index. Replace:

```sql
CREATE INDEX IF NOT EXISTS idx_summaries_meeting  ON summaries(meeting_id);
```

with:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_summaries_meeting  ON summaries(meeting_id);
```

**Note:** since we use `IF NOT EXISTS`, existing dev databases won't have this UNIQUE constraint applied — manual recreation of the dev DB may be needed (delete `meeting-notes.db` and re-run). For tests, each `t.TempDir()` creates a fresh DB so the new index is applied.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/repository/ -run TestSummaryRepository -v`
Expected: PASS for all 5 tests.

- [ ] **Step 6: Verify no regressions**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/repository/summary_repository.go internal/repository/summary_repository_test.go internal/database/migrations/001_initial.sql
git commit -m "feat(repository): add SummaryRepository with upsert and unique index"
```

---

## Task 3: Key Point Repository

**Files:**
- Create: `internal/repository/key_point_repository.go`
- Create: `internal/repository/key_point_repository_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/repository/key_point_repository_test.go`:

```go
package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func openKeyPointTestDB(t *testing.T) *repository.KeyPointRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	now := time.Now().UTC()
	if err := mr.Create(context.Background(), &models.Meeting{
		ID: "m-1", Title: "Reunião", StartedAt: &now, Status: models.StatusPending,
	}); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return repository.NewKeyPointRepository(db)
}

func TestKeyPointRepository_ListByMeetingID_Empty(t *testing.T) {
	repo := openKeyPointTestDB(t)
	kps, err := repo.ListByMeetingID(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(kps) != 0 {
		t.Errorf("expected 0, got %d", len(kps))
	}
	if kps == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestKeyPointRepository_CreateAndList(t *testing.T) {
	repo := openKeyPointTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.KeyPoint{ID: "kp-2", MeetingID: "m-1", Position: 1, Content: "Segundo"})
	repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "Primeiro"})

	kps, err := repo.ListByMeetingID(ctx, "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(kps) != 2 {
		t.Fatalf("expected 2, got %d", len(kps))
	}
	if kps[0].Content != "Primeiro" {
		t.Errorf("expected position 0 first, got %q", kps[0].Content)
	}
	if kps[1].Content != "Segundo" {
		t.Errorf("expected position 1 second, got %q", kps[1].Content)
	}
}

func TestKeyPointRepository_GetByID(t *testing.T) {
	repo := openKeyPointTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "X"})

	got, err := repo.GetByID(ctx, "kp-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Content != "X" {
		t.Errorf("Content = %q", got.Content)
	}
}

func TestKeyPointRepository_GetByID_NotFound(t *testing.T) {
	repo := openKeyPointTestDB(t)
	_, err := repo.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestKeyPointRepository_Update(t *testing.T) {
	repo := openKeyPointTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "Antigo"})
	if err := repo.Update(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 5, Content: "Novo"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := repo.GetByID(ctx, "kp-1")
	if got.Content != "Novo" {
		t.Errorf("Content = %q", got.Content)
	}
	if got.Position != 5 {
		t.Errorf("Position = %d", got.Position)
	}
}

func TestKeyPointRepository_Update_NotFound(t *testing.T) {
	repo := openKeyPointTestDB(t)
	err := repo.Update(context.Background(), &models.KeyPoint{ID: "nope", MeetingID: "m-1"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestKeyPointRepository_Delete(t *testing.T) {
	repo := openKeyPointTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Content: "X"})
	if err := repo.Delete(ctx, "kp-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, "kp-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestKeyPointRepository_Delete_NotFound(t *testing.T) {
	repo := openKeyPointTestDB(t)
	err := repo.Delete(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestKeyPointRepository_DeleteByMeetingID(t *testing.T) {
	repo := openKeyPointTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Content: "A"})
	repo.Create(ctx, &models.KeyPoint{ID: "kp-2", MeetingID: "m-1", Content: "B"})

	if err := repo.DeleteByMeetingID(ctx, "m-1"); err != nil {
		t.Fatalf("DeleteByMeetingID: %v", err)
	}
	kps, _ := repo.ListByMeetingID(ctx, "m-1")
	if len(kps) != 0 {
		t.Errorf("expected 0 after DeleteByMeetingID, got %d", len(kps))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repository/ -run TestKeyPointRepository -v`
Expected: FAIL with "undefined: repository.KeyPointRepository".

- [ ] **Step 3: Implement repository**

Create `internal/repository/key_point_repository.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"meeting-notes/internal/models"
)

type KeyPointRepository struct {
	db *sql.DB
}

func NewKeyPointRepository(db *sql.DB) *KeyPointRepository {
	return &KeyPointRepository{db: db}
}

func (r *KeyPointRepository) ListByMeetingID(ctx context.Context, meetingID string) ([]models.KeyPoint, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, meeting_id, position, content FROM key_points WHERE meeting_id = ? ORDER BY position ASC`,
		meetingID,
	)
	if err != nil {
		return nil, fmt.Errorf("list key points: %w", err)
	}
	defer rows.Close()

	kps := []models.KeyPoint{}
	for rows.Next() {
		var kp models.KeyPoint
		if err := rows.Scan(&kp.ID, &kp.MeetingID, &kp.Position, &kp.Content); err != nil {
			return nil, fmt.Errorf("scan key point: %w", err)
		}
		kps = append(kps, kp)
	}
	return kps, rows.Err()
}

func (r *KeyPointRepository) Create(ctx context.Context, kp *models.KeyPoint) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO key_points (id, meeting_id, position, content) VALUES (?, ?, ?, ?)`,
		kp.ID, kp.MeetingID, kp.Position, kp.Content,
	)
	if err != nil {
		return fmt.Errorf("create key point: %w", err)
	}
	return nil
}

func (r *KeyPointRepository) GetByID(ctx context.Context, id string) (*models.KeyPoint, error) {
	var kp models.KeyPoint
	err := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, position, content FROM key_points WHERE id = ?`, id,
	).Scan(&kp.ID, &kp.MeetingID, &kp.Position, &kp.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get key point: %w", err)
	}
	return &kp, nil
}

func (r *KeyPointRepository) Update(ctx context.Context, kp *models.KeyPoint) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE key_points SET position = ?, content = ? WHERE id = ?`,
		kp.Position, kp.Content, kp.ID,
	)
	if err != nil {
		return fmt.Errorf("update key point: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update key point rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *KeyPointRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM key_points WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete key point: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete key point rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *KeyPointRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM key_points WHERE meeting_id = ?`, meetingID)
	if err != nil {
		return fmt.Errorf("delete key points by meeting: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/repository/ -run TestKeyPointRepository -v`
Expected: PASS for all 9 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/key_point_repository.go internal/repository/key_point_repository_test.go
git commit -m "feat(repository): add KeyPointRepository with CRUD and DeleteByMeetingID"
```

---

## Task 4: Task Repository

**Files:**
- Create: `internal/repository/task_repository.go`
- Create: `internal/repository/task_repository_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/repository/task_repository_test.go`:

```go
package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func openTaskTestDB(t *testing.T) *repository.TaskRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	now := time.Now().UTC()
	if err := mr.Create(context.Background(), &models.Meeting{
		ID: "m-1", Title: "Reunião", StartedAt: &now, Status: models.StatusPending,
	}); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return repository.NewTaskRepository(db)
}

func TestTaskRepository_ListByMeetingID_Empty(t *testing.T) {
	repo := openTaskTestDB(t)
	tasks, err := repo.ListByMeetingID(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0, got %d", len(tasks))
	}
	if tasks == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestTaskRepository_CreateAndGet(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	due := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	assignee := "Ana"
	task := &models.Task{
		ID: "t-1", MeetingID: "m-1", Description: "Fazer X",
		Assignee: &assignee, DueDate: &due, Priority: models.PriorityHigh, Completed: false,
	}
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "t-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Description != "Fazer X" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Assignee == nil || *got.Assignee != "Ana" {
		t.Errorf("Assignee = %v", got.Assignee)
	}
	if got.DueDate == nil || !got.DueDate.Equal(due) {
		t.Errorf("DueDate = %v, want %v", got.DueDate, due)
	}
	if got.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", got.Priority)
	}
	if got.Completed {
		t.Error("Completed should be false")
	}
}

func TestTaskRepository_Create_NullableFields(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	if err := repo.Create(ctx, &models.Task{
		ID: "t-1", MeetingID: "m-1", Description: "Sem assignee", Priority: models.PriorityMedium,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, _ := repo.GetByID(ctx, "t-1")
	if got.Assignee != nil {
		t.Errorf("Assignee should be nil, got %v", *got.Assignee)
	}
	if got.DueDate != nil {
		t.Errorf("DueDate should be nil, got %v", *got.DueDate)
	}
}

func TestTaskRepository_GetByID_NotFound(t *testing.T) {
	repo := openTaskTestDB(t)
	_, err := repo.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskRepository_ListByMeetingID_OrderedByCreatedAt(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	t1 := time.Now().UTC().Add(-2 * time.Hour)
	t2 := time.Now().UTC().Add(-1 * time.Hour)
	repo.Create(ctx, &models.Task{ID: "a", MeetingID: "m-1", Description: "Antiga", Priority: "medium", CreatedAt: t1})
	repo.Create(ctx, &models.Task{ID: "b", MeetingID: "m-1", Description: "Recente", Priority: "medium", CreatedAt: t2})

	tasks, err := repo.ListByMeetingID(ctx, "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2, got %d", len(tasks))
	}
	if tasks[0].Description != "Antiga" {
		t.Errorf("expected ASC order, first = %q", tasks[0].Description)
	}
}

func TestTaskRepository_Update(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.Task{ID: "t-1", MeetingID: "m-1", Description: "Original", Priority: "medium"})

	got, _ := repo.GetByID(ctx, "t-1")
	got.Description = "Atualizada"
	got.Completed = true
	got.Priority = models.PriorityHigh
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := repo.GetByID(ctx, "t-1")
	if updated.Description != "Atualizada" {
		t.Errorf("Description = %q", updated.Description)
	}
	if !updated.Completed {
		t.Error("Completed should be true")
	}
	if updated.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", updated.Priority)
	}
}

func TestTaskRepository_Update_NotFound(t *testing.T) {
	repo := openTaskTestDB(t)
	err := repo.Update(context.Background(), &models.Task{ID: "nope", MeetingID: "m-1", Priority: "medium"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskRepository_Delete(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.Task{ID: "t-1", MeetingID: "m-1", Description: "X", Priority: "medium"})
	if err := repo.Delete(ctx, "t-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, "t-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTaskRepository_Delete_NotFound(t *testing.T) {
	repo := openTaskTestDB(t)
	err := repo.Delete(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskRepository_DeleteByMeetingID(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	repo.Create(ctx, &models.Task{ID: "a", MeetingID: "m-1", Description: "A", Priority: "medium"})
	repo.Create(ctx, &models.Task{ID: "b", MeetingID: "m-1", Description: "B", Priority: "medium"})
	if err := repo.DeleteByMeetingID(ctx, "m-1"); err != nil {
		t.Fatalf("DeleteByMeetingID: %v", err)
	}
	tasks, _ := repo.ListByMeetingID(ctx, "m-1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 after DeleteByMeetingID, got %d", len(tasks))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/repository/ -run TestTaskRepository -v`
Expected: FAIL with "undefined: repository.TaskRepository".

- [ ] **Step 3: Implement repository**

Create `internal/repository/task_repository.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"meeting-notes/internal/models"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) ListByMeetingID(ctx context.Context, meetingID string) ([]models.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, meeting_id, description, assignee, due_date, priority, completed, created_at FROM tasks WHERE meeting_id = ? ORDER BY created_at ASC`,
		meetingID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []models.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, *t)
	}
	return tasks, rows.Err()
}

func (r *TaskRepository) Create(ctx context.Context, t *models.Task) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	var dueDate *string
	if t.DueDate != nil {
		s := t.DueDate.UTC().Format(time.RFC3339Nano)
		dueDate = &s
	}
	completed := 0
	if t.Completed {
		completed = 1
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tasks (id, meeting_id, description, assignee, due_date, priority, completed, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.MeetingID, t.Description, t.Assignee, dueDate, string(t.Priority), completed,
		t.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*models.Task, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, meeting_id, description, assignee, due_date, priority, completed, created_at FROM tasks WHERE id = ?`, id,
	)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *TaskRepository) Update(ctx context.Context, t *models.Task) error {
	var dueDate *string
	if t.DueDate != nil {
		s := t.DueDate.UTC().Format(time.RFC3339Nano)
		dueDate = &s
	}
	completed := 0
	if t.Completed {
		completed = 1
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET description = ?, assignee = ?, due_date = ?, priority = ?, completed = ? WHERE id = ?`,
		t.Description, t.Assignee, dueDate, string(t.Priority), completed, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete task rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *TaskRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tasks WHERE meeting_id = ?`, meetingID)
	if err != nil {
		return fmt.Errorf("delete tasks by meeting: %w", err)
	}
	return nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(row taskScanner) (*models.Task, error) {
	var t models.Task
	var assignee sql.NullString
	var dueDate sql.NullString
	var createdAt string
	var completedInt int64
	var priority string

	err := row.Scan(&t.ID, &t.MeetingID, &t.Description, &assignee, &dueDate, &priority, &completedInt, &createdAt)
	if err != nil {
		return nil, err
	}

	if assignee.Valid {
		v := assignee.String
		t.Assignee = &v
	}
	if dueDate.Valid {
		parsed, err := parseTime(dueDate.String)
		if err != nil {
			return nil, err
		}
		t.DueDate = &parsed
	}
	t.Priority = models.TaskPriority(priority)
	t.Completed = completedInt != 0
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &t, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/repository/ -run TestTaskRepository -v`
Expected: PASS for all 10 tests.

- [ ] **Step 5: Verify no regressions**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/task_repository.go internal/repository/task_repository_test.go
git commit -m "feat(repository): add TaskRepository with bool/int conversion and DeleteByMeetingID"
```

---

## Task 5: Summary Service

**Files:**
- Create: `internal/services/summary_service.go`
- Create: `internal/services/summary_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/services/summary_service_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeAI struct {
	summaryText string
	keyPoints   []string
	tasks       []ai.TaskSuggestion
	err         error
}

func (f *fakeAI) GenerateSummary(ctx context.Context, transcript string) (string, int, int, error) {
	if f.err != nil {
		return "", 0, 0, f.err
	}
	return f.summaryText, 100, 50, nil
}
func (f *fakeAI) GenerateKeyPoints(ctx context.Context, transcript string) ([]string, int, int, error) {
	if f.err != nil {
		return nil, 0, 0, f.err
	}
	return f.keyPoints, 100, 50, nil
}
func (f *fakeAI) GenerateTasks(ctx context.Context, transcript string) ([]ai.TaskSuggestion, int, int, error) {
	if f.err != nil {
		return nil, 0, 0, f.err
	}
	return f.tasks, 100, 50, nil
}

func newSummaryTestService(t *testing.T, aiClient ai.AIClient) (*services.SummaryService, *models.Meeting) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	transcript := "Falamos sobre o roadmap do produto."
	now := time.Now().UTC()
	meeting := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusCompleted, Transcript: &transcript}
	if err := mr.Create(context.Background(), meeting); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return services.NewSummaryService(repository.NewSummaryRepository(db), aiClient), meeting
}

func TestSummaryService_Get_NotFound(t *testing.T) {
	svc, _ := newSummaryTestService(t, nil)
	_, err := svc.Get(context.Background(), "m-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSummaryService_Upsert(t *testing.T) {
	svc, _ := newSummaryTestService(t, nil)
	got, err := svc.Upsert(context.Background(), "m-1", "Conteúdo", "manual")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got.ID == "" {
		t.Error("ID should be set")
	}
	if got.Content != "Conteúdo" {
		t.Errorf("Content = %q", got.Content)
	}
	if got.ModelUsed != "manual" {
		t.Errorf("ModelUsed = %q", got.ModelUsed)
	}
}

func TestSummaryService_Upsert_ContentRequired(t *testing.T) {
	svc, _ := newSummaryTestService(t, nil)
	_, err := svc.Upsert(context.Background(), "m-1", "", "manual")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestSummaryService_Upsert_Replaces(t *testing.T) {
	svc, _ := newSummaryTestService(t, nil)
	ctx := context.Background()
	svc.Upsert(ctx, "m-1", "Original", "manual")
	svc.Upsert(ctx, "m-1", "Substituído", "manual")
	got, _ := svc.Get(ctx, "m-1")
	if got.Content != "Substituído" {
		t.Errorf("Content = %q", got.Content)
	}
}

func TestSummaryService_Delete(t *testing.T) {
	svc, _ := newSummaryTestService(t, nil)
	ctx := context.Background()
	svc.Upsert(ctx, "m-1", "X", "manual")
	if err := svc.Delete(ctx, "m-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := svc.Get(ctx, "m-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSummaryService_Generate(t *testing.T) {
	fake := &fakeAI{summaryText: "Resumo gerado pela AI"}
	svc, meeting := newSummaryTestService(t, fake)
	got, err := svc.Generate(context.Background(), meeting)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got.Content != "Resumo gerado pela AI" {
		t.Errorf("Content = %q", got.Content)
	}
	if got.InputTokens != 100 || got.OutputTokens != 50 {
		t.Errorf("Tokens = %d/%d", got.InputTokens, got.OutputTokens)
	}
}

func TestSummaryService_Generate_NoTranscript(t *testing.T) {
	fake := &fakeAI{summaryText: "x"}
	svc, meeting := newSummaryTestService(t, fake)
	meeting.Transcript = nil
	_, err := svc.Generate(context.Background(), meeting)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestSummaryService_Generate_AINotConfigured(t *testing.T) {
	svc, meeting := newSummaryTestService(t, nil)
	_, err := svc.Generate(context.Background(), meeting)
	if !errors.Is(err, services.ErrAINotConfigured) {
		t.Errorf("expected ErrAINotConfigured, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/services/ -run TestSummaryService -v`
Expected: FAIL with "undefined: services.SummaryService" / "undefined: services.ErrAINotConfigured".

- [ ] **Step 3: Implement service**

Create `internal/services/summary_service.go`:

```go
package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

var ErrAINotConfigured = errors.New("AI client not configured")

type SummaryService struct {
	repo *repository.SummaryRepository
	ai   ai.AIClient
}

func NewSummaryService(repo *repository.SummaryRepository, aiClient ai.AIClient) *SummaryService {
	return &SummaryService{repo: repo, ai: aiClient}
}

func (s *SummaryService) Get(ctx context.Context, meetingID string) (*models.Summary, error) {
	return s.repo.GetByMeetingID(ctx, meetingID)
}

func (s *SummaryService) Upsert(ctx context.Context, meetingID, content, modelUsed string) (*models.Summary, error) {
	if content == "" {
		return nil, &ValidationError{"content is required"}
	}
	summary := &models.Summary{
		ID:        uuid.New().String(),
		MeetingID: meetingID,
		Content:   content,
		ModelUsed: modelUsed,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Upsert(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *SummaryService) Delete(ctx context.Context, meetingID string) error {
	return s.repo.Delete(ctx, meetingID)
}

func (s *SummaryService) Generate(ctx context.Context, meeting *models.Meeting) (*models.Summary, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}
	if meeting.Transcript == nil || *meeting.Transcript == "" {
		return nil, &ValidationError{"transcript is required for generation"}
	}
	content, inputTokens, outputTokens, err := s.ai.GenerateSummary(ctx, *meeting.Transcript)
	if err != nil {
		return nil, err
	}
	summary := &models.Summary{
		ID:           uuid.New().String(),
		MeetingID:    meeting.ID,
		Content:      content,
		ModelUsed:    "anthropic",
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Upsert(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/services/ -run TestSummaryService -v`
Expected: PASS for all 8 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/summary_service.go internal/services/summary_service_test.go
git commit -m "feat(services): add SummaryService with AI generation and ErrAINotConfigured"
```

---

## Task 6: Key Point Service

**Files:**
- Create: `internal/services/key_point_service.go`
- Create: `internal/services/key_point_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/services/key_point_service_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newKeyPointTestService(t *testing.T, aiClient ai.AIClient) (*services.KeyPointService, *models.Meeting) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	transcript := "Discutimos prioridades."
	now := time.Now().UTC()
	meeting := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusCompleted, Transcript: &transcript}
	if err := mr.Create(context.Background(), meeting); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return services.NewKeyPointService(repository.NewKeyPointRepository(db), aiClient), meeting
}

func TestKeyPointService_List_Empty(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	got, err := svc.List(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestKeyPointService_Create(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	got, err := svc.Create(context.Background(), "m-1", "Ponto importante", 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID == "" {
		t.Error("ID should be set")
	}
	if got.Content != "Ponto importante" {
		t.Errorf("Content = %q", got.Content)
	}
}

func TestKeyPointService_Create_ContentRequired(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	_, err := svc.Create(context.Background(), "m-1", "", 0)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestKeyPointService_Update(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "Original", 0)
	updated, err := svc.Update(ctx, created.ID, "Atualizado", 5)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Content != "Atualizado" {
		t.Errorf("Content = %q", updated.Content)
	}
	if updated.Position != 5 {
		t.Errorf("Position = %d", updated.Position)
	}
}

func TestKeyPointService_Update_NotFound(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	_, err := svc.Update(context.Background(), "nope", "x", 0)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestKeyPointService_Update_ContentRequired(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "Original", 0)
	_, err := svc.Update(ctx, created.ID, "", 0)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestKeyPointService_Delete(t *testing.T) {
	svc, _ := newKeyPointTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "X", 0)
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestKeyPointService_Generate(t *testing.T) {
	fake := &fakeAI{keyPoints: []string{"Primeiro", "Segundo", "Terceiro"}}
	svc, meeting := newKeyPointTestService(t, fake)
	got, err := svc.Generate(context.Background(), meeting)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 key points, got %d", len(got))
	}
	if got[0].Content != "Primeiro" || got[0].Position != 0 {
		t.Errorf("first kp wrong: %+v", got[0])
	}
	if got[2].Content != "Terceiro" || got[2].Position != 2 {
		t.Errorf("third kp wrong: %+v", got[2])
	}
}

func TestKeyPointService_Generate_ReplacesExisting(t *testing.T) {
	fake := &fakeAI{keyPoints: []string{"Novo"}}
	svc, meeting := newKeyPointTestService(t, fake)
	ctx := context.Background()
	svc.Create(ctx, "m-1", "Manual", 0)

	got, err := svc.Generate(ctx, meeting)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(got) != 1 || got[0].Content != "Novo" {
		t.Errorf("expected only [Novo], got %+v", got)
	}

	all, _ := svc.List(ctx, "m-1")
	if len(all) != 1 {
		t.Errorf("expected 1 in DB after replace, got %d", len(all))
	}
}

func TestKeyPointService_Generate_NoTranscript(t *testing.T) {
	fake := &fakeAI{keyPoints: []string{"x"}}
	svc, meeting := newKeyPointTestService(t, fake)
	meeting.Transcript = nil
	_, err := svc.Generate(context.Background(), meeting)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestKeyPointService_Generate_AINotConfigured(t *testing.T) {
	svc, meeting := newKeyPointTestService(t, nil)
	_, err := svc.Generate(context.Background(), meeting)
	if !errors.Is(err, services.ErrAINotConfigured) {
		t.Errorf("expected ErrAINotConfigured, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/services/ -run TestKeyPointService -v`
Expected: FAIL with "undefined: services.KeyPointService".

- [ ] **Step 3: Implement service**

Create `internal/services/key_point_service.go`:

```go
package services

import (
	"context"

	"github.com/google/uuid"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type KeyPointService struct {
	repo *repository.KeyPointRepository
	ai   ai.AIClient
}

func NewKeyPointService(repo *repository.KeyPointRepository, aiClient ai.AIClient) *KeyPointService {
	return &KeyPointService{repo: repo, ai: aiClient}
}

func (s *KeyPointService) List(ctx context.Context, meetingID string) ([]models.KeyPoint, error) {
	return s.repo.ListByMeetingID(ctx, meetingID)
}

func (s *KeyPointService) Create(ctx context.Context, meetingID, content string, position int) (*models.KeyPoint, error) {
	if content == "" {
		return nil, &ValidationError{"content is required"}
	}
	kp := &models.KeyPoint{
		ID:        uuid.New().String(),
		MeetingID: meetingID,
		Position:  position,
		Content:   content,
	}
	if err := s.repo.Create(ctx, kp); err != nil {
		return nil, err
	}
	return kp, nil
}

func (s *KeyPointService) Update(ctx context.Context, id, content string, position int) (*models.KeyPoint, error) {
	if content == "" {
		return nil, &ValidationError{"content is required"}
	}
	kp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	kp.Content = content
	kp.Position = position
	if err := s.repo.Update(ctx, kp); err != nil {
		return nil, err
	}
	return kp, nil
}

func (s *KeyPointService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *KeyPointService) Generate(ctx context.Context, meeting *models.Meeting) ([]models.KeyPoint, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}
	if meeting.Transcript == nil || *meeting.Transcript == "" {
		return nil, &ValidationError{"transcript is required for generation"}
	}
	points, _, _, err := s.ai.GenerateKeyPoints(ctx, *meeting.Transcript)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteByMeetingID(ctx, meeting.ID); err != nil {
		return nil, err
	}
	created := make([]models.KeyPoint, 0, len(points))
	for i, content := range points {
		kp := models.KeyPoint{
			ID:        uuid.New().String(),
			MeetingID: meeting.ID,
			Position:  i,
			Content:   content,
		}
		if err := s.repo.Create(ctx, &kp); err != nil {
			return nil, err
		}
		created = append(created, kp)
	}
	return created, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/services/ -run TestKeyPointService -v`
Expected: PASS for all 11 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/services/key_point_service.go internal/services/key_point_service_test.go
git commit -m "feat(services): add KeyPointService with AI generation that replaces existing"
```

---

## Task 7: Task Service

**Files:**
- Create: `internal/services/task_service.go`
- Create: `internal/services/task_service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/services/task_service_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTaskTestService(t *testing.T, aiClient ai.AIClient) (*services.TaskService, *models.Meeting) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	transcript := "Definimos ações."
	now := time.Now().UTC()
	meeting := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusCompleted, Transcript: &transcript}
	if err := mr.Create(context.Background(), meeting); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return services.NewTaskService(repository.NewTaskRepository(db), aiClient), meeting
}

func TestTaskService_List_Empty(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	got, err := svc.List(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestTaskService_Create(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	got, err := svc.Create(context.Background(), "m-1", "Fazer X", nil, nil, "high")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID == "" {
		t.Error("ID should be set")
	}
	if got.Description != "Fazer X" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", got.Priority)
	}
	if got.Completed {
		t.Error("Completed should default to false")
	}
}

func TestTaskService_Create_DefaultPriority(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	got, _ := svc.Create(context.Background(), "m-1", "Fazer X", nil, nil, "")
	if got.Priority != models.PriorityMedium {
		t.Errorf("Priority = %q, want medium (default)", got.Priority)
	}
}

func TestTaskService_Create_DescriptionRequired(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	_, err := svc.Create(context.Background(), "m-1", "", nil, nil, "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Create_InvalidPriority(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	_, err := svc.Create(context.Background(), "m-1", "X", nil, nil, "urgent")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Update(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "Original", nil, nil, "low")
	updated, err := svc.Update(ctx, created.ID, "Atualizada", nil, nil, "high", true)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "Atualizada" {
		t.Errorf("Description = %q", updated.Description)
	}
	if !updated.Completed {
		t.Error("Completed should be true")
	}
	if updated.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", updated.Priority)
	}
}

func TestTaskService_Update_NotFound(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	_, err := svc.Update(context.Background(), "nope", "X", nil, nil, "low", false)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskService_Update_DescriptionRequired(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "Original", nil, nil, "low")
	_, err := svc.Update(ctx, created.ID, "", nil, nil, "low", false)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Delete(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "X", nil, nil, "low")
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestTaskService_Generate(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{
		{Description: "Task 1", Assignee: "Ana", Priority: "high"},
		{Description: "Task 2", Assignee: "", Priority: "low"},
	}}
	svc, meeting := newTaskTestService(t, fake)
	got, err := svc.Generate(context.Background(), meeting)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(got))
	}
	if got[0].Description != "Task 1" {
		t.Errorf("first task wrong: %+v", got[0])
	}
	if got[0].Assignee == nil || *got[0].Assignee != "Ana" {
		t.Errorf("first assignee wrong: %v", got[0].Assignee)
	}
	if got[1].Assignee != nil {
		t.Errorf("empty assignee should be nil, got %v", got[1].Assignee)
	}
	if got[1].Priority != models.PriorityLow {
		t.Errorf("second priority wrong: %q", got[1].Priority)
	}
}

func TestTaskService_Generate_ReplacesExisting(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{{Description: "Nova", Priority: "medium"}}}
	svc, meeting := newTaskTestService(t, fake)
	ctx := context.Background()
	svc.Create(ctx, "m-1", "Manual", nil, nil, "low")

	if _, err := svc.Generate(ctx, meeting); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	all, _ := svc.List(ctx, "m-1")
	if len(all) != 1 {
		t.Errorf("expected 1 in DB after replace, got %d", len(all))
	}
	if all[0].Description != "Nova" {
		t.Errorf("expected only Nova, got %q", all[0].Description)
	}
}

func TestTaskService_Generate_NoTranscript(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{{Description: "x"}}}
	svc, meeting := newTaskTestService(t, fake)
	meeting.Transcript = nil
	_, err := svc.Generate(context.Background(), meeting)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Generate_AINotConfigured(t *testing.T) {
	svc, meeting := newTaskTestService(t, nil)
	_, err := svc.Generate(context.Background(), meeting)
	if !errors.Is(err, services.ErrAINotConfigured) {
		t.Errorf("expected ErrAINotConfigured, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/services/ -run TestTaskService -v`
Expected: FAIL with "undefined: services.TaskService".

- [ ] **Step 3: Implement service**

Create `internal/services/task_service.go`:

```go
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

var validTaskPriorities = map[string]bool{
	string(models.PriorityLow):    true,
	string(models.PriorityMedium): true,
	string(models.PriorityHigh):   true,
}

type TaskService struct {
	repo *repository.TaskRepository
	ai   ai.AIClient
}

func NewTaskService(repo *repository.TaskRepository, aiClient ai.AIClient) *TaskService {
	return &TaskService{repo: repo, ai: aiClient}
}

func (s *TaskService) List(ctx context.Context, meetingID string) ([]models.Task, error) {
	return s.repo.ListByMeetingID(ctx, meetingID)
}

func (s *TaskService) Create(ctx context.Context, meetingID, description string, assignee *string, dueDate *time.Time, priority string) (*models.Task, error) {
	if description == "" {
		return nil, &ValidationError{"description is required"}
	}
	if priority == "" {
		priority = string(models.PriorityMedium)
	} else if !validTaskPriorities[priority] {
		return nil, &ValidationError{"invalid priority: must be one of low, medium, high"}
	}
	task := &models.Task{
		ID:          uuid.New().String(),
		MeetingID:   meetingID,
		Description: description,
		Assignee:    assignee,
		DueDate:     dueDate,
		Priority:    models.TaskPriority(priority),
		Completed:   false,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskService) Update(ctx context.Context, id, description string, assignee *string, dueDate *time.Time, priority string, completed bool) (*models.Task, error) {
	if description == "" {
		return nil, &ValidationError{"description is required"}
	}
	if priority != "" && !validTaskPriorities[priority] {
		return nil, &ValidationError{"invalid priority: must be one of low, medium, high"}
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	task.Description = description
	task.Assignee = assignee
	task.DueDate = dueDate
	if priority != "" {
		task.Priority = models.TaskPriority(priority)
	}
	task.Completed = completed
	if err := s.repo.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *TaskService) Generate(ctx context.Context, meeting *models.Meeting) ([]models.Task, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}
	if meeting.Transcript == nil || *meeting.Transcript == "" {
		return nil, &ValidationError{"transcript is required for generation"}
	}
	suggestions, _, _, err := s.ai.GenerateTasks(ctx, *meeting.Transcript)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteByMeetingID(ctx, meeting.ID); err != nil {
		return nil, err
	}
	created := make([]models.Task, 0, len(suggestions))
	for _, sug := range suggestions {
		priority := sug.Priority
		if priority == "" || !validTaskPriorities[priority] {
			priority = string(models.PriorityMedium)
		}
		var assignee *string
		if sug.Assignee != "" {
			a := sug.Assignee
			assignee = &a
		}
		task := models.Task{
			ID:          uuid.New().String(),
			MeetingID:   meeting.ID,
			Description: sug.Description,
			Assignee:    assignee,
			Priority:    models.TaskPriority(priority),
			Completed:   false,
			CreatedAt:   time.Now().UTC(),
		}
		if err := s.repo.Create(ctx, &task); err != nil {
			return nil, err
		}
		created = append(created, task)
	}
	return created, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/services/ -run TestTaskService -v`
Expected: PASS for all 13 tests.

- [ ] **Step 5: Verify no regressions**

Run: `go test ./...`
Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/services/task_service.go internal/services/task_service_test.go
git commit -m "feat(services): add TaskService with AI generation and priority validation"
```

---

## Task 8: Summary Handler

**Files:**
- Create: `internal/handlers/summary_handler.go`
- Create: `internal/handlers/summary_handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/handlers/summary_handler_test.go`:

```go
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeSummaryAI struct {
	text string
	err  error
}

func (f *fakeSummaryAI) GenerateSummary(ctx context.Context, transcript string) (string, int, int, error) {
	if f.err != nil {
		return "", 0, 0, f.err
	}
	return f.text, 100, 50, nil
}
func (f *fakeSummaryAI) GenerateKeyPoints(ctx context.Context, transcript string) ([]string, int, int, error) {
	return nil, 0, 0, nil
}
func (f *fakeSummaryAI) GenerateTasks(ctx context.Context, transcript string) ([]ai.TaskSuggestion, int, int, error) {
	return nil, 0, 0, nil
}

func newTestSummaryHandler(t *testing.T, aiClient ai.AIClient) (*handlers.SummaryHandler, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingSvc := services.NewMeetingService(repository.NewMeetingRepository(db))
	transcript := "Falamos sobre o roadmap."
	m, err := meetingSvc.Create(context.Background(), "Reunião", "", "completed", nil)
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	// Set transcript directly via repo since service Create doesn't accept it
	m.Transcript = &transcript
	if err := repository.NewMeetingRepository(db).Update(context.Background(), m); err != nil {
		t.Fatalf("set transcript: %v", err)
	}

	summarySvc := services.NewSummaryService(repository.NewSummaryRepository(db), aiClient)
	h := handlers.NewSummaryHandler(summarySvc, meetingSvc)
	return h, m.ID
}

func TestSummaryHandler_Get_NotFound(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSummaryHandler_Create(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	body := `{"content":"Resumo manual","model_used":"manual"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(body)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var s models.Summary
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Content != "Resumo manual" {
		t.Errorf("Content = %q", s.Content)
	}
}

func TestSummaryHandler_Create_ContentRequired(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(`{"model_used":"x"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestSummaryHandler_Get(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(`{"content":"X","model_used":"m"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestSummaryHandler_Delete(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(`{"content":"X","model_used":"m"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestSummaryHandler_Delete_NotFound(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSummaryHandler_Generate(t *testing.T) {
	fake := &fakeSummaryAI{text: "Resumo via AI"}
	h, mID := newTestSummaryHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var s models.Summary
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Content != "Resumo via AI" {
		t.Errorf("Content = %q", s.Content)
	}
}

func TestSummaryHandler_Generate_AINotConfigured(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestSummaryHandler_Generate_MeetingNotFound(t *testing.T) {
	fake := &fakeSummaryAI{text: "x"}
	h, _ := newTestSummaryHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/nope/summary/generate", nil), "nope")
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/ -run TestSummaryHandler -v`
Expected: FAIL with "undefined: handlers.SummaryHandler".

- [ ] **Step 3: Implement handler**

Create `internal/handlers/summary_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type SummaryHandler struct {
	svc        *services.SummaryService
	meetingSvc *services.MeetingService
}

func NewSummaryHandler(svc *services.SummaryService, meetingSvc *services.MeetingService) *SummaryHandler {
	return &SummaryHandler{svc: svc, meetingSvc: meetingSvc}
}

type createSummaryRequest struct {
	Content   string `json:"content"`
	ModelUsed string `json:"model_used"`
}

func (h *SummaryHandler) Get(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	s, err := h.svc.Get(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "summary not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get summary")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SummaryHandler) Create(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req createSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.Upsert(r.Context(), meetingID, req.Content, req.ModelUsed)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create summary")
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *SummaryHandler) Update(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req createSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.Upsert(r.Context(), meetingID, req.Content, req.ModelUsed)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update summary")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SummaryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), meetingID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "summary not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete summary")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SummaryHandler) Generate(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	meeting, err := h.meetingSvc.GetByID(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	s, err := h.svc.Generate(r.Context(), meeting)
	if err != nil {
		if errors.Is(err, services.ErrAINotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "AI service not configured")
			return
		}
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "AI generation failed")
		return
	}
	writeJSON(w, http.StatusCreated, s)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/ -run TestSummaryHandler -v`
Expected: PASS for all 9 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/summary_handler.go internal/handlers/summary_handler_test.go
git commit -m "feat(handlers): add SummaryHandler with CRUD and AI generation"
```

---

## Task 9: Key Point Handler

**Files:**
- Create: `internal/handlers/key_point_handler.go`
- Create: `internal/handlers/key_point_handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/handlers/key_point_handler_test.go`:

```go
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeKeyPointAI struct {
	points []string
}

func (f *fakeKeyPointAI) GenerateSummary(ctx context.Context, transcript string) (string, int, int, error) {
	return "", 0, 0, nil
}
func (f *fakeKeyPointAI) GenerateKeyPoints(ctx context.Context, transcript string) ([]string, int, int, error) {
	return f.points, 100, 50, nil
}
func (f *fakeKeyPointAI) GenerateTasks(ctx context.Context, transcript string) ([]ai.TaskSuggestion, int, int, error) {
	return nil, 0, 0, nil
}

func withChiIDAndKpID(req *http.Request, id, kpID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	rctx.URLParams.Add("kpId", kpID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newTestKeyPointHandler(t *testing.T, aiClient ai.AIClient) (*handlers.KeyPointHandler, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingSvc := services.NewMeetingService(repository.NewMeetingRepository(db))
	transcript := "x"
	m, _ := meetingSvc.Create(context.Background(), "R", "", "completed", nil)
	m.Transcript = &transcript
	repository.NewMeetingRepository(db).Update(context.Background(), m)

	kpSvc := services.NewKeyPointService(repository.NewKeyPointRepository(db), aiClient)
	return handlers.NewKeyPointHandler(kpSvc, meetingSvc), m.ID
}

func TestKeyPointHandler_List_Empty(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/key_points", nil), mID)
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var kps []models.KeyPoint
	if err := json.NewDecoder(w.Body).Decode(&kps); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(kps) != 0 {
		t.Errorf("expected 0, got %d", len(kps))
	}
}

func TestKeyPointHandler_Create(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	body := `{"position":0,"content":"Ponto 1"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(body)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var kp models.KeyPoint
	json.NewDecoder(w.Body).Decode(&kp)
	if kp.Content != "Ponto 1" {
		t.Errorf("Content = %q", kp.Content)
	}
}

func TestKeyPointHandler_Create_ContentRequired(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(`{"position":0}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestKeyPointHandler_Update(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(`{"position":0,"content":"Original"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.KeyPoint
	json.NewDecoder(wC.Body).Decode(&created)

	body := `{"position":5,"content":"Atualizado"}`
	req := withChiIDAndKpID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/key_points/"+created.ID, bytes.NewBufferString(body)), mID, created.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestKeyPointHandler_Update_NotFound(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiIDAndKpID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/key_points/nope", bytes.NewBufferString(`{"content":"x"}`)), mID, "nope")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestKeyPointHandler_Delete(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(`{"content":"X"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.KeyPoint
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiIDAndKpID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/key_points/"+created.ID, nil), mID, created.ID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestKeyPointHandler_Delete_NotFound(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiIDAndKpID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/key_points/nope", nil), mID, "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestKeyPointHandler_Generate(t *testing.T) {
	fake := &fakeKeyPointAI{points: []string{"P1", "P2"}}
	h, mID := newTestKeyPointHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var kps []models.KeyPoint
	json.NewDecoder(w.Body).Decode(&kps)
	if len(kps) != 2 {
		t.Errorf("expected 2 kps, got %d", len(kps))
	}
}

func TestKeyPointHandler_Generate_AINotConfigured(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/ -run TestKeyPointHandler -v`
Expected: FAIL with "undefined: handlers.KeyPointHandler".

- [ ] **Step 3: Implement handler**

Create `internal/handlers/key_point_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type KeyPointHandler struct {
	svc        *services.KeyPointService
	meetingSvc *services.MeetingService
}

func NewKeyPointHandler(svc *services.KeyPointService, meetingSvc *services.MeetingService) *KeyPointHandler {
	return &KeyPointHandler{svc: svc, meetingSvc: meetingSvc}
}

type keyPointRequest struct {
	Position int    `json:"position"`
	Content  string `json:"content"`
}

func (h *KeyPointHandler) List(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	kps, err := h.svc.List(r.Context(), meetingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list key points")
		return
	}
	writeJSON(w, http.StatusOK, kps)
}

func (h *KeyPointHandler) Create(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req keyPointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kp, err := h.svc.Create(r.Context(), meetingID, req.Content, req.Position)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create key point")
		return
	}
	writeJSON(w, http.StatusCreated, kp)
}

func (h *KeyPointHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "kpId")
	var req keyPointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kp, err := h.svc.Update(r.Context(), id, req.Content, req.Position)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "key point not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update key point")
		return
	}
	writeJSON(w, http.StatusOK, kp)
}

func (h *KeyPointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "kpId")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "key point not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete key point")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KeyPointHandler) Generate(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	meeting, err := h.meetingSvc.GetByID(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	kps, err := h.svc.Generate(r.Context(), meeting)
	if err != nil {
		if errors.Is(err, services.ErrAINotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "AI service not configured")
			return
		}
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "AI generation failed")
		return
	}
	writeJSON(w, http.StatusCreated, kps)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/ -run TestKeyPointHandler -v`
Expected: PASS for all 9 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/key_point_handler.go internal/handlers/key_point_handler_test.go
git commit -m "feat(handlers): add KeyPointHandler with CRUD and AI generation"
```

---

## Task 10: Task Handler

**Files:**
- Create: `internal/handlers/task_handler.go`
- Create: `internal/handlers/task_handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/handlers/task_handler_test.go`:

```go
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeTaskAI struct {
	tasks []ai.TaskSuggestion
}

func (f *fakeTaskAI) GenerateSummary(ctx context.Context, transcript string) (string, int, int, error) {
	return "", 0, 0, nil
}
func (f *fakeTaskAI) GenerateKeyPoints(ctx context.Context, transcript string) ([]string, int, int, error) {
	return nil, 0, 0, nil
}
func (f *fakeTaskAI) GenerateTasks(ctx context.Context, transcript string) ([]ai.TaskSuggestion, int, int, error) {
	return f.tasks, 100, 50, nil
}

func withChiIDAndTaskID(req *http.Request, id, taskID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	rctx.URLParams.Add("taskId", taskID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newTestTaskHandler(t *testing.T, aiClient ai.AIClient) (*handlers.TaskHandler, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingSvc := services.NewMeetingService(repository.NewMeetingRepository(db))
	transcript := "x"
	m, _ := meetingSvc.Create(context.Background(), "R", "", "completed", nil)
	m.Transcript = &transcript
	repository.NewMeetingRepository(db).Update(context.Background(), m)

	taskSvc := services.NewTaskService(repository.NewTaskRepository(db), aiClient)
	return handlers.NewTaskHandler(taskSvc, meetingSvc), m.ID
}

func TestTaskHandler_List_Empty(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/tasks", nil), mID)
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var tasks []models.Task
	json.NewDecoder(w.Body).Decode(&tasks)
	if len(tasks) != 0 {
		t.Errorf("expected 0, got %d", len(tasks))
	}
}

func TestTaskHandler_Create(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	body := `{"description":"Fazer X","priority":"high"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(body)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Description != "Fazer X" {
		t.Errorf("Description = %q", task.Description)
	}
	if task.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", task.Priority)
	}
}

func TestTaskHandler_Create_DescriptionRequired(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"priority":"low"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestTaskHandler_Create_InvalidPriority(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"X","priority":"urgent"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestTaskHandler_Create_InvalidDueDate(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"X","due_date":"not-a-date"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTaskHandler_Update(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"Original","priority":"low"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.Task
	json.NewDecoder(wC.Body).Decode(&created)

	body := `{"description":"Atualizada","priority":"high","completed":true}`
	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/tasks/"+created.ID, bytes.NewBufferString(body)), mID, created.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated models.Task
	json.NewDecoder(w.Body).Decode(&updated)
	if !updated.Completed {
		t.Error("Completed should be true")
	}
}

func TestTaskHandler_Update_NotFound(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/tasks/nope", bytes.NewBufferString(`{"description":"X"}`)), mID, "nope")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTaskHandler_Delete(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"X"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.Task
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/tasks/"+created.ID, nil), mID, created.ID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestTaskHandler_Delete_NotFound(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/tasks/nope", nil), mID, "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTaskHandler_Generate(t *testing.T) {
	fake := &fakeTaskAI{tasks: []ai.TaskSuggestion{
		{Description: "T1", Priority: "high"},
		{Description: "T2", Priority: "low"},
	}}
	h, mID := newTestTaskHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var tasks []models.Task
	json.NewDecoder(w.Body).Decode(&tasks)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTaskHandler_Generate_AINotConfigured(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handlers/ -run TestTaskHandler -v`
Expected: FAIL with "undefined: handlers.TaskHandler".

- [ ] **Step 3: Implement handler**

Create `internal/handlers/task_handler.go`:

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type TaskHandler struct {
	svc        *services.TaskService
	meetingSvc *services.MeetingService
}

func NewTaskHandler(svc *services.TaskService, meetingSvc *services.MeetingService) *TaskHandler {
	return &TaskHandler{svc: svc, meetingSvc: meetingSvc}
}

type createTaskRequest struct {
	Description string  `json:"description"`
	Assignee    *string `json:"assignee"`
	DueDate     *string `json:"due_date"`
	Priority    string  `json:"priority"`
}

type updateTaskRequest struct {
	Description string  `json:"description"`
	Assignee    *string `json:"assignee"`
	DueDate     *string `json:"due_date"`
	Priority    string  `json:"priority"`
	Completed   bool    `json:"completed"`
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	tasks, err := h.svc.List(r.Context(), meetingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dueDate, err := parseOptionalRFC3339(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_date: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
		return
	}
	task, err := h.svc.Create(r.Context(), meetingID, req.Description, req.Assignee, dueDate, req.Priority)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskId")
	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dueDate, err := parseOptionalRFC3339(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_date: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
		return
	}
	task, err := h.svc.Update(r.Context(), id, req.Description, req.Assignee, dueDate, req.Priority, req.Completed)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskId")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) Generate(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	meeting, err := h.meetingSvc.GetByID(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	tasks, err := h.svc.Generate(r.Context(), meeting)
	if err != nil {
		if errors.Is(err, services.ErrAINotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "AI service not configured")
			return
		}
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "AI generation failed")
		return
	}
	writeJSON(w, http.StatusCreated, tasks)
}

func parseOptionalRFC3339(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handlers/ -run TestTaskHandler -v`
Expected: PASS for all 11 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/task_handler.go internal/handlers/task_handler_test.go
git commit -m "feat(handlers): add TaskHandler with CRUD, due_date parsing, and AI generation"
```

---

## Task 11: Wire Up Meeting Detail + Routes

**Files:**
- Modify: `internal/handlers/meeting_handler.go` (update `NewMeetingHandler` and `GetByID`)
- Modify: `internal/handlers/meeting_handler_test.go` (update `newTestMeetingHandler` and `newTestMeetingAndThemeHandlers`)
- Modify: `cmd/api/main.go` (wire up new handlers and register routes)

This task is the integration step that ties everything together. The `MeetingHandler.GetByID` currently returns empty arrays — it needs the three new repositories to populate real data. Wiring routes also requires the new handlers and the AI client.

- [ ] **Step 1: Update `MeetingHandler` constructor and `GetByID`**

Edit `internal/handlers/meeting_handler.go`. Change the struct, constructor, and `GetByID`:

```go
type MeetingHandler struct {
	svc          *services.MeetingService
	summaryRepo  *repository.SummaryRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
}

func NewMeetingHandler(
	svc *services.MeetingService,
	summaryRepo *repository.SummaryRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
) *MeetingHandler {
	return &MeetingHandler{
		svc:          svc,
		summaryRepo:  summaryRepo,
		keyPointRepo: keyPointRepo,
		taskRepo:     taskRepo,
	}
}
```

Update `GetByID` to populate the nested data:

```go
func (h *MeetingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}

	var summary *models.Summary
	s, err := h.summaryRepo.GetByMeetingID(r.Context(), id)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "failed to get summary")
		return
	}
	if err == nil {
		summary = s
	}

	keyPoints, err := h.keyPointRepo.ListByMeetingID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get key points")
		return
	}
	tasks, err := h.taskRepo.ListByMeetingID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get tasks")
		return
	}

	writeJSON(w, http.StatusOK, MeetingDetailResponse{
		Meeting:   *m,
		Summary:   summary,
		KeyPoints: keyPoints,
		Tasks:     tasks,
	})
}
```

- [ ] **Step 2: Update `meeting_handler_test.go` test helpers**

Edit `internal/handlers/meeting_handler_test.go`. Update `newTestMeetingHandler` to provide the new repositories:

```go
func newTestMeetingHandler(t *testing.T) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db)),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
	)
}
```

And update `newTestMeetingAndThemeHandlers`:

```go
func newTestMeetingAndThemeHandlers(t *testing.T) (*handlers.MeetingHandler, *handlers.ThemeHandler) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mh := handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db)),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
	)
	th := handlers.NewThemeHandler(services.NewThemeService(repository.NewThemeRepository(db)))
	return mh, th
}
```

- [ ] **Step 3: Add a new test for GetByID with populated data**

Append to `internal/handlers/meeting_handler_test.go`:

```go
func TestMeetingHandler_GetByID_PopulatesNestedData(t *testing.T) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mr := repository.NewMeetingRepository(db)
	sr := repository.NewSummaryRepository(db)
	kpr := repository.NewKeyPointRepository(db)
	tr := repository.NewTaskRepository(db)

	mh := handlers.NewMeetingHandler(services.NewMeetingService(mr), sr, kpr, tr)

	now := time.Now().UTC()
	m := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusCompleted}
	if err := mr.Create(context.Background(), m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	sr.Upsert(context.Background(), &models.Summary{ID: "s-1", MeetingID: "m-1", Content: "Resumo", ModelUsed: "manual"})
	kpr.Create(context.Background(), &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "Ponto"})
	tr.Create(context.Background(), &models.Task{ID: "t-1", MeetingID: "m-1", Description: "Task", Priority: models.PriorityMedium})

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/m-1", nil), "m-1")
	w := httptest.NewRecorder()
	mh.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var detail handlers.MeetingDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.Summary == nil || detail.Summary.Content != "Resumo" {
		t.Errorf("Summary = %+v", detail.Summary)
	}
	if len(detail.KeyPoints) != 1 || detail.KeyPoints[0].Content != "Ponto" {
		t.Errorf("KeyPoints = %+v", detail.KeyPoints)
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].Description != "Task" {
		t.Errorf("Tasks = %+v", detail.Tasks)
	}
}
```

This test requires importing `"context"` and `"time"` in `meeting_handler_test.go` if they aren't already imported. Check the existing imports — `context` may already be there via other tests, but `time` likely isn't. Add what's missing.

- [ ] **Step 4: Run all handler tests**

Run: `go test ./internal/handlers/ -v`
Expected: all tests pass, including the new `TestMeetingHandler_GetByID_PopulatesNestedData` and the existing `TestMeetingHandler_GetByID` (which should still pass with empty arrays since no nested data was inserted in that test).

- [ ] **Step 5: Update `cmd/api/main.go` — wire up handlers and routes**

Edit `cmd/api/main.go`. Replace the existing wiring section (the `themeHandler` and `meetingHandler` blocks) with the full wire-up:

```go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/config"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// AI client (nil-safe — generate endpoints return 503 when API key absent)
	var aiClient ai.AIClient
	if cfg.AnthropicAPIKey != "" {
		aiClient = ai.NewAnthropicClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
	}

	// Repositories
	themeRepo := repository.NewThemeRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	// Services
	themeSvc := services.NewThemeService(themeRepo)
	meetingSvc := services.NewMeetingService(meetingRepo)
	summarySvc := services.NewSummaryService(summaryRepo, aiClient)
	keyPointSvc := services.NewKeyPointService(keyPointRepo, aiClient)
	taskSvc := services.NewTaskService(taskRepo, aiClient)

	// Handlers
	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", healthHandler(db))

	r.Route("/api/themes", func(r chi.Router) {
		r.Get("/", themeHandler.List)
		r.Post("/", themeHandler.Create)
		r.Get("/{id}", themeHandler.GetByID)
		r.Put("/{id}", themeHandler.Update)
		r.Delete("/{id}", themeHandler.Delete)
	})

	r.Route("/api/meetings", func(r chi.Router) {
		r.Get("/", meetingHandler.List)
		r.Post("/", meetingHandler.Create)
		r.Get("/{id}", meetingHandler.GetByID)
		r.Put("/{id}", meetingHandler.Update)
		r.Delete("/{id}", meetingHandler.Delete)

		r.Route("/{id}/summary", func(r chi.Router) {
			r.Get("/", summaryHandler.Get)
			r.Post("/", summaryHandler.Create)
			r.Put("/", summaryHandler.Update)
			r.Delete("/", summaryHandler.Delete)
			r.Post("/generate", summaryHandler.Generate)
		})

		r.Route("/{id}/key_points", func(r chi.Router) {
			r.Get("/", keyPointHandler.List)
			r.Post("/", keyPointHandler.Create)
			r.Post("/generate", keyPointHandler.Generate)
			r.Put("/{kpId}", keyPointHandler.Update)
			r.Delete("/{kpId}", keyPointHandler.Delete)
		})

		r.Route("/{id}/tasks", func(r chi.Router) {
			r.Get("/", taskHandler.List)
			r.Post("/", taskHandler.Create)
			r.Post("/generate", taskHandler.Generate)
			r.Put("/{taskId}", taskHandler.Update)
			r.Delete("/{taskId}", taskHandler.Delete)
		})
	})

	log.Printf("server listening on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		statusCode := http.StatusOK

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			dbStatus = "error"
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": dbStatus,
		})
	}
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./cmd/api/...`
Expected: success, no errors.

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: all tests pass across all packages (repository, services, handlers, database).

- [ ] **Step 8: Commit**

```bash
git add internal/handlers/meeting_handler.go internal/handlers/meeting_handler_test.go cmd/api/main.go
git commit -m "feat: wire up summary/key_points/tasks routes and populate meeting detail"
```

---

## Final verification

After Task 11 is complete, run:

```bash
go test ./...
go build ./...
```

Both must succeed. The full route table should be:

```
GET    /health
GET    /api/themes
POST   /api/themes
GET    /api/themes/{id}
PUT    /api/themes/{id}
DELETE /api/themes/{id}
GET    /api/meetings
POST   /api/meetings
GET    /api/meetings/{id}                         (returns populated detail)
PUT    /api/meetings/{id}
DELETE /api/meetings/{id}
GET    /api/meetings/{id}/summary
POST   /api/meetings/{id}/summary
PUT    /api/meetings/{id}/summary
DELETE /api/meetings/{id}/summary
POST   /api/meetings/{id}/summary/generate
GET    /api/meetings/{id}/key_points
POST   /api/meetings/{id}/key_points
PUT    /api/meetings/{id}/key_points/{kpId}
DELETE /api/meetings/{id}/key_points/{kpId}
POST   /api/meetings/{id}/key_points/generate
GET    /api/meetings/{id}/tasks
POST   /api/meetings/{id}/tasks
PUT    /api/meetings/{id}/tasks/{taskId}
DELETE /api/meetings/{id}/tasks/{taskId}
POST   /api/meetings/{id}/tasks/generate
```
