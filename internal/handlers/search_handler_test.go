package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newSearchHandler(t *testing.T) (*handlers.SearchHandler, *repository.MeetingRepository, *repository.SearchRepository) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingRepo := repository.NewMeetingRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	svc := services.NewSearchService(searchRepo, meetingRepo)
	return handlers.NewSearchHandler(svc), meetingRepo, searchRepo
}

func TestSearchHandler_Search(t *testing.T) {
	h, meetingRepo, searchRepo := newSearchHandler(t)
	ctx := t.Context()

	now := time.Now().UTC()
	m := &models.Meeting{
		ID: "m-1", Title: "Daily Standup",
		StartedAt: &now, Status: models.StatusCompleted, CreatedAt: now,
	}
	if err := meetingRepo.Create(ctx, m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := searchRepo.UpsertMeeting(ctx, "m-1", "Daily Standup", "", "", "", ""); err != nil {
		t.Fatalf("upsert fts: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=Daily", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var results []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["meeting_id"] != "m-1" {
		t.Errorf("meeting_id = %v, want 'm-1'", results[0]["meeting_id"])
	}
}

func TestSearchHandler_MissingQuery(t *testing.T) {
	h, _, _ := newSearchHandler(t)
	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSearchHandler_ShortQuery(t *testing.T) {
	h, _, _ := newSearchHandler(t)
	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=a", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSearchHandler_EmptyResults(t *testing.T) {
	h, _, _ := newSearchHandler(t)
	r := chi.NewRouter()
	r.Get("/api/search", h.Search)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=inexistente", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var results []any
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
