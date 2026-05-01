package repository_test

import (
	"context"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
)

func openLogTestDB(t *testing.T) *repository.LogRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewLogRepository(db)
}

func TestLogRepository_Insert(t *testing.T) {
	repo := openLogTestDB(t)
	ctx := context.Background()

	if err := repo.Insert(ctx, "info", "test", "hello world", nil); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	logs, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Level != "info" {
		t.Errorf("level = %q, want info", logs[0].Level)
	}
	if logs[0].Component != "test" {
		t.Errorf("component = %q, want test", logs[0].Component)
	}
	if logs[0].Message != "hello world" {
		t.Errorf("message = %q, want hello world", logs[0].Message)
	}
	if logs[0].Metadata != nil {
		t.Errorf("metadata = %v, want nil", logs[0].Metadata)
	}
}

func TestLogRepository_List_DescendingOrder(t *testing.T) {
	repo := openLogTestDB(t)
	ctx := context.Background()

	messages := []string{"first", "second", "third"}
	for _, msg := range messages {
		if err := repo.Insert(ctx, "info", "test", msg, nil); err != nil {
			t.Fatalf("Insert %q: %v", msg, err)
		}
	}

	logs, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	for i, l := range logs {
		if l.ID == "" {
			t.Errorf("log[%d].ID is empty", i)
		}
		if l.CreatedAt == "" {
			t.Errorf("log[%d].CreatedAt is empty", i)
		}
	}
}

func TestLogRepository_List_Limit(t *testing.T) {
	repo := openLogTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := repo.Insert(ctx, "warn", "test", "msg", nil); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	logs, err := repo.List(ctx, 3)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs with limit=3, got %d", len(logs))
	}
}

func TestLogRepository_Insert_WithMetadata(t *testing.T) {
	repo := openLogTestDB(t)
	ctx := context.Background()

	meta := `{"key":"value"}`
	if err := repo.Insert(ctx, "error", "orchestrator", "something failed", &meta); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	logs, err := repo.List(ctx, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Metadata == nil || *logs[0].Metadata != meta {
		t.Errorf("metadata = %v, want %q", logs[0].Metadata, meta)
	}
}
