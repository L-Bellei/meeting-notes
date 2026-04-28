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
