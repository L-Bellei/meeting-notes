package handlers

import (
	"context"
	"errors"
	"net/http"

	"meeting-notes/internal/ai"
)

type ClaudeLoginHandler struct {
	launch func() error
}

func NewClaudeLoginHandler(launch func() error) *ClaudeLoginHandler {
	return &ClaudeLoginHandler{launch: launch}
}

func (h *ClaudeLoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := h.launch(); err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "launched"})
}

type AITestHandler struct {
	test func(ctx context.Context) error
}

func NewAITestHandler(test func(ctx context.Context) error) *AITestHandler {
	return &AITestHandler{test: test}
}

func (h *AITestHandler) Test(w http.ResponseWriter, r *http.Request) {
	err := h.test(r.Context())
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	if ai.IsAuthError(err) {
		writeError(w, http.StatusBadGateway, "token inválido ou expirado — reconecte com Claude")
		return
	}
	writeError(w, http.StatusBadGateway, err.Error())
}
