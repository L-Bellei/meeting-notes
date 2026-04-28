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

func openTaskTestDB(t *testing.T) *repository.TaskRepository {
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
	return repository.NewTaskRepository(db)
}

func TestTaskRepository_ListByMeetingID_Empty(t *testing.T) {
	repo := openTaskTestDB(t)
	tasks, err := repo.ListByMeetingID(context.Background(), "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0, got %d", len(tasks))
	}
	if tasks == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestTaskRepository_CreateAndGet(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	due := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	assignee := "Ana"
	task := &models.Task{
		ID: "t-1", MeetingID: "m-1", Description: "Fazer X",
		Assignee: &assignee, DueDate: &due, Priority: models.PriorityHigh, Completed: false,
	}
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "t-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Description != "Fazer X" {
		t.Errorf("Description = %q", got.Description)
	}
	if got.Assignee == nil || *got.Assignee != "Ana" {
		t.Errorf("Assignee = %v", got.Assignee)
	}
	if got.DueDate == nil || !got.DueDate.Equal(due) {
		t.Errorf("DueDate = %v, want %v", got.DueDate, due)
	}
	if got.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", got.Priority)
	}
	if got.Completed {
		t.Error("Completed should be false")
	}
}

func TestTaskRepository_Create_NullableFields(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	if err := repo.Create(ctx, &models.Task{
		ID: "t-1", MeetingID: "m-1", Description: "Sem assignee", Priority: models.PriorityMedium,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, _ := repo.GetByID(ctx, "t-1")
	if got.Assignee != nil {
		t.Errorf("Assignee should be nil, got %v", *got.Assignee)
	}
	if got.DueDate != nil {
		t.Errorf("DueDate should be nil, got %v", *got.DueDate)
	}
}

func TestTaskRepository_GetByID_NotFound(t *testing.T) {
	repo := openTaskTestDB(t)
	_, err := repo.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskRepository_ListByMeetingID_OrderedByCreatedAt(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	t1 := time.Now().UTC().Add(-2 * time.Hour)
	t2 := time.Now().UTC().Add(-1 * time.Hour)
	if err := repo.Create(ctx, &models.Task{ID: "a", MeetingID: "m-1", Description: "Antiga", Priority: "medium", CreatedAt: t1}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, &models.Task{ID: "b", MeetingID: "m-1", Description: "Recente", Priority: "medium", CreatedAt: t2}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	tasks, err := repo.ListByMeetingID(ctx, "m-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2, got %d", len(tasks))
	}
	if tasks[0].Description != "Antiga" {
		t.Errorf("expected ASC order, first = %q", tasks[0].Description)
	}
}

func TestTaskRepository_Update(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	if err := repo.Create(ctx, &models.Task{ID: "t-1", MeetingID: "m-1", Description: "Original", Priority: "medium"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _ := repo.GetByID(ctx, "t-1")
	got.Description = "Atualizada"
	got.Completed = true
	got.Priority = models.PriorityHigh
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := repo.GetByID(ctx, "t-1")
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

func TestTaskRepository_Update_NotFound(t *testing.T) {
	repo := openTaskTestDB(t)
	err := repo.Update(context.Background(), &models.Task{ID: "nope", MeetingID: "m-1", Priority: "medium"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskRepository_Delete(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	if err := repo.Create(ctx, &models.Task{ID: "t-1", MeetingID: "m-1", Description: "X", Priority: "medium"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(ctx, "t-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, "t-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTaskRepository_Delete_NotFound(t *testing.T) {
	repo := openTaskTestDB(t)
	err := repo.Delete(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskRepository_DeleteByMeetingID(t *testing.T) {
	repo := openTaskTestDB(t)
	ctx := context.Background()
	if err := repo.Create(ctx, &models.Task{ID: "a", MeetingID: "m-1", Description: "A", Priority: "medium"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Create(ctx, &models.Task{ID: "b", MeetingID: "m-1", Description: "B", Priority: "medium"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.DeleteByMeetingID(ctx, "m-1"); err != nil {
		t.Fatalf("DeleteByMeetingID: %v", err)
	}
	tasks, _ := repo.ListByMeetingID(ctx, "m-1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 after DeleteByMeetingID, got %d", len(tasks))
	}
}
