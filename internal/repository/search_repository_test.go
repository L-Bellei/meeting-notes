package repository_test

import (
	"context"
	"testing"
	"time"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func newSearchRepo(t *testing.T) (*repository.SearchRepository, *repository.MeetingRepository, *repository.SummaryRepository, *repository.KeyPointRepository, *repository.TaskRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewSearchRepository(db),
		repository.NewMeetingRepository(db),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db)
}

func TestSearchRepository_SearchesCanonicalMeetingContent(t *testing.T) {
	repo, meetings, summaries, keyPoints, tasks := newSearchRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	meeting := &models.Meeting{
		ID: "id-1", Title: "Sprint Planning", StartedAt: &now,
		Status: models.StatusCompleted, Transcript: stringPointer("transcript text"),
	}
	if err := meetings.Create(ctx, meeting); err != nil {
		t.Fatal(err)
	}
	if err := summaries.Upsert(ctx, &models.Summary{ID: "s-1", MeetingID: meeting.ID, Content: "summary text", ModelUsed: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := keyPoints.Create(ctx, &models.KeyPoint{ID: "kp-1", MeetingID: meeting.ID, Content: "Deploy API"}); err != nil {
		t.Fatal(err)
	}
	if err := tasks.Create(ctx, &models.Task{ID: "t-1", MeetingID: meeting.ID, Description: "Write tests", Priority: models.PriorityMedium}); err != nil {
		t.Fatal(err)
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

func TestSearchRepository_TracksMeetingUpdates(t *testing.T) {
	repo, meetings, _, _, _ := newSearchRepo(t)
	ctx := context.Background()
	meeting := &models.Meeting{ID: "id-1", Title: "Old Title", Status: models.StatusCompleted}
	if err := meetings.Create(ctx, meeting); err != nil {
		t.Fatal(err)
	}
	meeting.Title = "New Title"
	if err := meetings.Update(ctx, meeting); err != nil {
		t.Fatal(err)
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
	repo, meetings, _, _, _ := newSearchRepo(t)
	ctx := context.Background()
	if err := meetings.Create(ctx, &models.Meeting{ID: "id-1", Title: "To Delete", Status: models.StatusCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := meetings.Delete(ctx, "id-1"); err != nil {
		t.Fatalf("Delete meeting: %v", err)
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
	repo, _, _, _, _ := newSearchRepo(t)
	ctx := context.Background()

	_, err := repo.Search(ctx, "")
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func stringPointer(value string) *string {
	return &value
}
