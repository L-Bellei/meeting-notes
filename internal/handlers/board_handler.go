package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type BoardHandler struct {
	columnSvc *services.BoardColumnService
	cardSvc   *services.BoardCardService // wired in Task 3
}

func NewBoardHandler(columnSvc *services.BoardColumnService, cardSvc *services.BoardCardService) *BoardHandler {
	return &BoardHandler{columnSvc: columnSvc, cardSvc: cardSvc}
}

func (h *BoardHandler) ListColumns(w http.ResponseWriter, r *http.Request) {
	cols, err := h.columnSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list columns")
		return
	}
	if cols == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, cols)
}

func (h *BoardHandler) CreateColumn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	col, err := h.columnSvc.Create(r.Context(), req.Name)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create column")
		return
	}
	writeJSON(w, http.StatusCreated, col)
}

func (h *BoardHandler) UpdateColumn(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	col, err := h.columnSvc.Update(r.Context(), id, req.Name)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "column not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update column")
		return
	}
	writeJSON(w, http.StatusOK, col)
}

func (h *BoardHandler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	moveTo := r.URL.Query().Get("move_to")
	err := h.columnSvc.Delete(r.Context(), id, moveTo)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		var chce *services.ColumnHasCardsError
		if errors.As(err, &chce) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":       "column has cards",
				"cards_count": chce.Count,
			})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "column not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete column")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BoardHandler) ReorderColumns(w http.ResponseWriter, r *http.Request) {
	var items []repository.ReorderItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.columnSvc.Reorder(r.Context(), items); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder columns")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
