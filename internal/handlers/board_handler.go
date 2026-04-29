package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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
			msg := "column not found"
			if moveTo != "" {
				msg = "move_to column not found"
			}
			writeError(w, http.StatusNotFound, msg)
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

func (h *BoardHandler) ListCards(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := repository.BoardCardFilters{}
	if v := q.Get("title"); v != "" {
		f.Title = v
	}
	if v := q.Get("number"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			f.Number = &n
		}
	}
	if v := q.Get("created_after"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			f.CreatedAfter = &t
		}
	}
	if v := q.Get("created_before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			f.CreatedBefore = &t
		}
	}
	if v := q.Get("updated_after"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			f.UpdatedAfter = &t
		}
	}
	if v := q.Get("updated_before"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			f.UpdatedBefore = &t
		}
	}
	cards, err := h.cardSvc.List(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cards")
		return
	}
	writeJSON(w, http.StatusOK, cards)
}

func (h *BoardHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MeetingID string `json:"meeting_id"`
		ColumnID  string `json:"column_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	card, err := h.cardSvc.Create(r.Context(), req.MeetingID, req.ColumnID)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "meeting or column not found")
			return
		}
		if errors.Is(err, repository.ErrDuplicate) {
			writeError(w, http.StatusConflict, "meeting already on board")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create card")
		return
	}
	writeJSON(w, http.StatusCreated, card)
}

func (h *BoardHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := h.cardSvc.GetDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get card")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *BoardHandler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	card, err := h.cardSvc.UpdateDescription(r.Context(), id, req.Description)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update card")
		return
	}
	writeJSON(w, http.StatusOK, card)
}

func (h *BoardHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.cardSvc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete card")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BoardHandler) MoveCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		ColumnID string  `json:"column_id"`
		Position float64 `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.cardSvc.Move(r.Context(), id, req.ColumnID, req.Position); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "card or column not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to move card")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
