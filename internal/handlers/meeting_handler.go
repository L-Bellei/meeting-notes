package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type MeetingHandler struct {
	svc *services.MeetingService
}

func NewMeetingHandler(svc *services.MeetingService) *MeetingHandler {
	return &MeetingHandler{svc: svc}
}

type createMeetingRequest struct {
	Title     string  `json:"title"`
	ThemeID   string  `json:"theme_id"`
	StartedAt *string `json:"started_at"`
	Status    string  `json:"status"`
}

type updateMeetingRequest struct {
	Title           string  `json:"title"`
	ThemeID         *string `json:"theme_id"`
	StartedAt       *string `json:"started_at"`
	Status          string  `json:"status"`
	DurationSeconds *int    `json:"duration_seconds"`
	Transcript      *string `json:"transcript"`
}

type MeetingDetailResponse struct {
	models.Meeting
	Summary   *models.Summary   `json:"summary"`
	KeyPoints []models.KeyPoint `json:"key_points"`
	Tasks     []models.Task     `json:"tasks"`
}

func (h *MeetingHandler) List(w http.ResponseWriter, r *http.Request) {
	themeID := r.URL.Query().Get("theme_id")
	status := r.URL.Query().Get("status")

	meetings, err := h.svc.List(r.Context(), themeID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list meetings")
		return
	}
	if meetings == nil {
		meetings = []models.Meeting{}
	}
	writeJSON(w, http.StatusOK, meetings)
}

func (h *MeetingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var startedAt *time.Time
	if req.StartedAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.StartedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid started_at: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
			return
		}
		startedAt = &parsed
	}
	m, err := h.svc.Create(r.Context(), req.Title, req.ThemeID, req.Status, startedAt)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create meeting")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (h *MeetingHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	writeJSON(w, http.StatusOK, MeetingDetailResponse{
		Meeting:   *m,
		Summary:   nil,
		KeyPoints: []models.KeyPoint{},
		Tasks:     []models.Task{},
	})
}

func (h *MeetingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateMeetingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var startedAt *time.Time
	if req.StartedAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.StartedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid started_at: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
			return
		}
		startedAt = &parsed
	}
	m, err := h.svc.Update(r.Context(), id, req.Title, req.ThemeID, req.Status, startedAt, req.DurationSeconds, req.Transcript)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update meeting")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *MeetingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete meeting")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
