# Global Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full-text search across all meeting content (title, transcript, summary, key_points, tasks) via SQLite FTS5, exposed through a `Ctrl+K` modal with snippet previews and highlight-on-navigate.

**Architecture:** SQLite FTS5 virtual table (`meetings_fts`) mirrors meeting content and is kept in sync by `SearchRepository` called from `MeetingService` (on update/delete) and `Orchestrator` (after AI pipeline). A new `GET /api/search?q=` endpoint returns enriched results; the frontend renders them in a portal modal with debounced input.

**Tech Stack:** Go 1.22, modernc/sqlite FTS5, chi v5, React 19, React Query v5, lucide-react, `createPortal`

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/database/migrations/009_fts.sql` | Create | FTS5 virtual table + initial population |
| `internal/repository/search_repository.go` | Create | `Search`, `UpsertMeeting`, `DeleteMeeting` |
| `internal/repository/search_repository_test.go` | Create | Repository tests against real SQLite |
| `internal/services/search_service.go` | Create | `Search` — validate, call repo, enrich with meeting data |
| `internal/services/search_service_test.go` | Create | Service tests |
| `internal/handlers/search_handler.go` | Create | `GET /api/search?q=` |
| `internal/handlers/search_handler_test.go` | Create | Handler tests |
| `internal/services/meeting_service.go` | Modify | Inject `SearchRepository`; call `UpsertMeeting` on update, `DeleteMeeting` on delete |
| `internal/services/meeting_service_test.go` | Modify | Tests for sync after update/delete |
| `internal/services/orchestrator.go` | Modify | Inject `SearchRepository`; call `UpsertMeeting` at end of `runAIGeneration` |
| `internal/services/orchestrator_test.go` | Modify | Test that FTS index is updated after pipeline |
| `cmd/api/main.go` | Modify | Wire `SearchRepository`, `SearchService`, `SearchHandler`; register route |
| `cmd/desktop/app.go` | Modify | Same wiring |
| `frontend/src/hooks/useSearch.ts` | Create | `useSearch(q)` — React Query hook |
| `frontend/src/components/search/SearchModal.tsx` | Create | Portal modal: debounced input, result list, snippet HTML |
| `frontend/src/components/layout/MeetingList.tsx` | Modify | Add Search icon button that calls `onOpenSearch` prop |
| `frontend/src/App.tsx` | Modify | `searchOpen` state, `Ctrl+K` listener, render `SearchModal`, pass `highlightQuery` to `MeetingDetail` |
| `frontend/src/components/layout/MeetingDetail.tsx` | Modify | Accept `highlightQuery?: string` prop; apply `highlightText` utility |
| `frontend/src/lib/highlight.ts` | Create | `highlightText(text, query): string` — wraps matches in `<mark>` |

---

### Task 1: Migration 009 — FTS5 virtual table

**Files:**
- Create: `internal/database/migrations/009_fts.sql`

- [ ] **Step 1: Write the migration file**

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS meetings_fts USING fts5(
    meeting_id UNINDEXED,
    title,
    transcript,
    summary,
    key_points,
    tasks,
    tokenize = 'unicode61'
);

INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
SELECT
    m.id,
    m.title,
    COALESCE(m.transcript, ''),
    COALESCE(s.content, ''),
    COALESCE((
        SELECT GROUP_CONCAT(kp.content, char(10))
        FROM key_points kp WHERE kp.meeting_id = m.id
    ), ''),
    COALESCE((
        SELECT GROUP_CONCAT(t.description, char(10))
        FROM tasks t WHERE t.meeting_id = m.id
    ), '')
FROM meetings m
LEFT JOIN summaries s ON s.meeting_id = m.id;
```

- [ ] **Step 2: Verify migration runs**

```bash
cd F:/dev/meeting-notes
go test ./internal/database/... -v -run TestOpen
```

Expected: PASS (migrations are applied automatically by `database.Open`; the test opens a DB in TempDir)

- [ ] **Step 3: Commit**

```bash
git add internal/database/migrations/009_fts.sql
git commit -m "feat: migration 009 — FTS5 virtual table for global search"
```

---

### Task 2: SearchRepository

**Files:**
- Create: `internal/repository/search_repository.go`
- Create: `internal/repository/search_repository_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/repository/search_repository_test.go`:
```go
package repository_test

import (
	"context"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
)

func newSearchRepo(t *testing.T) *repository.SearchRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewSearchRepository(db)
}

func TestSearchRepository_UpsertAndSearch(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	if err := repo.UpsertMeeting(ctx, "id-1", "Sprint Planning", "transcript text", "summary text", "Deploy API", "Write tests"); err != nil {
		t.Fatalf("UpsertMeeting: %v", err)
	}

	results, err := repo.Search(ctx, "Sprint")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MeetingID != "id-1" {
		t.Errorf("MeetingID = %q, want 'id-1'", results[0].MeetingID)
	}
	if results[0].Snippet == "" {
		t.Error("Snippet should not be empty")
	}
}

func TestSearchRepository_UpsertOverwrites(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	if err := repo.UpsertMeeting(ctx, "id-1", "Old Title", "", "", "", ""); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertMeeting(ctx, "id-1", "New Title", "", "", "", ""); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	results, err := repo.Search(ctx, "New")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after upsert, got %d", len(results))
	}

	old, err := repo.Search(ctx, "Old")
	if err != nil {
		t.Fatalf("Search old: %v", err)
	}
	if len(old) != 0 {
		t.Errorf("expected 0 results for old title after upsert, got %d", len(old))
	}
}

func TestSearchRepository_Delete(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	if err := repo.UpsertMeeting(ctx, "id-1", "To Delete", "", "", "", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.DeleteMeeting(ctx, "id-1"); err != nil {
		t.Fatalf("DeleteMeeting: %v", err)
	}

	results, err := repo.Search(ctx, "Delete")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestSearchRepository_EmptyQuery(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	_, err := repo.Search(ctx, "")
	if err == nil {
		t.Error("expected error for empty query")
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd F:/dev/meeting-notes
go test ./internal/repository/... -run TestSearchRepository -v
```

Expected: FAIL — `repository.SearchRepository` undefined

- [ ] **Step 3: Implement SearchRepository**

`internal/repository/search_repository.go`:
```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SearchResult struct {
	MeetingID string
	Snippet   string
}

type SearchRepository struct {
	db *sql.DB
}

func NewSearchRepository(db *sql.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

func (r *SearchRepository) Search(ctx context.Context, q string) ([]SearchResult, error) {
	if q == "" {
		return nil, errors.New("query must not be empty")
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT meeting_id, snippet(meetings_fts, -1, '<b>', '</b>', '...', 15) AS snippet
		 FROM meetings_fts
		 WHERE meetings_fts MATCH ?
		 ORDER BY rank
		 LIMIT 20`,
		q,
	)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.MeetingID, &r.Snippet); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *SearchRepository) UpsertMeeting(ctx context.Context, meetingID, title, transcript, summary, keyPoints, tasks string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM meetings_fts WHERE meeting_id = ?`, meetingID); err != nil {
		return fmt.Errorf("delete from fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		meetingID, title, transcript, summary, keyPoints, tasks,
	); err != nil {
		return fmt.Errorf("insert into fts: %w", err)
	}
	return tx.Commit()
}

func (r *SearchRepository) DeleteMeeting(ctx context.Context, meetingID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM meetings_fts WHERE meeting_id = ?`, meetingID)
	return err
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd F:/dev/meeting-notes
go test ./internal/repository/... -run TestSearchRepository -v
```

Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/repository/search_repository.go internal/repository/search_repository_test.go
git commit -m "feat: SearchRepository — FTS5 search, upsert, delete"
```

---

### Task 3: SearchService

**Files:**
- Create: `internal/services/search_service.go`
- Create: `internal/services/search_service_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/services/search_service_test.go`:
```go
package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newSearchService(t *testing.T) (*services.SearchService, *repository.MeetingRepository, *repository.SearchRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingRepo := repository.NewMeetingRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	svc := services.NewSearchService(searchRepo, meetingRepo)
	return svc, meetingRepo, searchRepo
}

func TestSearchService_Search(t *testing.T) {
	svc, meetingRepo, searchRepo := newSearchService(t)
	ctx := context.Background()

	now := time.Now().UTC()
	m := &models.Meeting{
		ID:        "m-1",
		Title:     "Sprint Planning",
		StartedAt: &now,
		Status:    models.StatusCompleted,
		CreatedAt: now,
	}
	if err := meetingRepo.Create(ctx, m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := searchRepo.UpsertMeeting(ctx, "m-1", "Sprint Planning", "", "", "", ""); err != nil {
		t.Fatalf("upsert fts: %v", err)
	}

	results, err := svc.Search(ctx, "Sprint")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MeetingID != "m-1" {
		t.Errorf("MeetingID = %q, want 'm-1'", results[0].MeetingID)
	}
	if results[0].MeetingTitle != "Sprint Planning" {
		t.Errorf("MeetingTitle = %q, want 'Sprint Planning'", results[0].MeetingTitle)
	}
}

func TestSearchService_Search_EmptyQuery(t *testing.T) {
	svc, _, _ := newSearchService(t)
	_, err := svc.Search(context.Background(), "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for empty query, got %T: %v", err, err)
	}
}

func TestSearchService_Search_ShortQuery(t *testing.T) {
	svc, _, _ := newSearchService(t)
	_, err := svc.Search(context.Background(), "a")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for 1-char query, got %T: %v", err, err)
	}
}

func TestSearchService_Search_NoResults(t *testing.T) {
	svc, _, _ := newSearchService(t)
	results, err := svc.Search(context.Background(), "inexistente")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd F:/dev/meeting-notes
go test ./internal/services/... -run TestSearchService -v
```

Expected: FAIL — `services.SearchService` undefined

- [ ] **Step 3: Implement SearchService**

`internal/services/search_service.go`:
```go
package services

import (
	"context"
	"fmt"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type SearchResultItem struct {
	MeetingID    string     `json:"meeting_id"`
	MeetingTitle string     `json:"meeting_title"`
	Snippet      string     `json:"snippet"`
	StartedAt    *string    `json:"started_at"`
	Status       string     `json:"status"`
}

type SearchService struct {
	searchRepo  *repository.SearchRepository
	meetingRepo *repository.MeetingRepository
}

func NewSearchService(searchRepo *repository.SearchRepository, meetingRepo *repository.MeetingRepository) *SearchService {
	return &SearchService{searchRepo: searchRepo, meetingRepo: meetingRepo}
}

func (s *SearchService) Search(ctx context.Context, q string) ([]SearchResultItem, error) {
	if len(q) < 2 {
		return nil, &ValidationError{"query must be at least 2 characters"}
	}

	raw, err := s.searchRepo.Search(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(raw) == 0 {
		return []SearchResultItem{}, nil
	}

	meetings, err := s.meetingRepo.List(ctx, repository.ListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	byID := make(map[string]*models.Meeting, len(meetings))
	for i := range meetings {
		byID[meetings[i].ID] = &meetings[i]
	}

	items := make([]SearchResultItem, 0, len(raw))
	for _, r := range raw {
		m, ok := byID[r.MeetingID]
		if !ok {
			continue
		}
		item := SearchResultItem{
			MeetingID:    r.MeetingID,
			MeetingTitle: m.Title,
			Snippet:      r.Snippet,
			Status:       string(m.Status),
		}
		if m.StartedAt != nil {
			s := m.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			item.StartedAt = &s
		}
		items = append(items, item)
	}
	return items, nil
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd F:/dev/meeting-notes
go test ./internal/services/... -run TestSearchService -v
```

Expected: PASS (4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/services/search_service.go internal/services/search_service_test.go
git commit -m "feat: SearchService — validate, search FTS, enrich with meeting data"
```

---

### Task 4: SearchHandler + routes

**Files:**
- Create: `internal/handlers/search_handler.go`
- Create: `internal/handlers/search_handler_test.go`
- Modify: `cmd/api/main.go`
- Modify: `cmd/desktop/app.go`

- [ ] **Step 1: Write the failing tests**

`internal/handlers/search_handler_test.go`:
```go
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newSearchHandler(t *testing.T) (*handlers.SearchHandler, *repository.MeetingRepository, *repository.SearchRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingRepo := repository.NewMeetingRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	svc := services.NewSearchService(searchRepo, meetingRepo)
	return handlers.NewSearchHandler(svc), meetingRepo, searchRepo
}

func TestSearchHandler_Search(t *testing.T) {
	h, meetingRepo, searchRepo := newSearchHandler(t)
	ctx := t.Context()

	now := time.Now().UTC()
	m := &models.Meeting{
		ID: "m-1", Title: "Daily Standup",
		StartedAt: &now, Status: models.StatusCompleted, CreatedAt: now,
	}
	if err := meetingRepo.Create(ctx, m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := searchRepo.UpsertMeeting(ctx, "m-1", "Daily Standup", "", "", "", ""); err != nil {
		t.Fatalf("upsert fts: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=Daily", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["meeting_id"] != "m-1" {
		t.Errorf("meeting_id = %v, want 'm-1'", results[0]["meeting_id"])
	}
}

func TestSearchHandler_MissingQuery(t *testing.T) {
	h, _, _ := newSearchHandler(t)
	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSearchHandler_ShortQuery(t *testing.T) {
	h, _, _ := newSearchHandler(t)
	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSearchHandler_EmptyResults(t *testing.T) {
	h, _, _ := newSearchHandler(t)
	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=inexistente", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "[]\n" && w.Body.String() != "[]" {
		t.Logf("body: %s", w.Body.String())
	}
	var results []any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd F:/dev/meeting-notes
go test ./internal/handlers/... -run TestSearchHandler -v
```

Expected: FAIL — `handlers.SearchHandler` undefined

- [ ] **Step 3: Implement SearchHandler**

`internal/handlers/search_handler.go`:
```go
package handlers

import (
	"errors"
	"net/http"

	"meeting-notes/internal/services"
)

type SearchHandler struct {
	svc *services.SearchService
}

func NewSearchHandler(svc *services.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 2 {
		writeError(w, http.StatusBadRequest, "q must be at least 2 characters")
		return
	}
	results, err := h.svc.Search(r.Context(), q)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, results)
}
```

- [ ] **Step 4: Run tests — verify they pass**

```bash
cd F:/dev/meeting-notes
go test ./internal/handlers/... -run TestSearchHandler -v
```

Expected: PASS (4 tests)

- [ ] **Step 5: Wire routes in both entry points**

In `cmd/api/main.go`, add after the existing repository/service declarations:

```go
searchRepo := repository.NewSearchRepository(db)
searchSvc := services.NewSearchService(searchRepo, meetingRepo)
searchHandler := handlers.NewSearchHandler(searchSvc)
```

And after the existing routes (before the final `http.ListenAndServe`):

```go
r.Get("/api/search", searchHandler.Search)
```

Apply identical changes to `cmd/desktop/app.go` in the equivalent positions.

- [ ] **Step 6: Build both entry points**

```bash
cd F:/dev/meeting-notes
go build ./cmd/api/...
go build ./cmd/desktop/...
```

Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/handlers/search_handler.go internal/handlers/search_handler_test.go cmd/api/main.go cmd/desktop/app.go
git commit -m "feat: SearchHandler and GET /api/search route"
```

---

### Task 5: MeetingService — FTS sync on update and delete

**Files:**
- Modify: `internal/services/meeting_service.go`
- Modify: `internal/services/meeting_service_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/services/meeting_service_test.go`:

```go
func newMeetingServiceWithSearch(t *testing.T) (*services.MeetingService, *repository.SearchRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingRepo := repository.NewMeetingRepository(db)
	themeRepo := repository.NewThemeRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	svc := services.NewMeetingService(meetingRepo, themeRepo, searchRepo, keyPointRepo, taskRepo, summaryRepo)
	return svc, searchRepo
}

func TestMeetingService_Update_SyncsSearch(t *testing.T) {
	svc, searchRepo := newMeetingServiceWithSearch(t)
	ctx := context.Background()

	m, err := svc.Create(ctx, "Original Title", "", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(ctx, m.ID, "Updated Title", nil, "", nil, nil, nil, nil); err != nil {
		t.Fatalf("Update: %v", err)
	}

	results, err := searchRepo.Search(ctx, "Updated")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 FTS result after update, got %d", len(results))
	}
}

func TestMeetingService_Delete_SyncsSearch(t *testing.T) {
	svc, searchRepo := newMeetingServiceWithSearch(t)
	ctx := context.Background()

	m, err := svc.Create(ctx, "To Delete", "", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := searchRepo.UpsertMeeting(ctx, m.ID, m.Title, "", "", "", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := svc.Delete(ctx, m.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	results, err := searchRepo.Search(ctx, "Delete")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 FTS results after delete, got %d", len(results))
	}
}
```

- [ ] **Step 2: Run tests — verify they fail**

```bash
cd F:/dev/meeting-notes
go test ./internal/services/... -run TestMeetingService_Update_SyncsSearch -v
go test ./internal/services/... -run TestMeetingService_Delete_SyncsSearch -v
```

Expected: FAIL — `services.NewMeetingService` signature mismatch

- [ ] **Step 3: Update MeetingService to accept and use SearchRepository**

In `internal/services/meeting_service.go`, change:

```go
// Old struct and constructor
type MeetingService struct {
	repo      *repository.MeetingRepository
	themeRepo *repository.ThemeRepository
}

func NewMeetingService(repo *repository.MeetingRepository, themeRepo *repository.ThemeRepository) *MeetingService {
	return &MeetingService{repo: repo, themeRepo: themeRepo}
}
```

To:

```go
type MeetingService struct {
	repo         *repository.MeetingRepository
	themeRepo    *repository.ThemeRepository
	searchRepo   *repository.SearchRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
	summaryRepo  *repository.SummaryRepository
}

func NewMeetingService(
	repo *repository.MeetingRepository,
	themeRepo *repository.ThemeRepository,
	searchRepo *repository.SearchRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
	summaryRepo *repository.SummaryRepository,
) *MeetingService {
	return &MeetingService{repo, themeRepo, searchRepo, keyPointRepo, taskRepo, summaryRepo}
}
```

At the end of the `Update` method, after the `repo.Update` call succeeds, add:

```go
// best-effort FTS sync — load related data for the index
go func() {
	bgCtx := context.Background()
	transcript := ""
	if m.Transcript != nil {
		transcript = *m.Transcript
	}
	summary := ""
	if s, err2 := s.summaryRepo.GetByMeetingID(bgCtx, m.ID); err2 == nil {
		summary = s.Content
	}
	kps, _ := s.keyPointRepo.ListByMeetingID(bgCtx, m.ID)
	var kpContents []string
	for _, kp := range kps {
		kpContents = append(kpContents, kp.Content)
	}
	tasks, _ := s.taskRepo.ListByMeetingID(bgCtx, m.ID)
	var taskContents []string
	for _, tk := range tasks {
		taskContents = append(taskContents, tk.Description)
	}
	_ = s.searchRepo.UpsertMeeting(bgCtx, m.ID, m.Title, transcript, summary,
		strings.Join(kpContents, "\n"), strings.Join(taskContents, "\n"))
}()
```

Add `"strings"` to the import block.

At the end of the `Delete` method, after `repo.Delete` succeeds:

```go
go func() {
	_ = s.searchRepo.DeleteMeeting(context.Background(), id)
}()
```

- [ ] **Step 4: Fix compilation — update callers of NewMeetingService**

In `cmd/api/main.go`, update the `NewMeetingService` call:

```go
meetingSvc := services.NewMeetingService(meetingRepo, themeRepo, searchRepo, keyPointRepo, taskRepo, summaryRepo)
```

In `cmd/desktop/app.go`, apply the same change.

**Note:** `searchRepo`, `keyPointRepo`, `taskRepo`, and `summaryRepo` are already declared in both entry points. The only change is passing them to `NewMeetingService`.

- [ ] **Step 5: Verify it compiles and tests pass**

```bash
cd F:/dev/meeting-notes
go build ./cmd/api/... ./cmd/desktop/...
go test ./internal/services/... -run TestMeetingService -v
```

Expected: all MeetingService tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/services/meeting_service.go internal/services/meeting_service_test.go cmd/api/main.go cmd/desktop/app.go
git commit -m "feat: MeetingService syncs FTS index on update and delete"
```

---

### Task 6: Orchestrator — FTS sync after AI pipeline

**Files:**
- Modify: `internal/services/orchestrator.go`
- Modify: `internal/services/orchestrator_test.go`
- Modify: `cmd/api/main.go`
- Modify: `cmd/desktop/app.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/services/orchestrator_test.go` (look for the existing `newOrchestrator` helper and use it):

```go
func TestOrchestrator_RunAIPipeline_SyncsSearch(t *testing.T) {
	o, db := newOrchestratorWithDB(t)
	ctx := context.Background()

	searchRepo := repository.NewSearchRepository(db)
	o.SetSearchRepo(searchRepo)

	meetingRepo := repository.NewMeetingRepository(db)
	transcript := "sprint review completed"
	m := &models.Meeting{
		ID: "m-fts", Title: "FTS Test Meeting",
		StartedAt: ptr(time.Now().UTC()), Status: models.StatusPending,
		Transcript: &transcript, CreatedAt: time.Now().UTC(),
	}
	if err := meetingRepo.Create(ctx, m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}

	if err := o.RunAIPipeline(ctx, "m-fts"); err != nil {
		t.Fatalf("RunAIPipeline: %v", err)
	}

	results, err := searchRepo.Search(ctx, "sprint")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected FTS result after pipeline, got none")
	}
}
```

Check what helpers already exist in `orchestrator_test.go` and use them. The `newOrchestratorWithDB` helper must return both the orchestrator and the `*sql.DB` so the test can create a `SearchRepository` from the same DB.

- [ ] **Step 2: Check existing orchestrator test helpers**

```bash
cd F:/dev/meeting-notes
grep -n "func new" internal/services/orchestrator_test.go
```

Read the existing helper signatures and adapt Step 1's test to match them. The key requirement is access to the same `*sql.DB` that the orchestrator's repos use, so `searchRepo.Search` queries the same FTS table.

- [ ] **Step 3: Run test — verify it fails**

```bash
cd F:/dev/meeting-notes
go test ./internal/services/... -run TestOrchestrator_RunAIPipeline_SyncsSearch -v
```

Expected: FAIL — `SetSearchRepo` undefined or similar

- [ ] **Step 4: Add searchRepo to Orchestrator**

In `internal/services/orchestrator.go`, add to the `Orchestrator` struct:

```go
searchRepo *repository.SearchRepository
```

Add a setter method:

```go
func (o *Orchestrator) SetSearchRepo(repo *repository.SearchRepository) {
	o.searchRepo = repo
}
```

At the end of `runAIGeneration`, after all AI calls succeed and before the return, add:

```go
if o.searchRepo != nil {
	go func() {
		bgCtx := context.Background()
		transcript := ""
		if m.Transcript != nil {
			transcript = *m.Transcript
		}
		summary := ""
		if s, err2 := o.summarySvc.Get(bgCtx, m.ID); err2 == nil && s != nil {
			summary = s.Content
		}
		kps, _ := o.keyPointSvc.List(bgCtx, m.ID)
		var kpContents []string
		for _, kp := range kps {
			kpContents = append(kpContents, kp.Content)
		}
		tasks, _ := o.taskSvc.List(bgCtx, m.ID)
		var taskContents []string
		for _, tk := range tasks {
			taskContents = append(taskContents, tk.Description)
		}
		_ = o.searchRepo.UpsertMeeting(bgCtx, m.ID, m.Title, transcript, summary,
			strings.Join(kpContents, "\n"), strings.Join(taskContents, "\n"))
	}()
}
```

Add `"strings"` to the import if not already present.

**Note:** `o.summarySvc.Get(ctx, meetingID)`, `o.keyPointSvc.List(ctx, meetingID)`, and `o.taskSvc.List(ctx, meetingID)` are the existing methods on these services — no new methods needed.

- [ ] **Step 5: Wire SetSearchRepo in entry points**

In `cmd/api/main.go`, after `orchestrator := services.NewOrchestrator(...)`:

```go
orchestrator.SetSearchRepo(searchRepo)
```

Same in `cmd/desktop/app.go`.

- [ ] **Step 6: Run tests**

```bash
cd F:/dev/meeting-notes
go test ./internal/services/... -run TestOrchestrator -v
go build ./cmd/api/... ./cmd/desktop/...
```

Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/services/orchestrator.go internal/services/orchestrator_test.go cmd/api/main.go cmd/desktop/app.go
git commit -m "feat: Orchestrator syncs FTS index after AI pipeline"
```

---

### Task 7: Frontend — `useSearch` hook

**Files:**
- Create: `frontend/src/hooks/useSearch.ts`

- [ ] **Step 1: Create the hook**

`frontend/src/hooks/useSearch.ts`:
```typescript
import { useQuery } from "@tanstack/react-query"
import { api } from "./useApi"

export interface SearchResultItem {
  meeting_id: string
  meeting_title: string
  snippet: string
  started_at: string | null
  status: string
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: ["search", q],
    queryFn: () => api<SearchResultItem[]>(`/api/search?q=${encodeURIComponent(q)}`),
    enabled: q.trim().length >= 2,
    staleTime: 0,
  })
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useSearch.ts
git commit -m "feat: useSearch hook — React Query wrapper for GET /api/search"
```

---

### Task 8: Frontend — `SearchModal` component

**Files:**
- Create: `frontend/src/components/search/SearchModal.tsx`
- Create: `frontend/src/lib/highlight.ts`

- [ ] **Step 1: Create the highlight utility**

`frontend/src/lib/highlight.ts`:
```typescript
export function highlightText(text: string, query: string): string {
  if (!query.trim()) return text
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
  const re = new RegExp(`(${escaped})`, "gi")
  return text.replace(re, "<mark>$1</mark>")
}
```

- [ ] **Step 2: Create SearchModal**

`frontend/src/components/search/SearchModal.tsx`:
```typescript
import { useState, useEffect, useRef } from "react"
import { createPortal } from "react-dom"
import { Search, X } from "lucide-react"
import { useSearch } from "../../hooks/useSearch"
import { Spinner } from "../ui/spinner"

interface Props {
  onClose: () => void
  onSelect: (meetingId: string, query: string) => void
}

export function SearchModal({ onClose, onSelect }: Props) {
  const [input, setInput] = useState("")
  const [q, setQ] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)
  const { data: results, isFetching } = useSearch(q)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  useEffect(() => {
    const timer = setTimeout(() => setQ(input), 200)
    return () => clearTimeout(timer)
  }, [input])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [onClose])

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-24 bg-black/50"
      onClick={onClose}
    >
      <div
        className="w-full max-w-xl bg-background border border-border rounded-lg shadow-xl overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          <Search size={16} className="text-muted-foreground shrink-0" />
          <input
            ref={inputRef}
            value={input}
            onChange={e => setInput(e.target.value)}
            placeholder="Buscar em reuniões..."
            className="flex-1 bg-transparent outline-none text-sm"
          />
          {isFetching && <Spinner size={14} />}
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X size={16} />
          </button>
        </div>

        {q.trim().length >= 2 && (
          <ul className="max-h-80 overflow-y-auto py-1">
            {results && results.length === 0 && (
              <li className="px-4 py-3 text-sm text-muted-foreground">
                Nenhum resultado para "{q}"
              </li>
            )}
            {results?.map(item => (
              <li key={item.meeting_id}>
                <button
                  className="w-full text-left px-4 py-3 hover:bg-accent transition-colors"
                  onClick={() => { onSelect(item.meeting_id, q); onClose() }}
                >
                  <p className="text-sm font-medium truncate">{item.meeting_title}</p>
                  <p
                    className="text-xs text-muted-foreground mt-0.5 line-clamp-2 [&_b]:font-semibold [&_b]:text-foreground"
                    dangerouslySetInnerHTML={{ __html: item.snippet }}
                  />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>,
    document.body,
  )
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/search/SearchModal.tsx frontend/src/lib/highlight.ts
git commit -m "feat: SearchModal component with debounce, snippet HTML, portal"
```

---

### Task 9: App.tsx — Ctrl+K listener, search icon, state

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/components/layout/MeetingList.tsx`

- [ ] **Step 1: Add search icon to MeetingList**

In `frontend/src/components/layout/MeetingList.tsx`, the `MeetingListProps` interface receives an additional prop:

```typescript
interface MeetingListProps {
  themeId: string | null
  selectedMeetingId: string | null
  onSelectMeeting: (id: string) => void
  onMeetingDeleted?: (id: string) => void
  onOpenSearch: () => void   // add this
}
```

In the component, destructure `onOpenSearch` and add a Search button in the list header (next to the existing filter/plus buttons). Find the header toolbar area and add:

```typescript
<button
  onClick={onOpenSearch}
  title="Busca global (Ctrl+K)"
  className="p-1.5 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
>
  <Search size={16} />
</button>
```

`Search` is already imported from `lucide-react` in this file.

- [ ] **Step 2: Update App.tsx**

In `frontend/src/App.tsx`:

1. Add imports:
```typescript
import { SearchModal } from "./components/search/SearchModal"
```

2. In `AppInner`, add new state:
```typescript
const [searchOpen, setSearchOpen] = useState(false)
const [highlightQuery, setHighlightQuery] = useState<string | undefined>(undefined)
```

3. Add `useEffect` for Ctrl+K (place after existing `useEffect`s):
```typescript
useEffect(() => {
  function onKey(e: KeyboardEvent) {
    if (e.key === "k" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      setSearchOpen(true)
    }
  }
  document.addEventListener("keydown", onKey)
  return () => document.removeEventListener("keydown", onKey)
}, [])
```

4. Add handler for search result selection:
```typescript
function handleSearchSelect(meetingId: string, query: string) {
  setSelectedMeetingId(meetingId)
  setHighlightQuery(query)
  setActiveView("meetings")
}
```

5. Pass `onOpenSearch` to `MeetingList`:
```typescript
<MeetingList
  themeId={selectedThemeId}
  selectedMeetingId={selectedMeetingId}
  onSelectMeeting={id => { setSelectedMeetingId(id); setHighlightQuery(undefined) }}
  onMeetingDeleted={id => { if (selectedMeetingId === id) setSelectedMeetingId(null) }}
  onOpenSearch={() => setSearchOpen(true)}
/>
```

6. Pass `highlightQuery` to `MeetingDetail`:
```typescript
<MeetingDetail
  meetingId={selectedMeetingId}
  onDeleted={() => setSelectedMeetingId(null)}
  highlightQuery={highlightQuery}
/>
```

7. Render `SearchModal` before closing `</div>`:
```typescript
{searchOpen && (
  <SearchModal
    onClose={() => setSearchOpen(false)}
    onSelect={handleSearchSelect}
  />
)}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Expected: no errors (MeetingDetail's new prop will cause an error until Task 10 is done — that's fine, fix it next)

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/layout/MeetingList.tsx
git commit -m "feat: Ctrl+K listener, search icon, SearchModal wired in App.tsx"
```

---

### Task 10: MeetingDetail — highlight on navigate

**Files:**
- Modify: `frontend/src/components/layout/MeetingDetail.tsx`

- [ ] **Step 1: Update MeetingDetail props and apply highlight**

In `frontend/src/components/layout/MeetingDetail.tsx`:

1. Add import:
```typescript
import { highlightText } from "../../lib/highlight"
```

2. Update the `Props` interface:
```typescript
interface Props {
  meetingId: string | null
  onDeleted?: () => void
  highlightQuery?: string
}
```

3. Destructure `highlightQuery` in the component:
```typescript
export function MeetingDetail({ meetingId, onDeleted, highlightQuery }: Props) {
```

4. Create a helper inside the component:
```typescript
function hl(text: string | undefined | null): string {
  if (!text || !highlightQuery) return text ?? ""
  return highlightText(text, highlightQuery)
}
```

5. Apply `hl()` and render with `dangerouslySetInnerHTML` for the highlighted fields. For each text field that was previously rendered as plain text, wrap it. Example for title:

```typescript
// Before
<h2 className="...">{meeting.title}</h2>

// After
<h2
  className="..."
  dangerouslySetInnerHTML={{ __html: hl(meeting.title) }}
/>
```

Apply the same pattern to:
- Transcript content (the `<pre>` or text block)
- Summary content
- Each key point content
- Each task description

For Markdown-rendered fields (summary uses `ReactMarkdown`): replace `ReactMarkdown` with a plain `<div dangerouslySetInnerHTML={{ __html: hl(meeting.summary?.content) }} />` only when `highlightQuery` is set. When `highlightQuery` is undefined, keep the existing `ReactMarkdown` rendering. Example:

```typescript
{highlightQuery ? (
  <div
    className="prose prose-sm max-w-none"
    dangerouslySetInnerHTML={{ __html: hl(meeting.summary?.content) }}
  />
) : (
  <ReactMarkdown remarkPlugins={[remarkGfm]}>
    {meeting.summary?.content ?? ""}
  </ReactMarkdown>
)}
```

Add `<style>` or Tailwind class for `mark` elements to use the accent color:
```typescript
// In the component's JSX, add a global style block or use @apply in CSS
// Simplest: add to the root div's className a style that targets mark
// Or: add to index.css:
// mark { background-color: hsl(var(--primary) / 0.2); color: inherit; border-radius: 2px; padding: 0 2px; }
```

Add `mark { background-color: hsl(var(--primary) / 0.2); color: inherit; border-radius: 2px; padding: 0 2px; }` to `frontend/src/index.css`.

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 3: Run all Go tests**

```bash
cd F:/dev/meeting-notes
go test ./...
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/layout/MeetingDetail.tsx frontend/src/index.css
git commit -m "feat: MeetingDetail highlights search term on navigate"
```

---

## Final verification

- [ ] Run the full Go test suite: `go test ./...` — all PASS
- [ ] Build both entry points: `go build ./cmd/api/... ./cmd/desktop/...` — no errors
- [ ] TypeScript: `cd frontend && npx tsc --noEmit` — no errors
- [ ] Manual smoke test: open the app, press `Ctrl+K`, type a term that exists in a meeting title or transcript, click a result, verify it navigates to the meeting and highlights the term
