package repository_test

import (
	"context"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
)

func newSettingsRepo(t *testing.T) *repository.SettingsRepository {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewSettingsRepository(db)
}

func TestSettingsRepo_GetAll_ReturnsDefaults(t *testing.T) {
	repo := newSettingsRepo(t)
	settings, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if settings["ai_provider"] != "anthropic" {
		t.Errorf("ai_provider = %q, want anthropic", settings["ai_provider"])
	}
	if settings["auto_generate"] != "true" {
		t.Errorf("auto_generate = %q, want true", settings["auto_generate"])
	}
}

func TestSettingsRepo_Set_UpdatesValue(t *testing.T) {
	repo := newSettingsRepo(t)
	if err := repo.Set(context.Background(), "user_name", "Leonardo"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	settings, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if settings["user_name"] != "Leonardo" {
		t.Errorf("user_name = %q, want Leonardo", settings["user_name"])
	}
}

func TestSettingsRepo_Set_InsertsNewKey(t *testing.T) {
	repo := newSettingsRepo(t)
	if err := repo.Set(context.Background(), "user_name", "Ana"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	settings, _ := repo.GetAll(context.Background())
	if settings["user_name"] != "Ana" {
		t.Errorf("user_name = %q, want Ana", settings["user_name"])
	}
}
