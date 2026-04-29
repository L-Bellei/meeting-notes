package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeKeyPointAI struct {
	points []string
}

func (f *fakeKeyPointAI) GenerateSummary(ctx context.Context, transcript, notes, customPrompt string) (string, int, int, error) {
	return "", 0, 0, nil
}
func (f *fakeKeyPointAI) GenerateKeyPoints(ctx context.Context, transcript, notes, customPrompt string) ([]string, int, int, error) {
	return f.points, 100, 50, nil
}
func (f *fakeKeyPointAI) GenerateTasks(ctx context.Context, transcript, notes, customPrompt string) ([]ai.TaskSuggestion, int, int, error) {
	return nil, 0, 0, nil
}

func withChiIDAndKpID(req *http.Request, id, kpID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	rctx.URLParams.Add("kpId", kpID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newTestKeyPointHandler(t *testing.T, aiClient ai.AIClient) (*handlers.KeyPointHandler, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	meetingSvc := services.NewMeetingService(repository.NewMeetingRepository(db), repository.NewThemeRepository(db))
	transcript := "x"
	m, _ := meetingSvc.Create(context.Background(), "R", "", "completed", nil)
	m.Transcript = &transcript
	repository.NewMeetingRepository(db).Update(context.Background(), m)

	kpSvc := services.NewKeyPointService(repository.NewKeyPointRepository(db), aiClient)
	return handlers.NewKeyPointHandler(kpSvc, meetingSvc, repository.NewThemeRepository(db)), m.ID
}

func TestKeyPointHandler_List_Empty(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/key_points", nil), mID)
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var kps []models.KeyPoint
	if err := json.NewDecoder(w.Body).Decode(&kps); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(kps) != 0 {
		t.Errorf("expected 0, got %d", len(kps))
	}
}

func TestKeyPointHandler_Create(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	body := `{"position":0,"content":"Ponto 1"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(body)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var kp models.KeyPoint
	json.NewDecoder(w.Body).Decode(&kp)
	if kp.Content != "Ponto 1" {
		t.Errorf("Content = %q", kp.Content)
	}
}

func TestKeyPointHandler_Create_ContentRequired(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(`{"position":0}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestKeyPointHandler_Update(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(`{"position":0,"content":"Original"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.KeyPoint
	json.NewDecoder(wC.Body).Decode(&created)

	body := `{"position":5,"content":"Atualizado"}`
	req := withChiIDAndKpID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/key_points/"+created.ID, bytes.NewBufferString(body)), mID, created.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

func TestKeyPointHandler_Update_NotFound(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiIDAndKpID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/key_points/nope", bytes.NewBufferString(`{"content":"x"}`)), mID, "nope")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestKeyPointHandler_Delete(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points", bytes.NewBufferString(`{"content":"X"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.KeyPoint
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiIDAndKpID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/key_points/"+created.ID, nil), mID, created.ID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestKeyPointHandler_Delete_NotFound(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiIDAndKpID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/key_points/nope", nil), mID, "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestKeyPointHandler_Generate(t *testing.T) {
	fake := &fakeKeyPointAI{points: []string{"P1", "P2"}}
	h, mID := newTestKeyPointHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var kps []models.KeyPoint
	json.NewDecoder(w.Body).Decode(&kps)
	if len(kps) != 2 {
		t.Errorf("expected 2 kps, got %d", len(kps))
	}
}

func TestKeyPointHandler_Generate_AINotConfigured(t *testing.T) {
	h, mID := newTestKeyPointHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/key_points/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}
