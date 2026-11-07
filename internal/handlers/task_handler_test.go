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

type fakeTaskAI struct {
	tasks []ai.TaskSuggestion
}

func (f *fakeTaskAI) GenerateSummary(ctx context.Context, transcript, notes, customPrompt string) (string, int, int, error) {
	return "", 0, 0, nil
}
func (f *fakeTaskAI) GenerateKeyPoints(ctx context.Context, transcript, notes, customPrompt string) ([]string, int, int, error) {
	return nil, 0, 0, nil
}
func (f *fakeTaskAI) GenerateTasks(ctx context.Context, transcript, notes, customPrompt string) ([]ai.TaskSuggestion, int, int, error) {
	return f.tasks, 100, 50, nil
}

func withChiIDAndTaskID(req *http.Request, id, taskID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	rctx.URLParams.Add("taskId", taskID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func newTestTaskHandler(t *testing.T, aiClient ai.AIClient) (*handlers.TaskHandler, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mr := repository.NewMeetingRepository(db)
	meetingSvc := services.NewMeetingService(mr, repository.NewThemeRepository(db))
	transcript := "x"
	m, err := meetingSvc.Create(context.Background(), "R", "", "completed", nil)
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	m.Transcript = &transcript
	if err := mr.Update(context.Background(), m); err != nil {
		t.Fatalf("set transcript: %v", err)
	}

	taskSvc := services.NewTaskService(repository.NewTaskRepository(db), aiClient)
	return handlers.NewTaskHandler(taskSvc, meetingSvc, repository.NewThemeRepository(db)), m.ID
}

func TestTaskHandler_List_Empty(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+mID+"/tasks", nil), mID)
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var tasks []models.Task
	json.NewDecoder(w.Body).Decode(&tasks)
	if len(tasks) != 0 {
		t.Errorf("expected 0, got %d", len(tasks))
	}
}

func TestTaskHandler_Create(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	body := `{"description":"Fazer X","priority":"high"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(body)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var task models.Task
	json.NewDecoder(w.Body).Decode(&task)
	if task.Description != "Fazer X" {
		t.Errorf("Description = %q", task.Description)
	}
	if task.Priority != models.PriorityHigh {
		t.Errorf("Priority = %q", task.Priority)
	}
}

func TestTaskHandler_Create_DescriptionRequired(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"priority":"low"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestTaskHandler_Create_InvalidPriority(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"X","priority":"urgent"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestTaskHandler_Create_InvalidDueDate(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"X","due_date":"not-a-date"}`)), mID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTaskHandler_Update(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"Original","priority":"low"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.Task
	json.NewDecoder(wC.Body).Decode(&created)

	body := `{"description":"Atualizada","priority":"high","completed":true}`
	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/tasks/"+created.ID, bytes.NewBufferString(body)), mID, created.ID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated models.Task
	json.NewDecoder(w.Body).Decode(&updated)
	if !updated.Completed {
		t.Error("Completed should be true")
	}
}

func TestTaskHandler_Update_NotFound(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodPut, "/api/meetings/"+mID+"/tasks/nope", bytes.NewBufferString(`{"description":"X"}`)), mID, "nope")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTaskHandler_Delete(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)

	createReq := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks", bytes.NewBufferString(`{"description":"X"}`)), mID)
	createReq.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, createReq)
	var created models.Task
	json.NewDecoder(wC.Body).Decode(&created)

	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/tasks/"+created.ID, nil), mID, created.ID)
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestTaskHandler_Delete_NotFound(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiIDAndTaskID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+mID+"/tasks/nope", nil), mID, "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestTaskHandler_Generate(t *testing.T) {
	fake := &fakeTaskAI{tasks: []ai.TaskSuggestion{
		{Description: "T1", Priority: "high"},
		{Description: "T2", Priority: "low"},
	}}
	h, mID := newTestTaskHandler(t, fake)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var tasks []models.Task
	json.NewDecoder(w.Body).Decode(&tasks)
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestTaskHandler_Generate_AINotConfigured(t *testing.T) {
	h, mID := newTestTaskHandler(t, nil)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/"+mID+"/tasks/generate", nil), mID)
	w := httptest.NewRecorder()
	h.Generate(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}
