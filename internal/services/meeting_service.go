package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type MeetingFilters struct {
	ThemeID       string // single theme; sub-themes are resolved automatically
	Status        string
	Q             string
	StartedAfter  string
	StartedBefore string
}

var validMeetingStatuses = map[string]bool{
	string(models.StatusPending):      true,
	string(models.StatusRecording):    true,
	string(models.StatusTranscribing): true,
	string(models.StatusProcessing):   true,
	string(models.StatusCompleted):    true,
	string(models.StatusFailed):       true,
}

type MeetingService struct {
	repo         *repository.MeetingRepository
	themeRepo    *repository.ThemeRepository
	searchRepo   *repository.SearchRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
	summaryRepo  *repository.SummaryRepository
}

func NewMeetingService(
	repo *repository.MeetingRepository,
	themeRepo *repository.ThemeRepository,
	searchRepo *repository.SearchRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
	summaryRepo *repository.SummaryRepository,
) *MeetingService {
	return &MeetingService{repo, themeRepo, searchRepo, keyPointRepo, taskRepo, summaryRepo}
}

func (s *MeetingService) List(ctx context.Context, f MeetingFilters) ([]models.Meeting, error) {
	rf := repository.ListFilters{
		Status:        f.Status,
		Q:             f.Q,
		StartedAfter:  f.StartedAfter,
		StartedBefore: f.StartedBefore,
	}
	if f.ThemeID != "" {
		rf.ThemeIDs = []string{f.ThemeID}
		// include direct children
		children, err := s.themeRepo.ChildIDs(ctx, f.ThemeID)
		if err != nil {
			return nil, err
		}
		rf.ThemeIDs = append(rf.ThemeIDs, children...)
	}
	return s.repo.List(ctx, rf)
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

func (s *MeetingService) Update(ctx context.Context, id, title string, themeID *string, status string, startedAt *time.Time, durationSeconds *int, transcript *string, notes *string) (*models.Meeting, error) {
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
	if notes != nil {
		m.Notes = notes
	}
	if err := s.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	go func() {
		bgCtx := context.Background()
		transcript := ""
		if m.Transcript != nil {
			transcript = *m.Transcript
		}
		summary := ""
		if sm, err2 := s.summaryRepo.GetByMeetingID(bgCtx, m.ID); err2 == nil && sm != nil {
			summary = sm.Content
		}
		kps, _ := s.keyPointRepo.ListByMeetingID(bgCtx, m.ID)
		var kpContents []string
		for _, kp := range kps {
			kpContents = append(kpContents, kp.Content)
		}
		tasks, _ := s.taskRepo.ListByMeetingID(bgCtx, m.ID)
		var taskContents []string
		for _, tk := range tasks {
			taskContents = append(taskContents, tk.Description)
		}
		_ = s.searchRepo.UpsertMeeting(bgCtx, m.ID, m.Title, transcript, summary,
			strings.Join(kpContents, "\n"), strings.Join(taskContents, "\n"))
	}()
	return m, nil
}

func (s *MeetingService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	go func() {
		_ = s.searchRepo.DeleteMeeting(context.Background(), id)
	}()
	return nil
}
