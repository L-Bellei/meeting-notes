package services_test

import (
	"context"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func newSettingsSvc(t *testing.T) *services.SettingsService {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return services.NewSettingsService(repository.NewSettingsRepository(db))
}

func TestSettingsService_GetAll_ReturnsMap(t *testing.T) {
	svc := newSettingsSvc(t)
	m, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if _, ok := m["ai_provider"]; !ok {
		t.Error("expected ai_provider key")
	}
}

func TestSettingsService_Update_ValidProvider(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"ai_provider": "openai"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := svc.GetAll(context.Background())
	if m["ai_provider"] != "openai" {
		t.Errorf("ai_provider = %q, want openai", m["ai_provider"])
	}
}

func TestSettingsService_Update_InvalidProvider(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"ai_provider": "gemini"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestSettingsService_Update_InvalidWhisperModel(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"whisper_model": "huge"})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestSettingsService_Update_UnknownKeyRejected(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"unknown_key": "value"})
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}
