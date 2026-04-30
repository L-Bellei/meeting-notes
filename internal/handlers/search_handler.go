package handlers

import (
	"errors"
	"net/http"

	"meeting-notes/internal/services"
)

type SearchHandler struct {
	svc *services.SearchService
}

func NewSearchHandler(svc *services.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 2 {
		writeError(w, http.StatusBadRequest, "q must be at least 2 characters")
		return
	}
	results, err := h.svc.Search(r.Context(), q)
	if err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusBadRequest, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	writeJSON(w, http.StatusOK, results)
}
