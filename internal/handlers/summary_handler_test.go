package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeSummaryAI struct {
	text string
	err  error
}

func (f *fakeSummaryAI) GenerateSummary(ctx context.Context, transcript, notes string) (string, int, int, error) {
	if f.err != nil {
		return "", 0, 0, f.err
	}
	return f.text, 100, 50, nil
}
func (f *fakeSummaryAI) GenerateKeyPoints(ctx context.Context, transcript, notes string) ([]string, int, int, error) {
	return nil, 0, 0, nil
}
func (f *fakeSummaryAI) GenerateTasks(ctx context.Context, transcript, notes string) ([]ai.TaskSuggestion, int, int, error) {
	return nil, 0, 0, nil
}

func newTestSummaryHandler(t *testing.T, aiClient ai.AIClient) (*handlers.SummaryHandler, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingSvc := services.NewMeetingService(repository.NewMeetingRepository(db), repository.NewThemeRepository(db))
	transcript := "Falamos sobre o roadmap."
	m, err := meetingSvc.Create(context.Background(), "Reunião", "", "completed", nil)
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	// Set transcript directly via repo since service Create doesn't accept it
	m.Transcript = &transcript
	if err := repository.NewMeetingRepository(db).Update(context.Background(), m); err != nil {
		t.Fatalf("set transcript: %v", err)
	}

	summarySvc := services.NewSummaryService(repository.NewSummaryRepository(db), aiClient)
	h := handlers.NewSummaryHandler(summarySvc, meetingSvc)
	return h, m.ID
}

func TestSummaryHandler_Get_NotFound(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSummaryHandler_Create(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	body := `{"content":"Resumo manual","model_used":"manual"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(body)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var s models.Summary
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Content != "Resumo manual" {
		t.Errorf("Content = %q", s.Content)
	}
}

func TestSummaryHandler_Create_ContentRequired(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(`{"model_used":"x"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestSummaryHandler_Get(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(`{"content":"X","model_used":"m"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Get(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestSummaryHandler_Delete(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary", bytes.NewBufferString(`{"content":"X","model_used":"m"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestSummaryHandler_Delete_NotFound(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/summary", nil), mID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSummaryHandler_Generate(t *testing.T) {
	fake := &fakeSummaryAI{text: "Resumo via AI"}
	h, mID := newTestSummaryHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var s models.Summary
	if err := json.NewDecoder(w.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Content != "Resumo via AI" {
		t.Errorf("Content = %q", s.Content)
	}
}

func TestSummaryHandler_Generate_AINotConfigured(t *testing.T) {
	h, mID := newTestSummaryHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/summary/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestSummaryHandler_Generate_MeetingNotFound(t *testing.T) {
	fake := &fakeSummaryAI{text: "x"}
	h, _ := newTestSummaryHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/nope/summary/generate", nil), "nope")
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
