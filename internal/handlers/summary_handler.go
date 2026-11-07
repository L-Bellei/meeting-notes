package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type SummaryHandler struct {
	svc        *services.SummaryService
	meetingSvc *services.MeetingService
	themeRepo  *repository.ThemeRepository
	operations *services.MeetingOperationCoordinator
}

func NewSummaryHandler(svc *services.SummaryService, meetingSvc *services.MeetingService, themeRepo *repository.ThemeRepository, coordinators ...*services.MeetingOperationCoordinator) *SummaryHandler {
	h := &SummaryHandler{svc: svc, meetingSvc: meetingSvc, themeRepo: themeRepo}
	if len(coordinators) > 0 {
		h.operations = coordinators[0]
	}
	return h
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
	s, err := runMeetingIdle(h.operations, meetingID, func() (*models.Summary, error) {
		return h.svc.Upsert(r.Context(), meetingID, req.Content, req.ModelUsed)
	})
	if err != nil {
		if writeMeetingBusy(w, err) {
			return
		}
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
	s, err := runMeetingIdle(h.operations, meetingID, func() (*models.Summary, error) {
		return h.svc.Upsert(r.Context(), meetingID, req.Content, req.ModelUsed)
	})
	if err != nil {
		if writeMeetingBusy(w, err) {
			return
		}
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
	if err := runMeetingIdleError(h.operations, meetingID, func() error {
		return h.svc.Delete(r.Context(), meetingID)
	}); err != nil {
		if writeMeetingBusy(w, err) {
			return
		}
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
	if h.operations != nil {
		if err := h.operations.Begin(meetingID, "summary"); err != nil {
			writeError(w, http.StatusConflict, "meeting already has an active operation")
			return
		}
		defer h.operations.End(meetingID)
	}
	meeting, err := h.meetingSvc.GetByID(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	customPrompt := ""
	if meeting.ThemeID != nil {
		if theme, err := h.themeRepo.GetByID(r.Context(), *meeting.ThemeID); err == nil {
			customPrompt = theme.CustomPrompt
		}
	}
	s, err := h.svc.Generate(r.Context(), meeting, customPrompt)
	if err != nil {
		if errors.Is(err, services.ErrAINotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "IA não configurada — abra Configurações → IA")
			return
		}
		if errors.Is(err, services.ErrAIAuthFailed) {
			writeError(w, http.StatusBadGateway, "Chave de API inválida ou expirada — verifique em Configurações → IA")
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
