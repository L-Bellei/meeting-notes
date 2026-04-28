package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type SummaryHandler struct {
	svc        *services.SummaryService
	meetingSvc *services.MeetingService
}

func NewSummaryHandler(svc *services.SummaryService, meetingSvc *services.MeetingService) *SummaryHandler {
	return &SummaryHandler{svc: svc, meetingSvc: meetingSvc}
}

type createSummaryRequest struct {
	Content   string `json:"content"`
	ModelUsed string `json:"model_used"`
}

func (h *SummaryHandler) Get(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	s, err := h.svc.Get(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "summary not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get summary")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SummaryHandler) Create(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req createSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.Upsert(r.Context(), meetingID, req.Content, req.ModelUsed)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create summary")
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *SummaryHandler) Update(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req createSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	s, err := h.svc.Upsert(r.Context(), meetingID, req.Content, req.ModelUsed)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update summary")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *SummaryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), meetingID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "summary not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete summary")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SummaryHandler) Generate(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	meeting, err := h.meetingSvc.GetByID(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	s, err := h.svc.Generate(r.Context(), meeting)
	if err != nil {
		if errors.Is(err, services.ErrAINotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "AI service not configured")
			return
		}
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "AI generation failed")
		return
	}
	writeJSON(w, http.StatusCreated, s)
}
