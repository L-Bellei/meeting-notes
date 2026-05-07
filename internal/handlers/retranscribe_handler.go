package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type retranscribeOrchestrator interface {
	RetranscribeRecording(ctx context.Context, meetingID string) error
}

type RetranscribeHandler struct {
	orch retranscribeOrchestrator
}

func NewRetranscribeHandler(orch retranscribeOrchestrator) *RetranscribeHandler {
	return &RetranscribeHandler{orch: orch}
}

func (h *RetranscribeHandler) Retranscribe(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.RetranscribeRecording(r.Context(), id); err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reunião não encontrada"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "transcribing"})
}
