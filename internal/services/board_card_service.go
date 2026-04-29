package services

import (
	"context"
	"errors"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type BoardCardService struct {
	cardRepo     *repository.BoardCardRepository
	columnRepo   *repository.BoardColumnRepository
	meetingRepo  *repository.MeetingRepository
	summaryRepo  *repository.SummaryRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
}

func NewBoardCardService(
	cardRepo *repository.BoardCardRepository,
	columnRepo *repository.BoardColumnRepository,
	meetingRepo *repository.MeetingRepository,
	summaryRepo *repository.SummaryRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
) *BoardCardService {
	return &BoardCardService{cardRepo, columnRepo, meetingRepo, summaryRepo, keyPointRepo, taskRepo}
}

func (s *BoardCardService) List(ctx context.Context, f repository.BoardCardFilters) ([]models.BoardCardSummary, error) {
	cards, err := s.cardRepo.List(ctx, f)
	if err != nil {
		return nil, err
	}
	if cards == nil {
		return []models.BoardCardSummary{}, nil
	}
	return cards, nil
}

func (s *BoardCardService) Create(ctx context.Context, meetingID, columnID string) (*models.BoardCard, error) {
	if _, err := s.meetingRepo.GetByID(ctx, meetingID); err != nil {
		return nil, err
	}
	if columnID == "" {
		cols, err := s.columnRepo.List(ctx)
		if err != nil {
			return nil, err
		}
		if len(cols) == 0 {
			return nil, &ValidationError{Message: "no columns exist; create a column first"}
		}
		columnID = cols[0].ID
	} else {
		if _, err := s.columnRepo.GetByID(ctx, columnID); err != nil {
			return nil, err
		}
	}

	description := ""
	if sum, err := s.summaryRepo.GetByMeetingID(ctx, meetingID); err == nil {
		description = sum.Content
	}

	lastPos, err := s.cardRepo.LastPositionInColumn(ctx, columnID)
	if err != nil {
		return nil, err
	}
	return s.cardRepo.Create(ctx, meetingID, columnID, description, lastPos+1000)
}

func (s *BoardCardService) GetDetail(ctx context.Context, id string) (*models.BoardCardDetail, error) {
	detail, err := s.cardRepo.GetDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if sum, err := s.summaryRepo.GetByMeetingID(ctx, detail.MeetingID); err == nil {
		detail.Summary = sum
	}
	kps, err := s.keyPointRepo.ListByMeetingID(ctx, detail.MeetingID)
	if err == nil {
		detail.KeyPoints = kps
	}
	tasks, err := s.taskRepo.ListByMeetingID(ctx, detail.MeetingID)
	if err == nil {
		detail.Tasks = tasks
	}
	if detail.KeyPoints == nil {
		detail.KeyPoints = []models.KeyPoint{}
	}
	if detail.Tasks == nil {
		detail.Tasks = []models.Task{}
	}
	return detail, nil
}

func (s *BoardCardService) UpdateDescription(ctx context.Context, id, description string) (*models.BoardCard, error) {
	if err := s.cardRepo.UpdateDescription(ctx, id, description); err != nil {
		return nil, err
	}
	return s.cardRepo.GetByID(ctx, id)
}

func (s *BoardCardService) Move(ctx context.Context, id, columnID string, position float64) error {
	if _, err := s.columnRepo.GetByID(ctx, columnID); err != nil {
		return err
	}
	return s.cardRepo.Move(ctx, id, columnID, position)
}

func (s *BoardCardService) Delete(ctx context.Context, id string) error {
	return s.cardRepo.Delete(ctx, id)
}

func (s *BoardCardService) GetByMeetingID(ctx context.Context, meetingID string) (*models.BoardCard, error) {
	card, err := s.cardRepo.GetByMeetingID(ctx, meetingID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	return card, err
}
