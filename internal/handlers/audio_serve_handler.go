package handlers

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"meeting-notes/internal/repository"
)

type AudioServeHandler struct {
	meetingRepo *repository.MeetingRepository
}

func NewAudioServeHandler(repo *repository.MeetingRepository) *AudioServeHandler {
	return &AudioServeHandler{meetingRepo: repo}
}

func (h *AudioServeHandler) ServeAudio(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.meetingRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, "meeting not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if m.AudioPath == nil || *m.AudioPath == "" {
		http.Error(w, "no audio file", http.StatusNotFound)
		return
	}
	f, err := os.Open(*m.AudioPath)
	if err != nil {
		http.Error(w, "audio file not found on disk", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "audio.wav", info.ModTime(), f)
}
