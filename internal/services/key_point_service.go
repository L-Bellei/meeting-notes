package services

import (
	"context"

	"github.com/google/uuid"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type KeyPointService struct {
	repo *repository.KeyPointRepository
	ai   ai.AIClient
}

func NewKeyPointService(repo *repository.KeyPointRepository, aiClient ai.AIClient) *KeyPointService {
	return &KeyPointService{repo: repo, ai: aiClient}
}

func (s *KeyPointService) List(ctx context.Context, meetingID string) ([]models.KeyPoint, error) {
	return s.repo.ListByMeetingID(ctx, meetingID)
}

func (s *KeyPointService) Create(ctx context.Context, meetingID, content string, position int) (*models.KeyPoint, error) {
	if content == "" {
		return nil, &ValidationError{"content is required"}
	}
	kp := &models.KeyPoint{
		ID:        uuid.New().String(),
		MeetingID: meetingID,
		Position:  position,
		Content:   content,
	}
	if err := s.repo.Create(ctx, kp); err != nil {
		return nil, err
	}
	return kp, nil
}

func (s *KeyPointService) Update(ctx context.Context, id, content string, position int) (*models.KeyPoint, error) {
	if content == "" {
		return nil, &ValidationError{"content is required"}
	}
	kp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	kp.Content = content
	kp.Position = position
	if err := s.repo.Update(ctx, kp); err != nil {
		return nil, err
	}
	return kp, nil
}

func (s *KeyPointService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *KeyPointService) Generate(ctx context.Context, meeting *models.Meeting, customPrompt string) ([]models.KeyPoint, error) {
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
	points, _, _, err := s.ai.GenerateKeyPoints(ctx, *meeting.Transcript, notes, customPrompt)
	if err != nil {
		return nil, mapAIError(err)
	}
	if err := s.repo.DeleteByMeetingID(ctx, meeting.ID); err != nil {
		return nil, err
	}
	created := make([]models.KeyPoint, 0, len(points))
	for i, content := range points {
		kp := models.KeyPoint{
			ID:        uuid.New().String(),
			MeetingID: meeting.ID,
			Position:  i,
			Content:   content,
		}
		if err := s.repo.Create(ctx, &kp); err != nil {
			return nil, err
		}
		created = append(created, kp)
	}
	return created, nil
}
