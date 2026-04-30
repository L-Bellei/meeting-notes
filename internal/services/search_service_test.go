package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newSearchService(t *testing.T) (*services.SearchService, *repository.MeetingRepository, *repository.SearchRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingRepo := repository.NewMeetingRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	svc := services.NewSearchService(searchRepo, meetingRepo)
	return svc, meetingRepo, searchRepo
}

func TestSearchService_Search(t *testing.T) {
	svc, meetingRepo, searchRepo := newSearchService(t)
	ctx := context.Background()

	now := time.Now().UTC()
	m := &models.Meeting{
		ID:        "m-1",
		Title:     "Sprint Planning",
		StartedAt: &now,
		Status:    models.StatusCompleted,
		CreatedAt: now,
	}
	if err := meetingRepo.Create(ctx, m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := searchRepo.UpsertMeeting(ctx, "m-1", "Sprint Planning", "", "", "", ""); err != nil {
		t.Fatalf("upsert fts: %v", err)
	}

	results, err := svc.Search(ctx, "Sprint")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MeetingID != "m-1" {
		t.Errorf("MeetingID = %q, want 'm-1'", results[0].MeetingID)
	}
	if results[0].MeetingTitle != "Sprint Planning" {
		t.Errorf("MeetingTitle = %q, want 'Sprint Planning'", results[0].MeetingTitle)
	}
}

func TestSearchService_Search_EmptyQuery(t *testing.T) {
	svc, _, _ := newSearchService(t)
	_, err := svc.Search(context.Background(), "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for empty query, got %T: %v", err, err)
	}
}

func TestSearchService_Search_ShortQuery(t *testing.T) {
	svc, _, _ := newSearchService(t)
	_, err := svc.Search(context.Background(), "a")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError for 1-char query, got %T: %v", err, err)
	}
}

func TestSearchService_Search_NoResults(t *testing.T) {
	svc, _, _ := newSearchService(t)
	results, err := svc.Search(context.Background(), "inexistente")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
