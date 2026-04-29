package services

import (
	"context"
	"fmt"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type ColumnHasCardsError struct {
	Count int
}

func (e *ColumnHasCardsError) Error() string {
	return fmt.Sprintf("column has %d cards", e.Count)
}

type BoardColumnService struct {
	repo *repository.BoardColumnRepository
}

func NewBoardColumnService(repo *repository.BoardColumnRepository) *BoardColumnService {
	return &BoardColumnService{repo: repo}
}

func (s *BoardColumnService) List(ctx context.Context) ([]models.BoardColumnWithCount, error) {
	return s.repo.List(ctx)
}

func (s *BoardColumnService) Create(ctx context.Context, name string) (*models.BoardColumn, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	return s.repo.Create(ctx, name)
}

func (s *BoardColumnService) Update(ctx context.Context, id, name string) (*models.BoardColumn, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if err := s.repo.Update(ctx, id, name); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *BoardColumnService) Delete(ctx context.Context, id, moveTo string) error {
	count, err := s.repo.Count(ctx)
	if err != nil {
		return err
	}
	if count <= 1 {
		return &ValidationError{"cannot delete the last column"}
	}
	cardCount, err := s.repo.CardCount(ctx, id)
	if err != nil {
		return err
	}
	if cardCount > 0 && moveTo == "" {
		return &ColumnHasCardsError{Count: cardCount}
	}
	if cardCount > 0 {
		return s.repo.DeleteWithMove(ctx, id, moveTo)
	}
	return s.repo.Delete(ctx, id)
}

func (s *BoardColumnService) Reorder(ctx context.Context, items []repository.ReorderItem) error {
	return s.repo.Reorder(ctx, items)
}
