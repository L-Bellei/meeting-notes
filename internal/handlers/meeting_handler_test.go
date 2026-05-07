package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"meeting-notes/internal/audio"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

// fakeOrchestrator implements handlers.MeetingOrchestrator for tests.
type fakeOrchestrator struct {
	startErr         error
	stopErr          error
	reprocessErr     error
	setTranscriptErr error

	lastMeetingID  string
	lastTranscript string
}

func (f *fakeOrchestrator) StartRecording(_ context.Context, meetingID string) error {
	f.lastMeetingID = meetingID
	return f.startErr
}

func (f *fakeOrchestrator) StopRecording(_ context.Context, meetingID string, _ bool) error {
	f.lastMeetingID = meetingID
	return f.stopErr
}

func (f *fakeOrchestrator) Reprocess(_ context.Context, meetingID string) error {
	f.lastMeetingID = meetingID
	return f.reprocessErr
}

func (f *fakeOrchestrator) SetTranscriptAndProcess(_ context.Context, meetingID, transcript string) error {
	f.lastMeetingID = meetingID
	f.lastTranscript = transcript
	return f.setTranscriptErr
}

func newTestMeetingHandler(t *testing.T) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db), repository.NewThemeRepository(db), nil, nil, nil, nil),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
		nil,
	)
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
	var detail handlers.MeetingDetailResponse
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

func TestMeetingHandler_Create_InvalidStartedAt(t *testing.T) {
	h := newTestMeetingHandler(t)
	body := `{"title":"Test","started_at":"not-a-date"}`
	req := httptest.NewRequest(http.MethodPost, "/api/meetings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMeetingHandler_Update_InvalidStartedAt(t *testing.T) {
	h := newTestMeetingHandler(t)

	// First create a meeting
	reqC := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(`{"title":"Original"}`))
	reqC.Header.Set("Content-Type", "application/json")
	wC := httptest.NewRecorder()
	h.Create(wC, reqC)
	if wC.Code != http.StatusCreated {
		t.Fatalf("create failed with status %d", wC.Code)
	}
	var created models.Meeting
	if err := json.NewDecoder(wC.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	// Now try to update with invalid started_at
	updateBody := `{"title":"Updated","started_at":"not-a-date"}`
	req := withChiID(
		httptest.NewRequest(http.MethodPut, "/api/meetings/"+created.ID, strings.NewReader(updateBody)),
		created.ID,
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func newTestMeetingAndThemeHandlers(t *testing.T) (*handlers.MeetingHandler, *handlers.ThemeHandler) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mh := handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db), repository.NewThemeRepository(db), nil, nil, nil, nil),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
		nil,
	)
	th := handlers.NewThemeHandler(services.NewThemeService(repository.NewThemeRepository(db)))
	return mh, th
}

func TestMeetingHandler_List_FilterByTheme(t *testing.T) {
	mh, th := newTestMeetingAndThemeHandlers(t)

	// Create a theme
	themeBody := `{"name":"Engineering"}`
	reqT := httptest.NewRequest(http.MethodPost, "/api/themes", bytes.NewBufferString(themeBody))
	reqT.Header.Set("Content-Type", "application/json")
	wT := httptest.NewRecorder()
	th.Create(wT, reqT)
	if wT.Code != http.StatusCreated {
		t.Fatalf("create theme failed with status %d: %s", wT.Code, wT.Body.String())
	}
	var theme models.Theme
	if err := json.NewDecoder(wT.Body).Decode(&theme); err != nil {
		t.Fatalf("decode theme: %v", err)
	}

	// Create a meeting with the theme
	m1Body := `{"title":"Meeting with theme","theme_id":"` + theme.ID + `"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(m1Body))
	req1.Header.Set("Content-Type", "application/json")
	mh.Create(httptest.NewRecorder(), req1)

	// Create a meeting without a theme
	m2Body := `{"title":"Meeting without theme"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/meetings", bytes.NewBufferString(m2Body))
	req2.Header.Set("Content-Type", "application/json")
	mh.Create(httptest.NewRecorder(), req2)

	// List meetings filtered by theme_id
	req := httptest.NewRequest(http.MethodGet, "/api/meetings?theme_id="+theme.ID, nil)
	w := httptest.NewRecorder()
	mh.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []models.Meeting
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 meeting, got %d", len(result))
	}
	if result[0].Title != "Meeting with theme" {
		t.Errorf("Title = %q, want %q", result[0].Title, "Meeting with theme")
	}
}

func TestMeetingHandler_GetByID_PopulatesNestedData(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mr := repository.NewMeetingRepository(db)
	sr := repository.NewSummaryRepository(db)
	kpr := repository.NewKeyPointRepository(db)
	tr := repository.NewTaskRepository(db)

	mh := handlers.NewMeetingHandler(services.NewMeetingService(mr, repository.NewThemeRepository(db), nil, nil, nil, nil), sr, kpr, tr, nil)

	now := time.Now().UTC()
	m := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusCompleted}
	if err := mr.Create(context.Background(), m); err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := sr.Upsert(context.Background(), &models.Summary{ID: "s-1", MeetingID: "m-1", Content: "Resumo", ModelUsed: "manual"}); err != nil {
		t.Fatalf("upsert summary: %v", err)
	}
	if err := kpr.Create(context.Background(), &models.KeyPoint{ID: "kp-1", MeetingID: "m-1", Position: 0, Content: "Ponto"}); err != nil {
		t.Fatalf("create kp: %v", err)
	}
	if err := tr.Create(context.Background(), &models.Task{ID: "t-1", MeetingID: "m-1", Description: "Task", Priority: models.PriorityMedium}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	req := withChiID(httptest.NewRequest(http.MethodGet, "/api/meetings/m-1", nil), "m-1")
	w := httptest.NewRecorder()
	mh.GetByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var detail handlers.MeetingDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.Summary == nil || detail.Summary.Content != "Resumo" {
		t.Errorf("Summary = %+v", detail.Summary)
	}
	if len(detail.KeyPoints) != 1 || detail.KeyPoints[0].Content != "Ponto" {
		t.Errorf("KeyPoints = %+v", detail.KeyPoints)
	}
	if len(detail.Tasks) != 1 || detail.Tasks[0].Description != "Task" {
		t.Errorf("Tasks = %+v", detail.Tasks)
	}
}

// newOrchHandler creates a MeetingHandler with a fakeOrchestrator for orchestration tests.
func newOrchHandler(t *testing.T, fo *fakeOrchestrator) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db), repository.NewThemeRepository(db), nil, nil, nil, nil),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
		fo,
	)
}

func TestMeetingHandler_Start_Success(t *testing.T) {
	fo := &fakeOrchestrator{}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/start", nil), "abc")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202; body: %s", w.Code, w.Body.String())
	}
	if fo.lastMeetingID != "abc" {
		t.Errorf("lastMeetingID = %q, want %q", fo.lastMeetingID, "abc")
	}
}

func TestMeetingHandler_Start_AudioServiceDown(t *testing.T) {
	fo := &fakeOrchestrator{startErr: audio.ErrAudioServiceUnavailable}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/start", nil), "abc")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestMeetingHandler_Start_Conflict(t *testing.T) {
	fo := &fakeOrchestrator{startErr: audio.ErrAudioServiceConflict}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/start", nil), "abc")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestMeetingHandler_Start_NotFound(t *testing.T) {
	fo := &fakeOrchestrator{startErr: repository.ErrNotFound}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/nope/start", nil), "nope")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_Stop_Success(t *testing.T) {
	fo := &fakeOrchestrator{}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/stop", nil), "abc")
	w := httptest.NewRecorder()
	h.Stop(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202; body: %s", w.Code, w.Body.String())
	}
}

func TestMeetingHandler_Stop_NotRecording(t *testing.T) {
	fo := &fakeOrchestrator{stopErr: &services.ValidationError{Message: "meeting is not recording"}}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/stop", nil), "abc")
	w := httptest.NewRecorder()
	h.Stop(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_Process_Success(t *testing.T) {
	fo := &fakeOrchestrator{}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/process", nil), "abc")
	w := httptest.NewRecorder()
	h.Process(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202; body: %s", w.Code, w.Body.String())
	}
}

func TestMeetingHandler_Process_NoTranscript(t *testing.T) {
	fo := &fakeOrchestrator{reprocessErr: &services.ValidationError{Message: "transcript is required for processing"}}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/process", nil), "abc")
	w := httptest.NewRecorder()
	h.Process(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_SetTranscript_Success(t *testing.T) {
	fo := &fakeOrchestrator{}
	h := newOrchHandler(t, fo)
	body := `{"transcript":"hello world"}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/transcript", strings.NewReader(body)), "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SetTranscript(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202; body: %s", w.Code, w.Body.String())
	}
	if fo.lastTranscript != "hello world" {
		t.Errorf("lastTranscript = %q, want %q", fo.lastTranscript, "hello world")
	}
}

func TestMeetingHandler_SetTranscript_EmptyBody(t *testing.T) {
	fo := &fakeOrchestrator{setTranscriptErr: &services.ValidationError{Message: "transcript is required"}}
	h := newOrchHandler(t, fo)
	body := `{}`
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/transcript", strings.NewReader(body)), "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SetTranscript(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_SetTranscript_BadJSON(t *testing.T) {
	fo := &fakeOrchestrator{}
	h := newOrchHandler(t, fo)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/transcript", strings.NewReader("not json")), "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SetTranscript(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
