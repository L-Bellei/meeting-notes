package models

import "time"

type Theme struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
}

type MeetingStatus string

const (
	StatusPending      MeetingStatus = "pending"
	StatusRecording    MeetingStatus = "recording"
	StatusTranscribing MeetingStatus = "transcribing"
	StatusProcessing   MeetingStatus = "processing"
	StatusCompleted    MeetingStatus = "completed"
	StatusFailed       MeetingStatus = "failed"
)

type Meeting struct {
	ID              string        `json:"id"`
	ThemeID         *string       `json:"theme_id"`
	Title           string        `json:"title"`
	StartedAt       *time.Time    `json:"started_at"`
	DurationSeconds *int          `json:"duration_seconds"`
	Status          MeetingStatus `json:"status"`
	Transcript      *string       `json:"transcript"`
	Notes           *string       `json:"notes"`
	CreatedAt       time.Time     `json:"created_at"`
}

type Summary struct {
	ID           string    `json:"id"`
	MeetingID    string    `json:"meeting_id"`
	Content      string    `json:"content"`
	ModelUsed    string    `json:"model_used"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CreatedAt    time.Time `json:"created_at"`
}

type KeyPoint struct {
	ID        string `json:"id"`
	MeetingID string `json:"meeting_id"`
	Position  int    `json:"position"`
	Content   string `json:"content"`
}

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

type Task struct {
	ID          string       `json:"id"`
	MeetingID   string       `json:"meeting_id"`
	Description string       `json:"description"`
	Assignee    *string      `json:"assignee"`
	DueDate     *time.Time   `json:"due_date"`
	Priority    TaskPriority `json:"priority"`
	Completed   bool         `json:"completed"`
	CreatedAt   time.Time    `json:"created_at"`
}
