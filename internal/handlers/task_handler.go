package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type TaskHandler struct {
	svc        *services.TaskService
	meetingSvc *services.MeetingService
}

func NewTaskHandler(svc *services.TaskService, meetingSvc *services.MeetingService) *TaskHandler {
	return &TaskHandler{svc: svc, meetingSvc: meetingSvc}
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
	task, err := h.svc.Create(r.Context(), meetingID, req.Description, req.Assignee, dueDate, req.Priority)
	if err != nil {
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
	task, err := h.svc.Update(r.Context(), id, req.Description, req.Assignee, dueDate, req.Priority, req.Completed)
	if err != nil {
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
	if err := h.svc.Delete(r.Context(), id); err != nil {
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
	meeting, err := h.meetingSvc.GetByID(r.Context(), meetingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get meeting")
		return
	}
	tasks, err := h.svc.Generate(r.Context(), meeting)
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
