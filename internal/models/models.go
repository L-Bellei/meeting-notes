package models

import "time"

type Theme struct {
	ID             string    `json:"id"`
	ParentID       *string   `json:"parent_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Color          string    `json:"color"`
	CustomPrompt   string    `json:"custom_prompt"`
	AutoAddToBoard bool      `json:"auto_add_to_board"`
	CreatedAt      time.Time `json:"created_at"`
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
	AudioPath       *string       `json:"audio_path"`
	ErrorMessage    *string       `json:"error_message"`
	KeepAudio       bool          `json:"keep_audio"`
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

type BoardColumn struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Position  float64   `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

type BoardColumnWithCount struct {
	BoardColumn
	CardCount int `json:"card_count"`
}

type BoardCard struct {
	ID          string    `json:"id"`
	MeetingID   *string   `json:"meeting_id"`
	ColumnID    string    `json:"column_id"`
	Number      int       `json:"number"`
	Position    float64   `json:"position"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tasks       []string  `json:"tasks"`
	Source      string    `json:"source"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type AppLog struct {
	ID        string  `json:"id"`
	Level     string  `json:"level"`
	Component string  `json:"component"`
	Message   string  `json:"message"`
	Metadata  *string `json:"metadata,omitempty"`
	CreatedAt string  `json:"created_at"`
}

func (c *BoardCard) DisplayTitle(meeting *Meeting) string {
	if c.MeetingID != nil {
		if meeting != nil {
			return meeting.Title
		}
		return ""
	}
	return c.Title
}

type TaskProgress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
}

type BoardCardSummary struct {
	ID           string       `json:"id"`
	MeetingID    *string      `json:"meeting_id"`
	ColumnID     string       `json:"column_id"`
	Number       int          `json:"number"`
	Position     float64      `json:"position"`
	Description  string       `json:"description"`
	Source       string       `json:"source"`
	UpdatedAt    time.Time    `json:"updated_at"`
	CreatedAt    time.Time    `json:"created_at"`
	MeetingTitle string       `json:"meeting_title"`
	ThemeID      *string      `json:"theme_id"`
	ThemeName    *string      `json:"theme_name"`
	ThemeColor   *string      `json:"theme_color"`
	Status       string       `json:"status"`
	TaskProgress TaskProgress `json:"task_progress"`
}

type BoardCardDetail struct {
	ID           string     `json:"id"`
	MeetingID    *string    `json:"meeting_id"`
	ColumnID     string     `json:"column_id"`
	Number       int        `json:"number"`
	Position     float64    `json:"position"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Source       string     `json:"source"`
	ManualTasks  []string   `json:"manual_tasks"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CreatedAt    time.Time  `json:"created_at"`
	Status       string     `json:"status"`
	MeetingTitle string     `json:"meeting_title"`
	ThemeID      *string    `json:"theme_id"`
	ThemeName    *string    `json:"theme_name"`
	ThemeColor   *string    `json:"theme_color"`
	Summary      *Summary   `json:"summary"`
	KeyPoints    []KeyPoint `json:"key_points"`
	Tasks        []Task     `json:"tasks"`
}
