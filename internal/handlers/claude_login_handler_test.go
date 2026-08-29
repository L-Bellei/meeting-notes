package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/handlers"
)

func TestClaudeLoginHandler_Login_Launched(t *testing.T) {
	h := handlers.NewClaudeLoginHandler(func() error { return nil })
	w := httptest.NewRecorder()
	h.Login(w, httptest.NewRequest(http.MethodPost, "/api/ai/claude-login", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "launched" {
		t.Fatalf("want status=launched, got %v", body)
	}
}

func TestClaudeLoginHandler_Login_NotConfigured(t *testing.T) {
	h := handlers.NewClaudeLoginHandler(func() error {
		return fmt.Errorf("wrap: %w", ai.ErrNotConfigured)
	})
	w := httptest.NewRecorder()
	h.Login(w, httptest.NewRequest(http.MethodPost, "/api/ai/claude-login", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] == nil {
		t.Fatal("want error field")
	}
}

func TestClaudeLoginHandler_Login_GenericError(t *testing.T) {
	h := handlers.NewClaudeLoginHandler(func() error { return errors.New("boom") })
	w := httptest.NewRecorder()
	h.Login(w, httptest.NewRequest(http.MethodPost, "/api/ai/claude-login", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", w.Code)
	}
}

func TestAITestHandler_Test_OK(t *testing.T) {
	h := handlers.NewAITestHandler(func(_ context.Context) error { return nil })
	w := httptest.NewRecorder()
	h.Test(w, httptest.NewRequest(http.MethodPost, "/api/ai/test", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["ok"] != true {
		t.Fatalf("want ok=true, got %v", body)
	}
}

func TestAITestHandler_Test_AuthError(t *testing.T) {
	h := handlers.NewAITestHandler(func(_ context.Context) error {
		return errors.New("OAuth token expired. Please run /login")
	})
	w := httptest.NewRecorder()
	h.Test(w, httptest.NewRequest(http.MethodPost, "/api/ai/test", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	msg, _ := body["error"].(string)
	if msg == "" {
		t.Fatal("want error field")
	}
	if !strings.Contains(msg, "reconecte") {
		t.Fatalf("want message containing 'reconecte', got %q", msg)
	}
}

func TestAITestHandler_Test_GenericError(t *testing.T) {
	h := handlers.NewAITestHandler(func(_ context.Context) error { return errors.New("boom") })
	w := httptest.NewRecorder()
	h.Test(w, httptest.NewRequest(http.MethodPost, "/api/ai/test", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "boom" {
		t.Fatalf("want error=boom, got %v", body)
	}
}

