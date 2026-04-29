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

type ThemeHandler struct {
	svc *services.ThemeService
}

func NewThemeHandler(svc *services.ThemeService) *ThemeHandler {
	return &ThemeHandler{svc: svc}
}

type createThemeRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Color        string  `json:"color"`
	ParentID     *string `json:"parent_id"`
	CustomPrompt string  `json:"custom_prompt"`
}

type updateThemeRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Color        string  `json:"color"`
	ParentID     *string `json:"parent_id"`
	CustomPrompt string  `json:"custom_prompt"`
}

func (h *ThemeHandler) List(w http.ResponseWriter, r *http.Request) {
	themes, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list themes")
		return
	}
	if themes == nil {
		themes = []models.Theme{}
	}
	writeJSON(w, http.StatusOK, themes)
}

func (h *ThemeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	theme, err := h.svc.Create(r.Context(), req.Name, req.Description, req.Color, req.ParentID, req.CustomPrompt)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrDuplicate) {
			writeError(w, http.StatusConflict, "theme name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create theme")
		return
	}
	writeJSON(w, http.StatusCreated, theme)
}

func (h *ThemeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	theme, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "theme not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get theme")
		return
	}
	writeJSON(w, http.StatusOK, theme)
}

func (h *ThemeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateThemeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	theme, err := h.svc.Update(r.Context(), id, req.Name, req.Description, req.Color, req.ParentID, req.CustomPrompt)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "theme not found")
			return
		}
		if errors.Is(err, repository.ErrDuplicate) {
			writeError(w, http.StatusConflict, "theme name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update theme")
		return
	}
	writeJSON(w, http.StatusOK, theme)
}

func (h *ThemeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "theme not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete theme")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
