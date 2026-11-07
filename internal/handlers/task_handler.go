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

type TaskHandler struct {
	svc        *services.TaskService
	meetingSvc *services.MeetingService
	themeRepo  *repository.ThemeRepository
	operations *services.MeetingOperationCoordinator
}

func NewTaskHandler(svc *services.TaskService, meetingSvc *services.MeetingService, themeRepo *repository.ThemeRepository, coordinators ...*services.MeetingOperationCoordinator) *TaskHandler {
	h := &TaskHandler{svc: svc, meetingSvc: meetingSvc, themeRepo: themeRepo}
	if len(coordinators) > 0 {
		h.operations = coordinators[0]
	}
	return h
}

type createTaskRequest struct {
	Description string  `json:"description"`
	Assignee    *string `json:"assignee"`
	DueDate     *string `json:"due_date"`
	Priority    string  `json:"priority"`
}

type updateTaskRequest struct {
	Description string  `json:"description"`
	Assignee    *string `json:"assignee"`
	DueDate     *string `json:"due_date"`
	Priority    string  `json:"priority"`
	Completed   bool    `json:"completed"`
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	tasks, err := h.svc.List(r.Context(), meetingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dueDate, err := parseOptionalRFC3339(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_date: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
		return
	}
	task, err := runMeetingIdle(h.operations, meetingID, func() (*models.Task, error) {
		return h.svc.Create(r.Context(), meetingID, req.Description, req.Assignee, dueDate, req.Priority)
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
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskId")
	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dueDate, err := parseOptionalRFC3339(req.DueDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid due_date: use RFC3339 (e.g. 2006-01-02T15:04:05Z)")
		return
	}
	meetingID := chi.URLParam(r, "id")
	task, err := runMeetingIdle(h.operations, meetingID, func() (*models.Task, error) {
		return h.svc.Update(r.Context(), id, req.Description, req.Assignee, dueDate, req.Priority, req.Completed)
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
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "taskId")
	meetingID := chi.URLParam(r, "id")
	if err := runMeetingIdleError(h.operations, meetingID, func() error {
		return h.svc.Delete(r.Context(), id)
	}); err != nil {
		if writeMeetingBusy(w, err) {
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) Generate(w http.ResponseWriter, r *http.Request) {
	meetingID := chi.URLParam(r, "id")
	if h.operations != nil {
		if err := h.operations.Begin(meetingID, "tasks"); err != nil {
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
	tasks, err := h.svc.Generate(r.Context(), meeting, customPrompt)
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
	writeJSON(w, http.StatusCreated, tasks)
}

func parseOptionalRFC3339(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
