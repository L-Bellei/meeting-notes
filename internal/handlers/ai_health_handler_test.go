package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/handlers"
)

func TestAIHealthHandler_NotConfigured(t *testing.T) {
	h := handlers.NewAIHealthHandler(func(_ context.Context) (bool, error) {
		return false, nil
	})
	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequest(http.MethodGet, "/api/ai/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["configured"] != false {
		t.Fatalf("want configured=false, got %v", body["configured"])
	}
}

func TestAIHealthHandler_ValidKey(t *testing.T) {
	h := handlers.NewAIHealthHandler(func(_ context.Context) (bool, error) {
		return true, nil
	})
	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequest(http.MethodGet, "/api/ai/health", nil))
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["configured"] != true || body["valid"] != true {
		t.Fatalf("want configured=true valid=true, got %v", body)
	}
}

func TestAIHealthHandler_InvalidKey(t *testing.T) {
	h := handlers.NewAIHealthHandler(func(_ context.Context) (bool, error) {
		return true, errors.New("authentication_error")
	})
	w := httptest.NewRecorder()
	h.Check(w, httptest.NewRequest(http.MethodGet, "/api/ai/health", nil))
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["configured"] != true || body["valid"] != false {
		t.Fatalf("want configured=true valid=false, got %v", body)
	}
	if body["error"] == nil {
		t.Fatal("want error field")
	}
}
