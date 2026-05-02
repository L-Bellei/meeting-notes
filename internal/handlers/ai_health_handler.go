package handlers

import (
	"context"
	"net/http"
)

type AIHealthHandler struct {
	ping func(ctx context.Context) (configured bool, err error)
}

func NewAIHealthHandler(ping func(context.Context) (bool, error)) *AIHealthHandler {
	return &AIHealthHandler{ping: ping}
}

func (h *AIHealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	configured, err := h.ping(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"configured": true,
			"valid":      false,
			"error":      err.Error(),
		})
		return
	}
	if !configured {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"configured": true, "valid": true})
}
