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
	// Pre-seed theme IDs used by tests to satisfy FK constraints.
	if _, err := db.Exec(`INSERT INTO themes (id, name, color) VALUES (?, ?, '#ffffff')`, "theme-abc", "theme-abc"); err != nil {
		t.Fatalf("seed theme: %v", err)
	}
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
		title := "Título " + status + string(rune('0'+i))
		_, err := svc.Create(ctx, title, "", status, nil)
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

func TestMeetingService_Update_PreservesStatusWhenEmpty(t *testing.T) {
	svc := newTestMeetingService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Original", "", "completed", nil)
	updated, err := svc.Update(ctx, created.ID, "Novo título", nil, "", nil, nil, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Status != models.StatusCompleted {
		t.Errorf("Status should be preserved as completed, got %q", updated.Status)
	}
}
