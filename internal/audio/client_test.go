package audio_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"meeting-notes/internal/audio"
)

func TestClient_Health_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %q, want /health", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "state": "idle", "loopback_available": true,
			"model_loaded": true, "model_name": "medium", "device": "cuda",
		})
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	got, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got.Status != "ok" || got.State != "idle" || !got.LoopbackAvailable || !got.ModelLoaded {
		t.Errorf("got = %+v", got)
	}
}

func TestClient_Health_NetworkError(t *testing.T) {
	c := audio.NewHTTPClient("http://127.0.0.1:1") // unreachable port
	_, err := c.Health(context.Background())
	if !errors.Is(err, audio.ErrAudioServiceUnavailable) {
		t.Errorf("expected ErrAudioServiceUnavailable, got %v", err)
	}
}

func TestClient_StartRecording_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"recording_id":"rec-1","started_at":"2026-04-28T12:00:00Z"}`))
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	got, err := c.StartRecording(context.Background())
	if err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if got.RecordingID != "rec-1" {
		t.Errorf("RecordingID = %q", got.RecordingID)
	}
	if got.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
}

func TestClient_StartRecording_409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"already recording"}`, http.StatusConflict)
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	_, err := c.StartRecording(context.Background())
	if !errors.Is(err, audio.ErrAudioServiceConflict) {
		t.Errorf("expected ErrAudioServiceConflict, got %v", err)
	}
}

func TestClient_StopRecording_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"recording_id":"rec-1","path":"tmp/rec-1.wav","duration_seconds":12.5,"size_bytes":400000,"partial":false}`))
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	got, err := c.StopRecording(context.Background())
	if err != nil {
		t.Fatalf("StopRecording: %v", err)
	}
	if got.Path != "tmp/rec-1.wav" || got.DurationSeconds != 12.5 {
		t.Errorf("got = %+v", got)
	}
}

func TestClient_Transcribe_OK(t *testing.T) {
	var receivedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcribe" {
			t.Errorf("path = %q, want /transcribe", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transcript":"olá mundo","language":"pt","duration_seconds":12.5,"model":"medium"}`))
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	got, err := c.Transcribe(context.Background(), "tmp/rec-1.wav", "pt")
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if got.Transcript != "olá mundo" {
		t.Errorf("Transcript = %q", got.Transcript)
	}
	if receivedBody["path"] != "tmp/rec-1.wav" || receivedBody["language"] != "pt" {
		t.Errorf("body = %+v", receivedBody)
	}
}

func TestClient_Transcribe_500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"CUDA OOM"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	_, err := c.Transcribe(context.Background(), "tmp/x.wav", "pt")
	if !errors.Is(err, audio.ErrAudioGenericError) {
		t.Errorf("expected ErrAudioGenericError, got %v", err)
	}
}

func TestClient_GenericError_OnInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `not json`)
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	_, err := c.Health(context.Background())
	if !errors.Is(err, audio.ErrAudioGenericError) {
		t.Errorf("expected ErrAudioGenericError, got %v", err)
	}
}

func TestClient_Transcribe_BodyContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transcript":"","language":"pt","duration_seconds":0,"model":"medium"}`))
	}))
	defer srv.Close()

	c := audio.NewHTTPClient(srv.URL)
	if _, err := c.Transcribe(context.Background(), "tmp/x.wav", "pt"); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
}
