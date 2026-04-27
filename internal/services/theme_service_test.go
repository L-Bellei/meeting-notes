package services_test

import (
	"context"
	"errors"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newTestService(t *testing.T) *services.ThemeService {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return services.NewThemeService(repository.NewThemeRepository(db))
}

func TestThemeService_Create(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	theme, err := svc.Create(ctx, "Produto", "Reuniões de produto", "#8b5cf6")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if theme.ID == "" {
		t.Error("ID should be set")
	}
	if theme.Name != "Produto" {
		t.Errorf("Name = %q", theme.Name)
	}
	if theme.Color != "#8b5cf6" {
		t.Errorf("Color = %q", theme.Color)
	}
	if theme.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestThemeService_Create_DefaultColor(t *testing.T) {
	svc := newTestService(t)

	theme, err := svc.Create(context.Background(), "Sem cor", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if theme.Color != "#6366f1" {
		t.Errorf("default color = %q, want %q", theme.Color, "#6366f1")
	}
}

func TestThemeService_Create_NameRequired(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Create(context.Background(), "", "", "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Create_DuplicateName(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Create(ctx, "Dup", "", "")
	_, err := svc.Create(ctx, "Dup", "", "")
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("expected ErrDuplicate, got %v", err)
	}
}

func TestThemeService_GetByID(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Eng", "", "")
	got, err := svc.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch")
	}
}

func TestThemeService_GetByID_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.GetByID(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestThemeService_Update(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Original", "", "")
	updated, err := svc.Update(ctx, created.ID, "Novo Nome", "nova desc", "#ff0000")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Novo Nome" {
		t.Errorf("Name = %q", updated.Name)
	}
	if updated.Color != "#ff0000" {
		t.Errorf("Color = %q", updated.Color)
	}
}

func TestThemeService_Update_NameRequired(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Original", "", "")
	_, err := svc.Update(ctx, created.ID, "", "", "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestThemeService_Delete(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	created, _ := svc.Create(ctx, "Para deletar", "", "")
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := svc.GetByID(ctx, created.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestThemeService_List(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	svc.Create(ctx, "B", "", "")
	svc.Create(ctx, "A", "", "")

	themes, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(themes) != 2 {
		t.Fatalf("expected 2, got %d", len(themes))
	}
	if themes[0].Name != "A" {
		t.Errorf("expected sorted, got %q first", themes[0].Name)
	}
}
