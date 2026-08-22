package services_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestBoardCardServiceWithDB(t *testing.T) (*services.BoardCardService, *sql.DB) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return services.NewBoardCardService(
		repository.NewBoardCardRepository(db),
		repository.NewBoardColumnRepository(db),
		repository.NewMeetingRepository(db),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
	), db
}

func newTestBoardCardService(t *testing.T) *services.BoardCardService {
	t.Helper()
	svc, _ := newTestBoardCardServiceWithDB(t)
	return svc
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

func TestBoardCardService_Create_DoesNotCopySummaryIntoDescription(t *testing.T) {
	svc, db := newTestBoardCardServiceWithDB(t)
	ctx := context.Background()

	if _, err := db.Exec(
		`INSERT INTO meetings (id, title, status) VALUES ('m-1', 'Reunião', 'processed')`); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO summaries (id, meeting_id, content, model_used)
		 VALUES ('s-1', 'm-1', 'conteúdo do resumo', 'test')`); err != nil {
		t.Fatalf("seed summary: %v", err)
	}

	card, err := svc.Create(ctx, "m-1", "col-backlog")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if card.Description != "" {
		t.Errorf("Description = %q, want vazia — o resumo não deve ser copiado", card.Description)
	}
}

func TestBoardCardService_GetDetail_ReportsHasTranscript(t *testing.T) {
	svc, db := newTestBoardCardServiceWithDB(t)
	ctx := context.Background()

	if _, err := db.Exec(
		`INSERT INTO meetings (id, title, status, transcript)
		 VALUES ('m-com', 'Com transcrição', 'processed', 'texto')`); err != nil {
		t.Fatalf("seed m-com: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO meetings (id, title, status) VALUES ('m-sem', 'Sem transcrição', 'pending')`); err != nil {
		t.Fatalf("seed m-sem: %v", err)
	}

	comCard, err := svc.Create(ctx, "m-com", "col-backlog")
	if err != nil {
		t.Fatalf("Create m-com: %v", err)
	}
	semCard, err := svc.Create(ctx, "m-sem", "col-backlog")
	if err != nil {
		t.Fatalf("Create m-sem: %v", err)
	}

	com, err := svc.GetDetail(ctx, comCard.ID)
	if err != nil {
		t.Fatalf("GetDetail m-com: %v", err)
	}
	if !com.HasTranscript {
		t.Error("HasTranscript = false para reunião com transcrição")
	}

	sem, err := svc.GetDetail(ctx, semCard.ID)
	if err != nil {
		t.Fatalf("GetDetail m-sem: %v", err)
	}
	if sem.HasTranscript {
		t.Error("HasTranscript = true para reunião sem transcrição")
	}
}

func TestBoardCardService_GetDetail_ManualCardHasNoTranscript(t *testing.T) {
	svc := newTestBoardCardService(t)
	ctx := context.Background()

	card, err := svc.CreateManualCard(ctx, "col-backlog", "Card manual", "")
	if err != nil {
		t.Fatalf("CreateManualCard: %v", err)
	}
	detail, err := svc.GetDetail(ctx, card.ID)
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if detail.HasTranscript {
		t.Error("card manual não tem reunião, então HasTranscript deve ser false")
	}
}

var _ = models.BoardCard{}
