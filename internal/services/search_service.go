package services

import (
	"context"
	"fmt"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type SearchResultItem struct {
	MeetingID    string  `json:"meeting_id"`
	MeetingTitle string  `json:"meeting_title"`
	Snippet      string  `json:"snippet"`
	StartedAt    *string `json:"started_at"`
	Status       string  `json:"status"`
}

type SearchService struct {
	searchRepo  *repository.SearchRepository
	meetingRepo *repository.MeetingRepository
}

func NewSearchService(searchRepo *repository.SearchRepository, meetingRepo *repository.MeetingRepository) *SearchService {
	return &SearchService{searchRepo: searchRepo, meetingRepo: meetingRepo}
}

func (s *SearchService) Search(ctx context.Context, q string) ([]SearchResultItem, error) {
	if len(q) < 2 {
		return nil, &ValidationError{"query must be at least 2 characters"}
	}

	raw, err := s.searchRepo.Search(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(raw) == 0 {
		return []SearchResultItem{}, nil
	}

	meetings, err := s.meetingRepo.List(ctx, repository.ListFilters{})
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	byID := make(map[string]*models.Meeting, len(meetings))
	for i := range meetings {
		byID[meetings[i].ID] = &meetings[i]
	}

	items := make([]SearchResultItem, 0, len(raw))
	for _, r := range raw {
		m, ok := byID[r.MeetingID]
		if !ok {
			continue
		}
		item := SearchResultItem{
			MeetingID:    r.MeetingID,
			MeetingTitle: m.Title,
			Snippet:      r.Snippet,
			Status:       string(m.Status),
		}
		if m.StartedAt != nil {
			formatted := m.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			item.StartedAt = &formatted
		}
		items = append(items, item)
	}
	return items, nil
}
