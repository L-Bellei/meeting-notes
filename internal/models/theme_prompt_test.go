package models_test

import (
	"testing"

	"meeting-notes/internal/models"
)

func TestTheme_PromptFor(t *testing.T) {
	theme := &models.Theme{
		CustomPrompt:          "GERAL",
		CustomSummaryPrompt:   "RESUMO",
		CustomKeyPointsPrompt: "",
		CustomTasksPrompt:     "TAREFAS",
	}

	if got := theme.PromptFor(models.PromptSummary); got != "RESUMO" {
		t.Errorf("summary: got %q, want specific 'RESUMO'", got)
	}
	if got := theme.PromptFor(models.PromptKeyPoints); got != "GERAL" {
		t.Errorf("key points: got %q, want fallback to general 'GERAL'", got)
	}
	if got := theme.PromptFor(models.PromptTasks); got != "TAREFAS" {
		t.Errorf("tasks: got %q, want specific 'TAREFAS'", got)
	}

	empty := &models.Theme{}
	if got := empty.PromptFor(models.PromptSummary); got != "" {
		t.Errorf("all-empty: got %q, want \"\" (AI client falls back to default)", got)
	}
}
