package services

import (
	"context"
	"os"
	"testing"

	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

func TestClearAudioReferenceAndRemove_PreservesFileWhenDatabaseUpdateFails(t *testing.T) {
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := repository.NewMeetingRepository(db)
	meeting := &models.Meeting{ID: "m-1", Title: "Meeting", Status: models.StatusProcessing}
	if err := repo.Create(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}
	audioPath := t.TempDir() + "/recording.wav"
	if err := os.WriteFile(audioPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAudioPath(context.Background(), meeting.ID, &audioPath, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_audio_clear BEFORE UPDATE OF audio_path ON meetings
		WHEN NEW.audio_path IS NULL BEGIN SELECT RAISE(ABORT, 'audio clear blocked'); END`); err != nil {
		t.Fatal(err)
	}

	orchestrator := &Orchestrator{repo: repo, operations: NewMeetingOperationCoordinator()}
	if err := orchestrator.clearAudioReferenceAndRemove(context.Background(), meeting, audioPath); err == nil {
		t.Fatal("expected audio reference update to fail")
	}
	if _, err := os.Stat(audioPath); err != nil {
		t.Fatalf("audio file was removed despite database update failure: %v", err)
	}
	got, err := repo.GetByID(context.Background(), meeting.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AudioPath == nil || *got.AudioPath != audioPath {
		t.Fatalf("database audio path = %v, want %q", got.AudioPath, audioPath)
	}
}
