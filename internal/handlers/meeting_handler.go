package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/audio"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

// MeetingOrchestrator abstracts the orchestration service for testability.
type MeetingOrchestrator interface {
	StartRecording(ctx context.Context, meetingID string) error
	StopRecording(ctx context.Context, meetingID string) error
	Reprocess(ctx context.Context, meetingID string) error
	SetTranscriptAndProcess(ctx context.Context, meetingID, transcript string) error
}

type MeetingHandler struct {
	svc          *services.MeetingService
	summaryRepo  *repository.SummaryRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
	orch         MeetingOrchestrator
}

func NewMeetingHandler(
	svc *services.MeetingService,
	summaryRepo *repository.SummaryRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
	orch MeetingOrchestrator,
) *MeetingHandler {
	return &MeetingHandler{
		svc:          svc,
		summaryRepo:  summaryRepo,
		keyPointRepo: keyPointRepo,
		taskRepo:     taskRepo,
		orch:         orch,
	}
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
	Notes           *string `json:"notes"`
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

	var summary *models.Summary
	s, err := h.summaryRepo.GetByMeetingID(r.Context(), id)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "failed to get summary")
		return
	}
	if err == nil {
		summary = s
	}

	keyPoints, err := h.keyPointRepo.ListByMeetingID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get key points")
		return
	}
	tasks, err := h.taskRepo.ListByMeetingID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get tasks")
		return
	}

	writeJSON(w, http.StatusOK, MeetingDetailResponse{
		Meeting:   *m,
		Summary:   summary,
		KeyPoints: keyPoints,
		Tasks:     tasks,
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
	m, err := h.svc.Update(r.Context(), id, req.Title, req.ThemeID, req.Status, startedAt, req.DurationSeconds, req.Transcript, req.Notes)
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

// POST /api/meetings/{id}/start
func (h *MeetingHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.StartRecording(r.Context(), id); err != nil {
		writeOrchError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// POST /api/meetings/{id}/stop
func (h *MeetingHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.StopRecording(r.Context(), id); err != nil {
		writeOrchError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// POST /api/meetings/{id}/process
func (h *MeetingHandler) Process(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.Reprocess(r.Context(), id); err != nil {
		writeOrchError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type setTranscriptRequest struct {
	Transcript string `json:"transcript"`
}

// POST /api/meetings/{id}/transcript
func (h *MeetingHandler) SetTranscript(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setTranscriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.orch.SetTranscriptAndProcess(r.Context(), id, req.Transcript); err != nil {
		writeOrchError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func writeOrchError(w http.ResponseWriter, err error) {
	if errors.Is(err, audio.ErrAudioServiceUnavailable) || errors.Is(err, audio.ErrAudioGenericError) {
		writeError(w, http.StatusServiceUnavailable, "audio service unavailable")
		return
	}
	if errors.Is(err, audio.ErrAudioServiceConflict) {
		writeError(w, http.StatusConflict, "audio service conflict")
		return
	}
	var ve *services.ValidationError
	if errors.As(err, &ve) {
		writeError(w, http.StatusUnprocessableEntity, ve.Message)
		return
	}
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "meeting not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal server error")
}
