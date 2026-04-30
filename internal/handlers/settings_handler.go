package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"meeting-notes/internal/services"
)

type SettingsHandler struct {
	svc      *services.SettingsService
	onUpdate func(map[string]string)
}

func NewSettingsHandler(svc *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

func (h *SettingsHandler) SetOnUpdate(fn func(map[string]string)) {
	h.onUpdate = fn
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Update(r.Context(), updates); err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeError(w, http.StatusUnprocessableEntity, ve.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update settings")
		return
	}
	settings, err := h.svc.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read updated settings")
		return
	}
	if h.onUpdate != nil {
		h.onUpdate(settings)
	}
	writeJSON(w, http.StatusOK, settings)
}
