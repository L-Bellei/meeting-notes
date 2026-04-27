package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestMeetingHandler(t *testing.T) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(services.NewMeetingService(repository.NewMeetingRepository(db)))
}

type meetingDetailResp struct {
	models.Meeting
	Summary   *models.Summary   `json:"summary"`
	KeyPoints []models.KeyPoint `json:"key_points"`
	Tasks     []models.Task     `json:"tasks"`
}

func TestMeetingHandler_List_Empty(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/meetings", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var result []models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d", len(result))
	}
}

func TestMeetingHandler_Create(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"title":"Reunião de planejamento","status":"pending"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}
	var m models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.ID == "" {
		t.Error("ID should be set")
	}
	if m.Title != "Reunião de planejamento" {
		t.Errorf("Title = %q", m.Title)
	}
}

func TestMeetingHandler_Create_TitleRequired(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"status":"pending"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_Create_InvalidStatus(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"title":"Título","status":"invalido"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_GetByID(t *testing.T) {
	h := newTestMeetingHandler(t)

	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Eng"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/"+created.ID, nil), created.ID)
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var detail meetingDetailResp
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.ID != created.ID {
		t.Errorf("ID mismatch")
	}
	if detail.KeyPoints == nil {
		t.Error("key_points should be empty array, not null")
	}
	if detail.Tasks == nil {
		t.Error("tasks should be empty array, not null")
	}
}

func TestMeetingHandler_GetByID_NotFound(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.GetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_Update(t *testing.T) {
	h := newTestMeetingHandler(t)

	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Original"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	updateBody := `{"title":"Atualizado","status":"completed"}`
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/meetings/"+created.ID, bytes.NewBufferString(updateBody)),
		created.ID,
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var updated models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if updated.Title != "Atualizado" {
		t.Errorf("Title = %q", updated.Title)
	}
	if updated.Status != models.StatusCompleted {
		t.Errorf("Status = %q", updated.Status)
	}
}

func TestMeetingHandler_Update_NotFound(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/meetings/nope", bytes.NewBufferString(`{"title":"X"}`)),
		"nope",
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_Delete(t *testing.T) {
	h := newTestMeetingHandler(t)

	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Para deletar"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/"+created.ID, nil), created.ID)
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestMeetingHandler_Delete_NotFound(t *testing.T) {
	h := newTestMeetingHandler(t)
	req := withChiID(httptest.NewRequest(http.MethodDelete, "/api/meetings/nope", nil), "nope")
	w := httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_List_FilterByStatus(t *testing.T) {
	h := newTestMeetingHandler(t)

	for _, body := range []string{
		`{"title":"Pendente","status":"pending"}`,
		`{"title":"Completa","status":"completed"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		h.Create(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/meetings?status=completed", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var result []models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Title != "Completa" {
		t.Errorf("Title = %q", result[0].Title)
	}
}
