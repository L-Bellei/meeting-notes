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

func (f *fakeAI) GenerateSummary(ctx context.Context, transcript, notes string) (string, int, int, error) {
	if f.err != nil {
		return "", 0, 0, f.err
	}
	return f.summaryText, 100, 50, nil
}
func (f *fakeAI) GenerateKeyPoints(ctx context.Context, transcript, notes string) ([]string, int, int, error) {
	if f.err != nil {
		return nil, 0, 0, f.err
	}
	return f.keyPoints, 100, 50, nil
}
func (f *fakeAI) GenerateTasks(ctx context.Context, transcript, notes string) ([]ai.TaskSuggestion, int, int, error) {
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
	if _, err := svc.Upsert(ctx, "m-1", "Original", "manual"); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if _, err := svc.Upsert(ctx, "m-1", "Substituído", "manual"); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, _ := svc.Get(ctx, "m-1")
	if got.Content != "Substituído" {
		t.Errorf("Content = %q", got.Content)
	}
}

func TestSummaryService_Delete(t *testing.T) {
	svc, _ := newSummaryTestService(t, nil)
	ctx := context.Background()
	if _, err := svc.Upsert(ctx, "m-1", "X", "manual"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
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
