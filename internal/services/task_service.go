package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

var validTaskPriorities = map[string]bool{
	string(models.PriorityLow):    true,
	string(models.PriorityMedium): true,
	string(models.PriorityHigh):   true,
}

type TaskService struct {
	repo *repository.TaskRepository
	ai   ai.AIClient
}

func NewTaskService(repo *repository.TaskRepository, aiClient ai.AIClient) *TaskService {
	return &TaskService{repo: repo, ai: aiClient}
}

func (s *TaskService) List(ctx context.Context, meetingID string) ([]models.Task, error) {
	return s.repo.ListByMeetingID(ctx, meetingID)
}

func (s *TaskService) Create(ctx context.Context, meetingID, description string, assignee *string, dueDate *time.Time, priority string) (*models.Task, error) {
	if description == "" {
		return nil, &ValidationError{"description is required"}
	}
	if priority == "" {
		priority = string(models.PriorityMedium)
	} else if !validTaskPriorities[priority] {
		return nil, &ValidationError{"invalid priority: must be one of low, medium, high"}
	}
	task := &models.Task{
		ID:          uuid.New().String(),
		MeetingID:   meetingID,
		Description: description,
		Assignee:    assignee,
		DueDate:     dueDate,
		Priority:    models.TaskPriority(priority),
		Completed:   false,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskService) Update(ctx context.Context, id, description string, assignee *string, dueDate *time.Time, priority string, completed bool) (*models.Task, error) {
	if description == "" {
		return nil, &ValidationError{"description is required"}
	}
	if priority != "" && !validTaskPriorities[priority] {
		return nil, &ValidationError{"invalid priority: must be one of low, medium, high"}
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	task.Description = description
	task.Assignee = assignee
	task.DueDate = dueDate
	if priority != "" {
		task.Priority = models.TaskPriority(priority)
	}
	task.Completed = completed
	if err := s.repo.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *TaskService) Generate(ctx context.Context, meeting *models.Meeting, customPrompt string) ([]models.Task, error) {
	if s.ai == nil {
		return nil, ErrAINotConfigured
	}
	if meeting.Transcript == nil || *meeting.Transcript == "" {
		return nil, &ValidationError{"transcript is required for generation"}
	}
	notes := ""
	if meeting.Notes != nil {
		notes = *meeting.Notes
	}
	suggestions, _, _, err := s.ai.GenerateTasks(ctx, *meeting.Transcript, notes, customPrompt)
	if err != nil {
		return nil, mapAIError(err)
	}
	if err := s.repo.DeleteByMeetingID(ctx, meeting.ID); err != nil {
		return nil, err
	}
	created := make([]models.Task, 0, len(suggestions))
	for _, sug := range suggestions {
		priority := sug.Priority
		if priority == "" || !validTaskPriorities[priority] {
			priority = string(models.PriorityMedium)
		}
		var assignee *string
		if sug.Assignee != "" {
			a := sug.Assignee
			assignee = &a
		}
		task := models.Task{
			ID:          uuid.New().String(),
			MeetingID:   meeting.ID,
			Description: sug.Description,
			Assignee:    assignee,
			Priority:    models.TaskPriority(priority),
			Completed:   false,
			CreatedAt:   time.Now().UTC(),
		}
		if err := s.repo.Create(ctx, &task); err != nil {
			return nil, err
		}
		created = append(created, task)
	}
	return created, nil
}
