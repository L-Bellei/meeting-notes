package repository_test

import (
	"context"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
)

func newSearchRepo(t *testing.T) *repository.SearchRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewSearchRepository(db)
}

func TestSearchRepository_UpsertAndSearch(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	if err := repo.UpsertMeeting(ctx, "id-1", "Sprint Planning", "transcript text", "summary text", "Deploy API", "Write tests"); err != nil {
		t.Fatalf("UpsertMeeting: %v", err)
	}

	results, err := repo.Search(ctx, "Sprint")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MeetingID != "id-1" {
		t.Errorf("MeetingID = %q, want 'id-1'", results[0].MeetingID)
	}
	if results[0].Snippet == "" {
		t.Error("Snippet should not be empty")
	}
}

func TestSearchRepository_UpsertOverwrites(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	if err := repo.UpsertMeeting(ctx, "id-1", "Old Title", "", "", "", ""); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertMeeting(ctx, "id-1", "New Title", "", "", "", ""); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	results, err := repo.Search(ctx, "New")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after upsert, got %d", len(results))
	}

	old, err := repo.Search(ctx, "Old")
	if err != nil {
		t.Fatalf("Search old: %v", err)
	}
	if len(old) != 0 {
		t.Errorf("expected 0 results for old title after upsert, got %d", len(old))
	}
}

func TestSearchRepository_Delete(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	if err := repo.UpsertMeeting(ctx, "id-1", "To Delete", "", "", "", ""); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := repo.DeleteMeeting(ctx, "id-1"); err != nil {
		t.Fatalf("DeleteMeeting: %v", err)
	}

	results, err := repo.Search(ctx, "Delete")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestSearchRepository_EmptyQuery(t *testing.T) {
	repo := newSearchRepo(t)
	ctx := context.Background()

	_, err := repo.Search(ctx, "")
	if err == nil {
		t.Error("expected error for empty query")
	}
}
