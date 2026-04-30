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

func newTestBoardCardService(t *testing.T) *services.BoardCardService {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cardRepo := repository.NewBoardCardRepository(db)
	columnRepo := repository.NewBoardColumnRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	return services.NewBoardCardService(cardRepo, columnRepo, meetingRepo, summaryRepo, keyPointRepo, taskRepo)
}

func TestBoardCardService_CreateManualCard(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Revisar proposta", "Detalhes")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}
	if card.Source != "manual" {
		t.Errorf("Source = %q, want 'manual'", card.Source)
	}
	if card.MeetingID != nil {
		t.Errorf("MeetingID should be nil")
	}
	if card.Title != "Revisar proposta" {
		t.Errorf("Title = %q, want 'Revisar proposta'", card.Title)
	}
}

func TestBoardCardService_CreateManualCard_EmptyTitle(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	_, err := svc.CreateManualCard(ctx, "col-backlog", "", "desc")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for empty title, got %T: %v", err, err)
	}
}

func TestBoardCardService_CreateManualCard_InvalidColumn(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	_, err := svc.CreateManualCard(ctx, "col-nonexistent", "Title", "")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for invalid column, got %v", err)
	}
}

func TestBoardCardService_Update(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Title", "original desc")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}

	updated, err := svc.Update(ctx, card.ID, "updated desc", []string{"Task 1"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "updated desc" {
		t.Errorf("Description = %q, want 'updated desc'", updated.Description)
	}
}

func TestBoardCardService_Update_NotFound(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	_, err := svc.Update(ctx, "nonexistent-id", "desc", []string{})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBoardCardService_CreateManualCard_UsesFirstColumnWhenEmpty(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "", "Title", "")
	if err != nil {
		t.Fatalf("CreateManualCard with empty column: %v", err)
	}
	if card.ColumnID == "" {
		t.Error("ColumnID should not be empty when no column provided")
	}
}

func TestBoardCardService_LinkCardToMeeting_NotFound(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Manual", "")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}

	err = svc.LinkCardToMeeting(ctx, card.ID, "nonexistent-meeting")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent meeting, got %v", err)
	}
}

func TestBoardCardService_LinkCardToMeeting_CardNotFound(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	err := svc.LinkCardToMeeting(ctx, "nonexistent-card", "any-meeting")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound for nonexistent card, got %v", err)
	}
}

var _ = models.BoardCard{}
