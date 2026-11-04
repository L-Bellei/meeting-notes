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
	var m map[string]any
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["ai_provider"] != "claude-code" {
		t.Errorf("ai_provider = %q, want claude-code", m["ai_provider"])
	}
}

func TestSettingsHandler_RedactsClaudeToken(t *testing.T) {
	h := newSettingsHandler(t)
	body, _ := json.Marshal(map[string]string{"claude_code_token": "sk-sensitive-token"})
	updateReq := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	h.Update(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200", updateRec.Code)
	}

	assertRedacted := func(t *testing.T, body []byte) {
		t.Helper()
		if bytes.Contains(body, []byte("sk-sensitive-token")) {
			t.Fatal("response exposed the Claude token")
		}
		var settings map[string]any
		if err := json.Unmarshal(body, &settings); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := settings["claude_code_token"]; ok {
			t.Fatal("response included claude_code_token")
		}
		if settings["claude_code_token_configured"] != true {
			t.Fatalf("claude_code_token_configured = %v, want true", settings["claude_code_token_configured"])
		}
	}

	assertRedacted(t, updateRec.Body.Bytes())

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRec := httptest.NewRecorder()
	h.Get(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", getRec.Code)
	}
	assertRedacted(t, getRec.Body.Bytes())
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
	var m map[string]any
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["user_name"] != "Leonardo" {
		t.Errorf("user_name = %q, want Leonardo", m["user_name"])
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

func TestSettingsHandler_Update_UnknownKey(t *testing.T) {
	h := newSettingsHandler(t)
	body, _ := json.Marshal(map[string]string{"foo": "bar"})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestSettingsHandler_SetOnUpdate(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	settingsRepo := repository.NewSettingsRepository(db)
	settingsSvc := services.NewSettingsService(settingsRepo)
	h := handlers.NewSettingsHandler(settingsSvc)

	var called map[string]string
	h.SetOnUpdate(func(s map[string]string) { called = s })

	body, _ := json.Marshal(map[string]string{"recording_hotkey": "ctrl+alt+r"})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if called == nil {
		t.Fatal("onUpdate was not called")
	}
	if called["recording_hotkey"] != "ctrl+alt+r" {
		t.Fatalf("expected ctrl+alt+r, got %q", called["recording_hotkey"])
	}
}

func TestSettingsHandler_NoOnUpdate(t *testing.T) {
	h := newSettingsHandler(t)

	body, _ := json.Marshal(map[string]string{"recording_hotkey": "ctrl+shift+r"})
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
