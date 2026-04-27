package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

var validMeetingStatuses = map[string]bool{
	string(models.StatusPending):      true,
	string(models.StatusRecording):    true,
	string(models.StatusTranscribing): true,
	string(models.StatusProcessing):   true,
	string(models.StatusCompleted):    true,
	string(models.StatusFailed):       true,
}

type MeetingService struct {
	repo *repository.MeetingRepository
}

func NewMeetingService(repo *repository.MeetingRepository) *MeetingService {
	return &MeetingService{repo: repo}
}

func (s *MeetingService) List(ctx context.Context, themeID, status string) ([]models.Meeting, error) {
	return s.repo.List(ctx, themeID, status)
}

func (s *MeetingService) Create(ctx context.Context, title, themeID, status string, startedAt *time.Time) (*models.Meeting, error) {
	if title == "" {
		return nil, &ValidationError{"title is required"}
	}
	if status == "" {
		status = string(models.StatusPending)
	} else if !validMeetingStatuses[status] {
		return nil, &ValidationError{"invalid status: must be one of pending, recording, transcribing, processing, completed, failed"}
	}
	if startedAt == nil {
		now := time.Now().UTC()
		startedAt = &now
	}
	var themeIDPtr *string
	if themeID != "" {
		themeIDPtr = &themeID
	}
	m := &models.Meeting{
		ID:        uuid.New().String(),
		ThemeID:   themeIDPtr,
		Title:     title,
		StartedAt: startedAt,
		Status:    models.MeetingStatus(status),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MeetingService) GetByID(ctx context.Context, id string) (*models.Meeting, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MeetingService) Update(ctx context.Context, id, title string, themeID *string, status string, startedAt *time.Time, durationSeconds *int, transcript *string) (*models.Meeting, error) {
	if title == "" {
		return nil, &ValidationError{"title is required"}
	}
	if status != "" && !validMeetingStatuses[status] {
		return nil, &ValidationError{"invalid status: must be one of pending, recording, transcribing, processing, completed, failed"}
	}
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.Title = title
	if themeID != nil {
		if *themeID == "" {
			m.ThemeID = nil
		} else {
			m.ThemeID = themeID
		}
	}
	if status != "" {
		m.Status = models.MeetingStatus(status)
	}
	if startedAt != nil {
		m.StartedAt = startedAt
	}
	if durationSeconds != nil {
		m.DurationSeconds = durationSeconds
	}
	if transcript != nil {
		m.Transcript = transcript
	}
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *MeetingService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
