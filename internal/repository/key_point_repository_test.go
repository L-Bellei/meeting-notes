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
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-2", MeetingID: "m-1", Position: 1, Content: "Segundo"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "Primeiro"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

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
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "X"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

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
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "Antigo"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
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
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Content: "X"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
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
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Content: "A"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, &models.KeyPoint{ID: "kp-2", MeetingID: "m-1", Content: "B"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.DeleteByMeetingID(ctx, "m-1"); err != nil {
		t.Fatalf("DeleteByMeetingID: %v", err)
	}
	kps, _ := repo.ListByMeetingID(ctx, "m-1")
	if len(kps) != 0 {
		t.Errorf("expected 0 after DeleteByMeetingID, got %d", len(kps))
	}
}
