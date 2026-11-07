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

type KeyPointHandler struct {
	svc        *services.KeyPointService
	meetingSvc *services.MeetingService
	themeRepo  *repository.ThemeRepository
	operations *services.MeetingOperationCoordinator
}

func NewKeyPointHandler(svc *services.KeyPointService, meetingSvc *services.MeetingService, themeRepo *repository.ThemeRepository, coordinators ...*services.MeetingOperationCoordinator) *KeyPointHandler {
	h := &KeyPointHandler{svc: svc, meetingSvc: meetingSvc, themeRepo: themeRepo}
	if len(coordinators) > 0 {
		h.operations = coordinators[0]
	}
	return h
}

type keyPointRequest struct {
	Position int    `json:"position"`
	Content  string `json:"content"`
}

func (h *KeyPointHandler) List(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	kps, err := h.svc.List(r.Context(), meetingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list key points")
		return
	}
	writeJSON(w, http.StatusOK, kps)
}

func (h *KeyPointHandler) Create(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req keyPointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	kp, err := runMeetingIdle(h.operations, meetingID, func() (*models.KeyPoint, error) {
		return h.svc.Create(r.Context(), meetingID, req.Content, req.Position)
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
		writeError(w, http.StatusInternalServerError, "failed to create key point")
		return
	}
	writeJSON(w, http.StatusCreated, kp)
}

func (h *KeyPointHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "kpId")
	var req keyPointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	meetingID := chi.URLParam(r, "id")
	kp, err := runMeetingIdle(h.operations, meetingID, func() (*models.KeyPoint, error) {
		return h.svc.Update(r.Context(), id, req.Content, req.Position)
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
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "key point not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update key point")
		return
	}
	writeJSON(w, http.StatusOK, kp)
}

func (h *KeyPointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "kpId")
	meetingID := chi.URLParam(r, "id")
	if err := runMeetingIdleError(h.operations, meetingID, func() error {
		return h.svc.Delete(r.Context(), id)
	}); err != nil {
		if writeMeetingBusy(w, err) {
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "key point not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete key point")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *KeyPointHandler) Generate(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	if h.operations != nil {
		if err := h.operations.Begin(meetingID, "key_points"); err != nil {
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
	kps, err := h.svc.Generate(r.Context(), meeting, customPrompt)
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
	writeJSON(w, http.StatusCreated, kps)
}
