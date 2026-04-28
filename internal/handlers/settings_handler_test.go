package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newSettingsHandler(t *testing.T) *handlers.SettingsHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewSettingsHandler(services.NewSettingsService(repository.NewSettingsRepository(db)))
}

func TestSettingsHandler_Get_OK(t *testing.T) {
	h := newSettingsHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var m map[string]string
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["ai_provider"] != "anthropic" {
		t.Errorf("ai_provider = %q, want anthropic", m["ai_provider"])
	}
}

func TestSettingsHandler_Update_OK(t *testing.T) {
	h := newSettingsHandler(t)
	body, _ := json.Marshal(map[string]string{"user_name": "Leonardo"})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
}

func TestSettingsHandler_Update_InvalidProvider(t *testing.T) {
	h := newSettingsHandler(t)
	body, _ := json.Marshal(map[string]string{"ai_provider": "gemini"})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}
