package services

import (
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
