package repository_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func openTestDB(t *testing.T) *repository.ThemeRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewThemeRepository(db)
}

func TestThemeRepository_CreateAndGetByID(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	theme := &models.Theme{
		ID:          "id-001",
		Name:        "Engenharia",
		Description: "Reuniões de eng",
		Color:       "#3b82f6",
	}
	if err := repo.Create(ctx, theme); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "id-001")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Engenharia" {
		t.Errorf("Name = %q, want %q", got.Name, "Engenharia")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestThemeRepository_Create_DuplicateName(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	theme := &models.Theme{ID: "id-001", Name: "Dup", Color: "#fff"}
	if err := repo.Create(ctx, theme); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	theme2 := &models.Theme{ID: "id-002", Name: "Dup", Color: "#000"}
	err := repo.Create(ctx, theme2)
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

func TestThemeRepository_List(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	themes, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 0 {
		t.Errorf("expected 0 themes, got %d", len(themes))
	}

	repo.Create(ctx, &models.Theme{ID: "a", Name: "Zeta", Color: "#fff"})
	repo.Create(ctx, &models.Theme{ID: "b", Name: "Alpha", Color: "#000"})

	themes, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("List after inserts: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("expected 2 themes, got %d", len(themes))
	}
	if themes[0].Name != "Alpha" {
		t.Errorf("expected sorted by name, first = %q", themes[0].Name)
	}
}

func TestThemeRepository_Update(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	repo.Create(ctx, &models.Theme{ID: "id-001", Name: "Original", Color: "#fff"})

	got, _ := repo.GetByID(ctx, "id-001")
	got.Name = "Atualizado"
	got.Color = "#000"

	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, _ := repo.GetByID(ctx, "id-001")
	if updated.Name != "Atualizado" {
		t.Errorf("Name = %q, want %q", updated.Name, "Atualizado")
	}
}

func TestThemeRepository_Update_NotFound(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	err := repo.Update(ctx, &models.Theme{ID: "nope", Name: "X", Color: "#fff"})
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestThemeRepository_Delete(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	repo.Create(ctx, &models.Theme{ID: "id-001", Name: "Para deletar", Color: "#fff"})
	if err := repo.Delete(ctx, "id-001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := repo.GetByID(ctx, "id-001")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestThemeRepository_Delete_NotFound(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestThemeRepository_GetByID_NotFound(t *testing.T) {
	repo := openTestDB(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
