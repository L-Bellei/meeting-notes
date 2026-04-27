# Fatia 3 — CRUD Meetings: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implementar CRUD completo de reuniões com repositório, serviço, handlers HTTP e rotas registradas no servidor.

**Architecture:** Três camadas: `repository` (SQL + filtros opcionais), `services` (validação + UUID + defaults), `handlers` (HTTP + DTOs). `GET /api/meetings/{id}` retorna DTO enriquecido com arrays vazios para `summary`, `key_points` e `tasks` — a serem preenchidos na Fatia 4. Testes usam SQLite real em memória temporária — sem mocks. Context propagado em todas as chamadas de DB.

**Tech Stack:** Go std lib (`database/sql`, `net/http/httptest`), chi v5, google/uuid, modernc.org/sqlite

**Success Criteria:**
```
curl -s -X POST http://localhost:8080/api/meetings \
  -H "Content-Type: application/json" \
  -d '{"title":"Reunião de planejamento"}' | jq .
# → {"id":"...","theme_id":null,"title":"Reunião de planejamento","started_at":"...","status":"pending",...}

curl -s "http://localhost:8080/api/meetings?status=pending" | jq .
# → [{"id":"...","title":"Reunião de planejamento",...}]

curl -s http://localhost:8080/api/meetings/<id> | jq .
# → {"id":"...","summary":null,"key_points":[],"tasks":[]}
```

---

## File Map

| Arquivo | Responsabilidade |
|---|---|
| `internal/repository/meeting_repository.go` | SQL CRUD + filtros opcionais em `List` + `scanMeeting` helper |
| `internal/repository/meeting_repository_test.go` | Testes TDD do repositório com DB real |
| `internal/services/meeting_service.go` | Validação, UUID, defaults de status/startedAt |
| `internal/services/meeting_service_test.go` | Testes TDD do serviço com DB real |
| `internal/handlers/meeting_handler.go` | HTTP handlers + DTOs + `meetingDetailResponse` |
| `internal/handlers/meeting_handler_test.go` | Testes TDD dos handlers com httptest |
| `cmd/api/main.go` | Modificar: adicionar wiring e rotas `/api/meetings` |

---

## Task 1: Meeting Repository

**Files:**
- Create: `internal/repository/meeting_repository.go`
- Create: `internal/repository/meeting_repository_test.go`

O repositório lida com campos nullable (`theme_id`, `started_at`, `duration_seconds`, `transcript`) usando `sql.NullString` / `sql.NullInt64` no scan. `started_at` é armazenado como string RFC3339Nano e convertido em `*time.Time`. O helper `scanMeeting` aceita um `scanner` interface satisfeito tanto por `*sql.Row` quanto por `*sql.Rows`. `ErrNotFound` já existe no pacote — reutilizado.

- [ ] **Step 1: Escrever os testes que vão falhar**

Crie `internal/repository/meeting_repository_test.go`:

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

func openMeetingTestDB(t *testing.T) *repository.MeetingRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewMeetingRepository(db)
}

func TestMeetingRepository_CreateAndGetByID(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	m := &models.Meeting{
		ID:        "id-001",
		Title:     "Reunião de Eng",
		StartedAt: &now,
		Status:    models.StatusPending,
	}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "id-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Reunião de Eng" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.ThemeID != nil {
		t.Errorf("ThemeID should be nil, got %v", *got.ThemeID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestMeetingRepository_GetByID_NotFound(t *testing.T) {
	repo := openMeetingTestDB(t)
	_, err := repo.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMeetingRepository_List_Empty(t *testing.T) {
	repo := openMeetingTestDB(t)
	meetings, err := repo.List(context.Background(), "", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 0 {
		t.Errorf("expected 0, got %d", len(meetings))
	}
}

func TestMeetingRepository_List_OrderedByStartedAt(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	t1 := time.Now().UTC().Add(-2 * time.Hour)
	t2 := time.Now().UTC().Add(-1 * time.Hour)
	repo.Create(ctx, &models.Meeting{ID: "a", Title: "Antiga", StartedAt: &t1, Status: models.StatusPending})
	repo.Create(ctx, &models.Meeting{ID: "b", Title: "Recente", StartedAt: &t2, Status: models.StatusCompleted})

	meetings, err := repo.List(ctx, "", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 2 {
		t.Fatalf("expected 2, got %d", len(meetings))
	}
	if meetings[0].Title != "Recente" {
		t.Errorf("expected DESC order, first = %q", meetings[0].Title)
	}
}

func TestMeetingRepository_List_FilterByThemeID(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	themeID := "theme-abc"
	now := time.Now().UTC()
	repo.Create(ctx, &models.Meeting{ID: "a", Title: "Com tema", ThemeID: &themeID, StartedAt: &now, Status: models.StatusPending})
	repo.Create(ctx, &models.Meeting{ID: "b", Title: "Sem tema", StartedAt: &now, Status: models.StatusPending})

	meetings, err := repo.List(ctx, themeID, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 1 {
		t.Fatalf("expected 1, got %d", len(meetings))
	}
	if meetings[0].Title != "Com tema" {
		t.Errorf("Title = %q", meetings[0].Title)
	}
}

func TestMeetingRepository_List_FilterByStatus(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	repo.Create(ctx, &models.Meeting{ID: "a", Title: "Pendente", StartedAt: &now, Status: models.StatusPending})
	repo.Create(ctx, &models.Meeting{ID: "b", Title: "Completa", StartedAt: &now, Status: models.StatusCompleted})

	meetings, err := repo.List(ctx, "", string(models.StatusCompleted))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 1 {
		t.Fatalf("expected 1, got %d", len(meetings))
	}
	if meetings[0].Title != "Completa" {
		t.Errorf("Title = %q", meetings[0].Title)
	}
}

func TestMeetingRepository_List_FilterByBoth(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	themeID := "theme-xyz"
	now := time.Now().UTC()
	repo.Create(ctx, &models.Meeting{ID: "a", Title: "Match", ThemeID: &themeID, StartedAt: &now, Status: models.StatusCompleted})
	repo.Create(ctx, &models.Meeting{ID: "b", Title: "Tema errado", StartedAt: &now, Status: models.StatusCompleted})
	repo.Create(ctx, &models.Meeting{ID: "c", Title: "Status errado", ThemeID: &themeID, StartedAt: &now, Status: models.StatusPending})

	meetings, err := repo.List(ctx, themeID, string(models.StatusCompleted))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 1 {
		t.Fatalf("expected 1, got %d", len(meetings))
	}
	if meetings[0].Title != "Match" {
		t.Errorf("Title = %q", meetings[0].Title)
	}
}

func TestMeetingRepository_Update(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	repo.Create(ctx, &models.Meeting{ID: "id-001", Title: "Original", StartedAt: &now, Status: models.StatusPending})

	got, _ := repo.GetByID(ctx, "id-001")
	got.Title = "Atualizado"
	got.Status = models.StatusCompleted
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := repo.GetByID(ctx, "id-001")
	if updated.Title != "Atualizado" {
		t.Errorf("Title = %q", updated.Title)
	}
	if updated.Status != models.StatusCompleted {
		t.Errorf("Status = %q", updated.Status)
	}
}

func TestMeetingRepository_Update_NotFound(t *testing.T) {
	repo := openMeetingTestDB(t)
	now := time.Now().UTC()
	err := repo.Update(context.Background(), &models.Meeting{ID: "nope", Title: "X", StartedAt: &now, Status: models.StatusPending})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMeetingRepository_Delete(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	repo.Create(ctx, &models.Meeting{ID: "id-001", Title: "Para deletar", StartedAt: &now, Status: models.StatusPending})
	if err := repo.Delete(ctx, "id-001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, "id-001")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMeetingRepository_Delete_NotFound(t *testing.T) {
	repo := openMeetingTestDB(t)
	err := repo.Delete(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
go test ./internal/repository/... -v -run TestMeeting
```

Resultado esperado: erro de compilação — `MeetingRepository` não existe.

- [ ] **Step 3: Implementar `internal/repository/meeting_repository.go`**

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"meeting-notes/internal/models"
)

type MeetingRepository struct {
	db *sql.DB
}

func NewMeetingRepository(db *sql.DB) *MeetingRepository {
	return &MeetingRepository{db: db}
}

func (r *MeetingRepository) List(ctx context.Context, themeID, status string) ([]models.Meeting, error) {
	query := `SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, created_at FROM meetings`
	var args []any
	var conditions []string

	if themeID != "" {
		conditions = append(conditions, "theme_id = ?")
		args = append(args, themeID)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY started_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	defer rows.Close()

	var meetings []models.Meeting
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, fmt.Errorf("scan meeting: %w", err)
		}
		meetings = append(meetings, *m)
	}
	return meetings, rows.Err()
}

func (r *MeetingRepository) Create(ctx context.Context, m *models.Meeting) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	var startedAt *string
	if m.StartedAt != nil {
		s := m.StartedAt.UTC().Format(time.RFC3339Nano)
		startedAt = &s
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO meetings (id, theme_id, title, started_at, duration_seconds, status, transcript, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript,
		m.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create meeting: %w", err)
	}
	return nil
}

func (r *MeetingRepository) GetByID(ctx context.Context, id string) (*models.Meeting, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, created_at FROM meetings WHERE id = ?`, id,
	)
	m, err := scanMeeting(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get meeting: %w", err)
	}
	return m, nil
}

func (r *MeetingRepository) Update(ctx context.Context, m *models.Meeting) error {
	var startedAt *string
	if m.StartedAt != nil {
		s := m.StartedAt.UTC().Format(time.RFC3339Nano)
		startedAt = &s
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE meetings SET theme_id = ?, title = ?, started_at = ?, duration_seconds = ?, status = ?, transcript = ? WHERE id = ?`,
		m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript, m.ID,
	)
	if err != nil {
		return fmt.Errorf("update meeting: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update meeting rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MeetingRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM meetings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete meeting: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete meeting rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type meetingScanner interface {
	Scan(dest ...any) error
}

func scanMeeting(row meetingScanner) (*models.Meeting, error) {
	var m models.Meeting
	var themeID sql.NullString
	var startedAt sql.NullString
	var duration sql.NullInt64
	var transcript sql.NullString
	var createdAt string
	var status string

	err := row.Scan(&m.ID, &themeID, &m.Title, &startedAt, &duration, &status, &transcript, &createdAt)
	if err != nil {
		return nil, err
	}

	if themeID.Valid {
		v := themeID.String
		m.ThemeID = &v
	}
	if startedAt.Valid {
		t, err := parseMeetingTime(startedAt.String)
		if err != nil {
			return nil, err
		}
		m.StartedAt = &t
	}
	if duration.Valid {
		d := int(duration.Int64)
		m.DurationSeconds = &d
	}
	if transcript.Valid {
		v := transcript.String
		m.Transcript = &v
	}
	m.Status = models.MeetingStatus(status)
	if m.CreatedAt, err = parseMeetingTime(createdAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func parseMeetingTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}
```

- [ ] **Step 4: Rodar os testes**

```bash
go test ./internal/repository/... -v -run TestMeeting
```

Resultado esperado: todos os 10 testes `TestMeeting*` PASS.

- [ ] **Step 5: Rodar todos os testes do pacote para garantir regressão zero**

```bash
go test ./internal/repository/... -v
```

Resultado esperado: todos os testes (theme + meeting) PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/meeting_repository.go internal/repository/meeting_repository_test.go
git commit -m "feat: add meeting repository with CRUD and optional filters"
```

---

## Task 2: Meeting Service

**Files:**
- Create: `internal/services/meeting_service.go`
- Create: `internal/services/meeting_service_test.go`

O serviço valida `title` (obrigatório), valida `status` contra os 6 valores válidos definidos em `models`, aplica default `"pending"` quando status vazio, aplica default `time.Now().UTC()` quando `startedAt` é nil, e converte `themeID string` para `*string` (nil se vazio). `ValidationError` já existe no pacote `services` — reutilizado.

- [ ] **Step 1: Escrever os testes que vão falhar**

Crie `internal/services/meeting_service_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestMeetingService(t *testing.T) *services.MeetingService {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return services.NewMeetingService(repository.NewMeetingRepository(db))
}

func TestMeetingService_Create(t *testing.T) {
	svc := newTestMeetingService(t)
	m, err := svc.Create(context.Background(), "Reunião de eng", "", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.ID == "" {
		t.Error("ID should be set")
	}
	if m.Status != models.StatusPending {
		t.Errorf("Status = %q, want pending", m.Status)
	}
	if m.StartedAt == nil {
		t.Error("StartedAt should be set")
	}
	if m.ThemeID != nil {
		t.Errorf("ThemeID should be nil, got %v", *m.ThemeID)
	}
	if m.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestMeetingService_Create_TitleRequired(t *testing.T) {
	svc := newTestMeetingService(t)
	_, err := svc.Create(context.Background(), "", "", "", nil)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestMeetingService_Create_InvalidStatus(t *testing.T) {
	svc := newTestMeetingService(t)
	_, err := svc.Create(context.Background(), "Título", "", "invalido", nil)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestMeetingService_Create_AllValidStatuses(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	for i, status := range []string{"pending", "recording", "transcribing", "processing", "completed", "failed"} {
		title := "Título " + status
		_, err := svc.Create(ctx, title+string(rune('0'+i)), "", status, nil)
		if err != nil {
			t.Errorf("status %q should be valid, got: %v", status, err)
		}
	}
}

func TestMeetingService_Create_WithTheme(t *testing.T) {
	svc := newTestMeetingService(t)
	m, err := svc.Create(context.Background(), "Título", "theme-abc", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.ThemeID == nil || *m.ThemeID != "theme-abc" {
		t.Errorf("ThemeID = %v, want theme-abc", m.ThemeID)
	}
}

func TestMeetingService_GetByID(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "Eng", "", "", nil)
	got, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != created.ID {
		t.Error("ID mismatch")
	}
}

func TestMeetingService_GetByID_NotFound(t *testing.T) {
	svc := newTestMeetingService(t)
	_, err := svc.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMeetingService_List(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	svc.Create(ctx, "A", "", "", nil)
	svc.Create(ctx, "B", "", "", nil)
	meetings, err := svc.List(ctx, "", "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 2 {
		t.Fatalf("expected 2, got %d", len(meetings))
	}
}

func TestMeetingService_List_FilterByStatus(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	svc.Create(ctx, "Pendente", "", "pending", nil)
	svc.Create(ctx, "Completa", "", "completed", nil)
	meetings, err := svc.List(ctx, "", "completed")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(meetings) != 1 {
		t.Fatalf("expected 1, got %d", len(meetings))
	}
	if meetings[0].Title != "Completa" {
		t.Errorf("Title = %q", meetings[0].Title)
	}
}

func TestMeetingService_Update(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "Original", "", "", nil)
	updated, err := svc.Update(ctx, created.ID, "Atualizado", nil, "completed", nil, nil, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Atualizado" {
		t.Errorf("Title = %q", updated.Title)
	}
	if updated.Status != models.StatusCompleted {
		t.Errorf("Status = %q", updated.Status)
	}
}

func TestMeetingService_Update_TitleRequired(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "Original", "", "", nil)
	_, err := svc.Update(ctx, created.ID, "", nil, "", nil, nil, nil)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestMeetingService_Update_InvalidStatus(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "Original", "", "", nil)
	_, err := svc.Update(ctx, created.ID, "Título", nil, "invalido", nil, nil, nil)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestMeetingService_Update_NotFound(t *testing.T) {
	svc := newTestMeetingService(t)
	_, err := svc.Update(context.Background(), "nope", "Título", nil, "", nil, nil, nil)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMeetingService_Delete(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "Para deletar", "", "", nil)
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := svc.GetByID(ctx, created.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
go test ./internal/services/... -v -run TestMeeting
```

Resultado esperado: erro de compilação — `MeetingService` não existe.

- [ ] **Step 3: Implementar `internal/services/meeting_service.go`**

```go
package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

var validMeetingStatuses = map[string]bool{
	string(models.StatusPending):      true,
	string(models.StatusRecording):    true,
	string(models.StatusTranscribing): true,
	string(models.StatusProcessing):   true,
	string(models.StatusCompleted):    true,
	string(models.StatusFailed):       true,
}

type MeetingService struct {
	repo *repository.MeetingRepository
}

func NewMeetingService(repo *repository.MeetingRepository) *MeetingService {
	return &MeetingService{repo: repo}
}

func (s *MeetingService) List(ctx context.Context, themeID, status string) ([]models.Meeting, error) {
	return s.repo.List(ctx, themeID, status)
}

func (s *MeetingService) Create(ctx context.Context, title, themeID, status string, startedAt *time.Time) (*models.Meeting, error) {
	if title == "" {
		return nil, &ValidationError{"title is required"}
	}
	if status == "" {
		status = string(models.StatusPending)
	} else if !validMeetingStatuses[status] {
		return nil, &ValidationError{"invalid status: must be one of pending, recording, transcribing, processing, completed, failed"}
	}
	if startedAt == nil {
		now := time.Now().UTC()
		startedAt = &now
	}
	var themeIDPtr *string
	if themeID != "" {
		themeIDPtr = &themeID
	}
	m := &models.Meeting{
		ID:        uuid.New().String(),
		ThemeID:   themeIDPtr,
		Title:     title,
		StartedAt: startedAt,
		Status:    models.MeetingStatus(status),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MeetingService) GetByID(ctx context.Context, id string) (*models.Meeting, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MeetingService) Update(ctx context.Context, id, title string, themeID *string, status string, startedAt *time.Time, durationSeconds *int, transcript *string) (*models.Meeting, error) {
	if title == "" {
		return nil, &ValidationError{"title is required"}
	}
	if status != "" && !validMeetingStatuses[status] {
		return nil, &ValidationError{"invalid status: must be one of pending, recording, transcribing, processing, completed, failed"}
	}
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Title = title
	if themeID != nil {
		if *themeID == "" {
			m.ThemeID = nil
		} else {
			m.ThemeID = themeID
		}
	}
	if status != "" {
		m.Status = models.MeetingStatus(status)
	}
	if startedAt != nil {
		m.StartedAt = startedAt
	}
	if durationSeconds != nil {
		m.DurationSeconds = durationSeconds
	}
	if transcript != nil {
		m.Transcript = transcript
	}
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MeetingService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Rodar os testes**

```bash
go test ./internal/services/... -v -run TestMeeting
```

Resultado esperado: todos os 13 testes `TestMeeting*` PASS.

- [ ] **Step 5: Rodar todos os testes do pacote**

```bash
go test ./internal/services/... -v
```

Resultado esperado: todos os testes (theme + meeting) PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services/meeting_service.go internal/services/meeting_service_test.go
git commit -m "feat: add meeting service with validation and UUID generation"
```

---

## Task 3: Meeting Handler

**Files:**
- Create: `internal/handlers/meeting_handler.go`
- Create: `internal/handlers/meeting_handler_test.go`

O handler lê query params `theme_id` e `status` em `List`. O `started_at` é recebido como `*string` (ISO 8601 RFC3339) e convertido para `*time.Time` no handler antes de chamar o service — 400 se o parse falhar. `GET /{id}` retorna `meetingDetailResponse` com `Summary: nil`, `KeyPoints: []models.KeyPoint{}`, `Tasks: []models.Task{}`. O `withChiID` helper já existe em `theme_handler_test.go` (mesmo pacote `handlers_test`) — não redefina.

- [ ] **Step 1: Escrever os testes que vão falhar**

Crie `internal/handlers/meeting_handler_test.go`:

```go
package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestMeetingHandler(t *testing.T) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(services.NewMeetingService(repository.NewMeetingRepository(db)))
}

type meetingDetailResp struct {
	models.Meeting
	Summary   *models.Summary   `json:"summary"`
	KeyPoints []models.KeyPoint `json:"key_points"`
	Tasks     []models.Task     `json:"tasks"`
}

func TestMeetingHandler_List_Empty(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/meetings", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var result []models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d", len(result))
	}
}

func TestMeetingHandler_Create(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"title":"Reunião de planejamento","status":"pending"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var m models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.ID == "" {
		t.Error("ID should be set")
	}
	if m.Title != "Reunião de planejamento" {
		t.Errorf("Title = %q", m.Title)
	}
}

func TestMeetingHandler_Create_TitleRequired(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"status":"pending"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_Create_InvalidStatus(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"title":"Título","status":"invalido"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_GetByID(t *testing.T) {
	h := newTestMeetingHandler(t)

	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Eng"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+created.ID, nil), created.ID)
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var detail meetingDetailResp
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.ID != created.ID {
		t.Errorf("ID mismatch")
	}
	if detail.KeyPoints == nil {
		t.Error("key_points should be empty array, not null")
	}
	if detail.Tasks == nil {
		t.Error("tasks should be empty array, not null")
	}
}

func TestMeetingHandler_GetByID_NotFound(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_Update(t *testing.T) {
	h := newTestMeetingHandler(t)

	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Original"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	updateBody := `{"title":"Atualizado","status":"completed"}`
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/meetings/"+created.ID, bytes.NewBufferString(updateBody)),
		created.ID,
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Title != "Atualizado" {
		t.Errorf("Title = %q", updated.Title)
	}
	if updated.Status != models.StatusCompleted {
		t.Errorf("Status = %q", updated.Status)
	}
}

func TestMeetingHandler_Update_NotFound(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/meetings/nope", bytes.NewBufferString(`{"title":"X"}`)),
		"nope",
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_Delete(t *testing.T) {
	h := newTestMeetingHandler(t)

	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Para deletar"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+created.ID, nil), created.ID)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestMeetingHandler_Delete_NotFound(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_List_FilterByStatus(t *testing.T) {
	h := newTestMeetingHandler(t)

	for _, body := range []string{
		`{"title":"Pendente","status":"pending"}`,
		`{"title":"Completa","status":"completed"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		h.Create(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/meetings?status=completed", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var result []models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Title != "Completa" {
		t.Errorf("Title = %q", result[0].Title)
	}
}
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
go test ./internal/handlers/... -v -run TestMeeting
```

Resultado esperado: erro de compilação — `MeetingHandler` não existe.

- [ ] **Step 3: Implementar `internal/handlers/meeting_handler.go`**

```go
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type MeetingHandler struct {
	svc *services.MeetingService
}

func NewMeetingHandler(svc *services.MeetingService) *MeetingHandler {
	return &MeetingHandler{svc: svc}
}

type createMeetingRequest struct {
	Title     string  `json:"title"`
	ThemeID   string  `json:"theme_id"`
	StartedAt *string `json:"started_at"`
	Status    string  `json:"status"`
}

type updateMeetingRequest struct {
	Title           string  `json:"title"`
	ThemeID         *string `json:"theme_id"`
	StartedAt       *string `json:"started_at"`
	Status          string  `json:"status"`
	DurationSeconds *int    `json:"duration_seconds"`
	Transcript      *string `json:"transcript"`
}

type meetingDetailResponse struct {
	models.Meeting
	Summary   *models.Summary   `json:"summary"`
	KeyPoints []models.KeyPoint `json:"key_points"`
	Tasks     []models.Task     `json:"tasks"`
}

func (h *MeetingHandler) List(w http.ResponseWriter, r *http.Request) {
	themeID := r.URL.Query().Get("theme_id")
	status := r.URL.Query().Get("status")

	meetings, err := h.svc.List(r.Context(), themeID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list meetings")
		return
	}
	if meetings == nil {
		meetings = []models.Meeting{}
	}
	writeJSON(w, http.StatusOK, meetings)
}

func (h *MeetingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var startedAt *time.Time
	if req.StartedAt != nil {
		t, err := time.Parse(time.RFC3339, *req.StartedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid started_at: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
			return
		}
		startedAt = &t
	}
	m, err := h.svc.Create(r.Context(), req.Title, req.ThemeID, req.Status, startedAt)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create meeting")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

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
	writeJSON(w, http.StatusOK, meetingDetailResponse{
		Meeting:   *m,
		Summary:   nil,
		KeyPoints: []models.KeyPoint{},
		Tasks:     []models.Task{},
	})
}

func (h *MeetingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var startedAt *time.Time
	if req.StartedAt != nil {
		t, err := time.Parse(time.RFC3339, *req.StartedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid started_at: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
			return
		}
		startedAt = &t
	}
	m, err := h.svc.Update(r.Context(), id, req.Title, req.ThemeID, req.Status, startedAt, req.DurationSeconds, req.Transcript)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update meeting")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *MeetingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete meeting")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Rodar os testes**

```bash
go test ./internal/handlers/... -v -run TestMeeting
```

Resultado esperado: todos os 11 testes `TestMeeting*` PASS.

- [ ] **Step 5: Rodar todos os testes do projeto**

```bash
go test ./... -v
```

Resultado esperado: todos os testes dos 4 pacotes passam.

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/meeting_handler.go internal/handlers/meeting_handler_test.go
git commit -m "feat: add meeting handler with detail response and query filters"
```

---

## Task 4: Registrar rotas em main.go

**Files:**
- Modify: `cmd/api/main.go`

Os imports de `handlers`, `repository` e `services` já existem — não adicionar duplicatas. Adicionar apenas o wiring do `MeetingHandler` e o bloco `r.Route("/api/meetings", ...)`.

- [ ] **Step 1: Modificar `cmd/api/main.go`**

Adicionar após o bloco `r.Route("/api/themes", ...)` existente:

```go
meetingHandler := handlers.NewMeetingHandler(
    services.NewMeetingService(
        repository.NewMeetingRepository(db),
    ),
)
r.Route("/api/meetings", func(r chi.Router) {
    r.Get("/", meetingHandler.List)
    r.Post("/", meetingHandler.Create)
    r.Get("/{id}", meetingHandler.GetByID)
    r.Put("/{id}", meetingHandler.Update)
    r.Delete("/{id}", meetingHandler.Delete)
})
```

O `main()` completo após a modificação:

```go
func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", healthHandler(db))

	themeHandler := handlers.NewThemeHandler(
		services.NewThemeService(
			repository.NewThemeRepository(db),
		),
	)
	r.Route("/api/themes", func(r chi.Router) {
		r.Get("/", themeHandler.List)
		r.Post("/", themeHandler.Create)
		r.Get("/{id}", themeHandler.GetByID)
		r.Put("/{id}", themeHandler.Update)
		r.Delete("/{id}", themeHandler.Delete)
	})

	meetingHandler := handlers.NewMeetingHandler(
		services.NewMeetingService(
			repository.NewMeetingRepository(db),
		),
	)
	r.Route("/api/meetings", func(r chi.Router) {
		r.Get("/", meetingHandler.List)
		r.Post("/", meetingHandler.Create)
		r.Get("/{id}", meetingHandler.GetByID)
		r.Put("/{id}", meetingHandler.Update)
		r.Delete("/{id}", meetingHandler.Delete)
	})

	log.Printf("server listening on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
```

- [ ] **Step 2: Compilar**

```bash
go build ./...
```

Resultado esperado: sem output.

- [ ] **Step 3: Rodar o servidor e testar via curl**

Em um terminal:
```bash
go run ./cmd/api
```

Em outro terminal:
```bash
# Criar reunião
curl -s -X POST http://localhost:8080/api/meetings \
  -H "Content-Type: application/json" \
  -d '{"title":"Reunião de planejamento"}' | jq .
```
Resultado esperado:
```json
{
  "id": "...",
  "theme_id": null,
  "title": "Reunião de planejamento",
  "started_at": "...",
  "duration_seconds": null,
  "status": "pending",
  "transcript": null,
  "created_at": "..."
}
```

```bash
# Listar reuniões
curl -s http://localhost:8080/api/meetings | jq .

# Buscar por ID (substitua <id>)
curl -s http://localhost:8080/api/meetings/<id> | jq .
# Resultado esperado: meeting + "summary":null, "key_points":[], "tasks":[]

# Filtrar por status
curl -s "http://localhost:8080/api/meetings?status=pending" | jq .

# Atualizar
curl -s -X PUT http://localhost:8080/api/meetings/<id> \
  -H "Content-Type: application/json" \
  -d '{"title":"Reunião atualizada","status":"completed"}' | jq .

# Deletar
curl -s -X DELETE http://localhost:8080/api/meetings/<id>
# Resultado esperado: sem body (204)
```

- [ ] **Step 4: Parar o servidor (Ctrl+C) e rodar todos os testes**

```bash
go test ./... -v
```

Resultado esperado: todos os testes de todos os pacotes passam.

- [ ] **Step 5: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: register meeting routes in HTTP server"
```

---

## Self-Review

**Cobertura do spec:**
- [x] `GET /api/meetings` com filtros `theme_id` e `status` — Task 1 (repository List), Task 2 (service List), Task 3 (handler List)
- [x] `POST /api/meetings` com title obrigatório, status validado, defaults — Tasks 1-3
- [x] `GET /api/meetings/{id}` com `summary:null`, `key_points:[]`, `tasks:[]` — Task 3 (`meetingDetailResponse`)
- [x] `PUT /api/meetings/{id}` com preservação de campos nil — Tasks 1-3
- [x] `DELETE /api/meetings/{id}` 204 — Tasks 1-3
- [x] Rotas registradas em main.go — Task 4
- [x] Campos nullable (`theme_id`, `started_at`, `duration_seconds`, `transcript`) — Task 1 (`scanMeeting` com `sql.NullString`/`sql.NullInt64`)
- [x] Ordem `started_at DESC` em List — Task 1

**HTTP status codes:**
- [x] 200 — GET list, GET/{id}, PUT/{id}
- [x] 201 — POST
- [x] 204 — DELETE
- [x] 400 — JSON inválido, `started_at` mal formatado
- [x] 404 — ID inexistente
- [x] 422 — title vazio, status inválido
- [x] 500 — erros de DB

**Consistência de tipos entre tasks:**
- `repository.MeetingRepository` → `services.MeetingService` → `handlers.MeetingHandler` ✓
- `models.MeetingStatus` e constantes `StatusPending` etc. usados consistentemente ✓
- `*string` / `*time.Time` / `*int` para campos nullable consistentes em todas as camadas ✓
- `services.ValidationError` (já existente) reutilizado no handler com `errors.As` ✓
- `repository.ErrNotFound` (já existente) reutilizado no handler com `errors.Is` ✓
- `withChiID` do `theme_handler_test.go` reutilizado — não redefinido ✓
