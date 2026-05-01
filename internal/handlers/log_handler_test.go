package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func newLogHandler(t *testing.T) *handlers.LogHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewLogHandler(repository.NewLogRepository(db))
}

func TestLogHandler_List_EmptyReturns200WithArray(t *testing.T) {
	h := newLogHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var logs []models.AppLog
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if logs == nil {
		t.Fatal("expected non-nil array, got nil")
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}
}

func TestLogHandler_List_ReturnsInsertedLogs(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := repository.NewLogRepository(db)
	if err := repo.Insert(t.Context(), "error", "orchestrator", "something went wrong", nil); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := repo.Insert(t.Context(), "warn", "orchestrator", "minor issue", nil); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	h := handlers.NewLogHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var logs []models.AppLog
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
}

func TestLogHandler_List_LimitQueryParam(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := repository.NewLogRepository(db)
	for i := 0; i < 5; i++ {
		if err := repo.Insert(t.Context(), "info", "test", "msg", nil); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	h := handlers.NewLogHandler(repo)
	req := httptest.NewRequest(http.MethodGet, "/api/logs?limit=2", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var logs []models.AppLog
	if err := json.NewDecoder(w.Body).Decode(&logs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs with limit=2, got %d", len(logs))
	}
}
