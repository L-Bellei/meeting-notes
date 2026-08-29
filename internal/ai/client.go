package ai

import (
	"context"
	"strings"
)

type TaskSuggestion struct {
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	Priority    string `json:"priority"`
}

type AIClient interface {
	GenerateSummary(ctx context.Context, transcript, notes, customPrompt string) (content string, inputTokens, outputTokens int, err error)
	GenerateKeyPoints(ctx context.Context, transcript, notes, customPrompt string) (points []string, inputTokens, outputTokens int, err error)
	GenerateTasks(ctx context.Context, transcript, notes, customPrompt string) (tasks []TaskSuggestion, inputTokens, outputTokens int, err error)
}

func buildInstruction(defaultInstruction, customPrompt string) string {
	if customPrompt != "" {
		return customPrompt
	}
	return defaultInstruction
}

func buildContext(transcript, notes string) string {
	if notes == "" {
		return transcript
	}
	return "Transcript:\n" + transcript + "\n\nMeeting Notes (added by the user):\n" + notes
}

// stripJSONFence removes leading/trailing whitespace and ```json fences if present.
func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}
