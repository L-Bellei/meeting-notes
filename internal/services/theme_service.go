package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type ThemeService struct {
	repo *repository.ThemeRepository
}

func NewThemeService(repo *repository.ThemeRepository) *ThemeService {
	return &ThemeService{repo: repo}
}

func (s *ThemeService) List(ctx context.Context) ([]models.Theme, error) {
	return s.repo.List(ctx)
}

func (s *ThemeService) Create(ctx context.Context, name, description, color string, parentID *string, customPrompt string, autoAddToBoard bool) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	if color == "" {
		color = "#6366f1"
	}
	if err := s.validateParent(ctx, "", parentID); err != nil {
		return nil, err
	}
	t := &models.Theme{
		ID:             uuid.New().String(),
		ParentID:       parentID,
		Name:           name,
		Description:    description,
		Color:          color,
		CustomPrompt:   customPrompt,
		AutoAddToBoard: autoAddToBoard,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ThemeService) GetByID(ctx context.Context, id string) (*models.Theme, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ThemeService) Update(ctx context.Context, id, name, description, color string, parentID *string, customPrompt string, autoAddToBoard bool) (*models.Theme, error) {
	if name == "" {
		return nil, &ValidationError{"name is required"}
	}
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.validateParent(ctx, id, parentID); err != nil {
		return nil, err
	}
	t.Name = name
	t.Description = description
	if color != "" {
		t.Color = color
	}
	t.ParentID = parentID
	t.CustomPrompt = customPrompt
	t.AutoAddToBoard = autoAddToBoard
	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *ThemeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *ThemeService) validateParent(ctx context.Context, id string, parentID *string) error {
	if parentID == nil {
		return nil
	}
	if *parentID == id {
		return &ValidationError{"theme cannot be its own parent"}
	}
	parent, err := s.repo.GetByID(ctx, *parentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &ValidationError{"parent theme not found"}
		}
		return err
	}
	if parent.ParentID != nil {
		return &ValidationError{"parent theme cannot be a subcategory"}
	}
	if id != "" {
		children, err := s.repo.ChildIDs(ctx, id)
		if err != nil {
			return err
		}
		if len(children) > 0 {
			return &ValidationError{"theme with subcategories cannot become a subcategory"}
		}
	}
	return nil
}
