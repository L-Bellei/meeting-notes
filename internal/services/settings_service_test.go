package services_test

import (
	"context"
	"errors"
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

func TestSettingsService_Update_ClaudeCodeTokenAccepted(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"claude_code_token": "sk-ant-oat-abc123"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := svc.GetAll(context.Background())
	if m["claude_code_token"] != "sk-ant-oat-abc123" {
		t.Errorf("claude_code_token = %q, want sk-ant-oat-abc123", m["claude_code_token"])
	}
}

func TestSettingsService_Update_AnthropicApiKeyRejected(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"anthropic_api_key": "sk-ant-123"})
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}

func TestSettingsService_Update_ClaudeCodeModel_ValidValue(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"claude_code_model": "opus"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := svc.GetAll(context.Background())
	if m["claude_code_model"] != "opus" {
		t.Errorf("claude_code_model = %q, want opus", m["claude_code_model"])
	}
}

func TestSettingsService_Update_ClaudeCodeModel_FreeTextAccepted(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"claude_code_model": "claude-sonnet-4-6"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := svc.GetAll(context.Background())
	if m["claude_code_model"] != "claude-sonnet-4-6" {
		t.Errorf("claude_code_model = %q, want claude-sonnet-4-6", m["claude_code_model"])
	}
}

func TestSettingsService_Update_InvalidWhisperModel(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"whisper_model": "huge"})
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}

func TestSettingsService_Update_UnknownKeyRejected(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"unknown_key": "value"})
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}

func TestSettingsService_Update_SidebarPinned(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"sidebar_pinned": "true"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	m, _ := svc.GetAll(context.Background())
	if m["sidebar_pinned"] != "true" {
		t.Errorf("sidebar_pinned = %q, want true", m["sidebar_pinned"])
	}
}

func TestSettingsService_Update_InvalidSidebarPinned(t *testing.T) {
	svc := newSettingsSvc(t)
	err := svc.Update(context.Background(), map[string]string{"sidebar_pinned": "yes"})
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *services.ValidationError, got %T: %v", err, err)
	}
}
