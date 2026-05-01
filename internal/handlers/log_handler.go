package handlers

import (
	"net/http"
	"strconv"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type LogHandler struct{ repo *repository.LogRepository }

func NewLogHandler(repo *repository.LogRepository) *LogHandler { return &LogHandler{repo: repo} }

func (h *LogHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	logs, err := h.repo.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list logs")
		return
	}
	if logs == nil {
		logs = []models.AppLog{}
	}
	writeJSON(w, http.StatusOK, logs)
}
