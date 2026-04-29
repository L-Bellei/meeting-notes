package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

var ErrAINotConfigured = errors.New("AI client not configured")

type SummaryService struct {
	repo *repository.SummaryRepository
	ai   ai.AIClient
}

func NewSummaryService(repo *repository.SummaryRepository, aiClient ai.AIClient) *SummaryService {
	return &SummaryService{repo: repo, ai: aiClient}
}

func (s *SummaryService) Get(ctx context.Context, meetingID string) (*models.Summary, error) {
	return s.repo.GetByMeetingID(ctx, meetingID)
}

func (s *SummaryService) Upsert(ctx context.Context, meetingID, content, modelUsed string) (*models.Summary, error) {
	if content == "" {
		return nil, &ValidationError{"content is required"}
	}
	summary := &models.Summary{
		ID:        uuid.New().String(),
		MeetingID: meetingID,
		Content:   content,
		ModelUsed: modelUsed,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Upsert(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *SummaryService) Delete(ctx context.Context, meetingID string) error {
	return s.repo.Delete(ctx, meetingID)
}

func (s *SummaryService) Generate(ctx context.Context, meeting *models.Meeting, customPrompt string) (*models.Summary, error) {
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
	content, inputTokens, outputTokens, err := s.ai.GenerateSummary(ctx, *meeting.Transcript, notes, customPrompt)
	if err != nil {
		return nil, err
	}
	summary := &models.Summary{
		ID:           uuid.New().String(),
		MeetingID:    meeting.ID,
		Content:      content,
		ModelUsed:    "anthropic",
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Upsert(ctx, summary); err != nil {
		return nil, err
	}
	return summary, nil
}
