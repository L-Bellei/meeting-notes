package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/services"
)

type mockRetranscribeOrch struct {
	err error
}

func (m *mockRetranscribeOrch) RetranscribeRecording(_ context.Context, _ string) error {
	return m.err
}

func newRetranscribeRequest(meetingID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/meetings/"+meetingID+"/retranscribe", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", meetingID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	return r
}

func TestRetranscribeHandler_Success(t *testing.T) {
	h := handlers.NewRetranscribeHandler(&mockRetranscribeOrch{})
	w := httptest.NewRecorder()
	h.Retranscribe(w, newRetranscribeRequest("meet-1"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", w.Code)
	}
}

func TestRetranscribeHandler_NoAudioPath(t *testing.T) {
	h := handlers.NewRetranscribeHandler(&mockRetranscribeOrch{
		err: &services.ValidationError{Message: "nenhum arquivo de áudio disponível para transcrição"},
	})
	w := httptest.NewRecorder()
	h.Retranscribe(w, newRetranscribeRequest("meet-1"))
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", w.Code)
	}
}
