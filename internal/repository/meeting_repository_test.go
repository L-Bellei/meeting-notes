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
	// Pre-seed theme IDs used by filter tests to satisfy FK constraints.
	for _, id := range []string{"theme-abc", "theme-xyz"} {
		db.Exec(`INSERT INTO themes (id, name, color) VALUES (?, ?, '#ffffff')`, id, id)
	}
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
	meetings, err := repo.List(context.Background(), repository.ListFilters{})
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

	meetings, err := repo.List(ctx, repository.ListFilters{})
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

	meetings, err := repo.List(ctx, repository.ListFilters{ThemeIDs: []string{themeID}})
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

	meetings, err := repo.List(ctx, repository.ListFilters{Status: string(models.StatusCompleted)})
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

	meetings, err := repo.List(ctx, repository.ListFilters{ThemeIDs: []string{themeID}, Status: string(models.StatusCompleted)})
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
