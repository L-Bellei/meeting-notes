package services_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"meeting-notes/internal/audio"
	"meeting-notes/internal/services"
)

// stub audio client — implement ALL methods of audio.Client interface
type stubAudioClient struct {
	healthResp *audio.HealthResponse
	healthErr  error
}

func (s *stubAudioClient) Health(ctx context.Context) (*audio.HealthResponse, error) {
	return s.healthResp, s.healthErr
}
func (s *stubAudioClient) StartRecording(ctx context.Context) (*audio.StartResponse, error) { return nil, nil }
func (s *stubAudioClient) StopRecording(ctx context.Context) (*audio.StopResponse, error)   { return nil, nil }
func (s *stubAudioClient) Transcribe(ctx context.Context, path, lang string) (*audio.TranscribeResponse, error) {
	return nil, nil
}

func TestCheckModelLoaded_ServiceUnavailable(t *testing.T) {
	client := &stubAudioClient{healthErr: errors.New("connection refused")}
	err := services.CheckModelLoaded(context.Background(), client)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestCheckModelLoaded_NotReady(t *testing.T) {
	client := &stubAudioClient{healthResp: &audio.HealthResponse{ModelLoaded: false}}
	err := services.CheckModelLoaded(context.Background(), client)
	if err == nil {
		t.Fatal("want error when model not loaded")
	}
}

func TestCheckModelLoaded_Ready(t *testing.T) {
	client := &stubAudioClient{healthResp: &audio.HealthResponse{ModelLoaded: true}}
	err := services.CheckModelLoaded(context.Background(), client)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestValidateWAVFile_NotExist(t *testing.T) {
	err := services.ValidateWAVFile("/nonexistent/path.wav")
	if err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestValidateWAVFile_TooSmall(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "tiny.wav")
	if err := os.WriteFile(f, make([]byte, 100), 0644); err != nil {
		t.Fatal(err)
	}
	err := services.ValidateWAVFile(f)
	if err == nil {
		t.Fatal("want error for file < 10KB")
	}
}

func TestValidateWAVFile_Valid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "audio.wav")
	if err := os.WriteFile(f, make([]byte, 20*1024), 0644); err != nil {
		t.Fatal(err)
	}
	err := services.ValidateWAVFile(f)
	if err != nil {
		t.Fatalf("want nil for valid file, got %v", err)
	}
}
