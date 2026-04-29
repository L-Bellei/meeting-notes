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

func newTaskTestService(t *testing.T, aiClient ai.AIClient) (*services.TaskService, *models.Meeting) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	transcript := "Definimos ações."
	now := time.Now().UTC()
	meeting := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusCompleted, Transcript: &transcript}
	if err := mr.Create(context.Background(), meeting); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return services.NewTaskService(repository.NewTaskRepository(db), aiClient), meeting
}

func TestTaskService_List_Empty(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	got, err := svc.List(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestTaskService_Create(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	got, err := svc.Create(context.Background(), "m-1", "Fazer X", nil, nil, "high")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID == "" {
		t.Error("ID should be set")
	}
	if got.Description != "Fazer X" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", got.Priority)
	}
	if got.Completed {
		t.Error("Completed should default to false")
	}
}

func TestTaskService_Create_DefaultPriority(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	got, _ := svc.Create(context.Background(), "m-1", "Fazer X", nil, nil, "")
	if got.Priority != models.PriorityMedium {
		t.Errorf("Priority = %q, want medium (default)", got.Priority)
	}
}

func TestTaskService_Create_DescriptionRequired(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	_, err := svc.Create(context.Background(), "m-1", "", nil, nil, "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Create_InvalidPriority(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	_, err := svc.Create(context.Background(), "m-1", "X", nil, nil, "urgent")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Update(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "Original", nil, nil, "low")
	updated, err := svc.Update(ctx, created.ID, "Atualizada", nil, nil, "high", true)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "Atualizada" {
		t.Errorf("Description = %q", updated.Description)
	}
	if !updated.Completed {
		t.Error("Completed should be true")
	}
	if updated.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", updated.Priority)
	}
}

func TestTaskService_Update_NotFound(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	_, err := svc.Update(context.Background(), "nope", "X", nil, nil, "low", false)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskService_Update_DescriptionRequired(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "Original", nil, nil, "low")
	_, err := svc.Update(ctx, created.ID, "", nil, nil, "low", false)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Delete(t *testing.T) {
	svc, _ := newTaskTestService(t, nil)
	ctx := context.Background()
	created, _ := svc.Create(ctx, "m-1", "X", nil, nil, "low")
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestTaskService_Generate(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{
		{Description: "Task 1", Assignee: "Ana", Priority: "high"},
		{Description: "Task 2", Assignee: "", Priority: "low"},
	}}
	svc, meeting := newTaskTestService(t, fake)
	got, err := svc.Generate(context.Background(), meeting, "")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(got))
	}
	if got[0].Description != "Task 1" {
		t.Errorf("first task wrong: %+v", got[0])
	}
	if got[0].Assignee == nil || *got[0].Assignee != "Ana" {
		t.Errorf("first assignee wrong: %v", got[0].Assignee)
	}
	if got[1].Assignee != nil {
		t.Errorf("empty assignee should be nil, got %v", got[1].Assignee)
	}
	if got[1].Priority != models.PriorityLow {
		t.Errorf("second priority wrong: %q", got[1].Priority)
	}
}

func TestTaskService_Generate_ReplacesExisting(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{{Description: "Nova", Priority: "medium"}}}
	svc, meeting := newTaskTestService(t, fake)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "m-1", "Manual", nil, nil, "low"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Generate(ctx, meeting, ""); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	all, _ := svc.List(ctx, "m-1")
	if len(all) != 1 {
		t.Errorf("expected 1 in DB after replace, got %d", len(all))
	}
	if all[0].Description != "Nova" {
		t.Errorf("expected only Nova, got %q", all[0].Description)
	}
}

func TestTaskService_Generate_NoTranscript(t *testing.T) {
	fake := &fakeAI{tasks: []ai.TaskSuggestion{{Description: "x"}}}
	svc, meeting := newTaskTestService(t, fake)
	meeting.Transcript = nil
	_, err := svc.Generate(context.Background(), meeting, "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestTaskService_Generate_AINotConfigured(t *testing.T) {
	svc, meeting := newTaskTestService(t, nil)
	_, err := svc.Generate(context.Background(), meeting, "")
	if !errors.Is(err, services.ErrAINotConfigured) {
		t.Errorf("expected ErrAINotConfigured, got %v", err)
	}
}
