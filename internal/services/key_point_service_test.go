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
	if _, err := svc.Create(ctx, "m-1", "Manual", 0); err != nil {
		t.Fatalf("Create: %v", err)
	}

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
