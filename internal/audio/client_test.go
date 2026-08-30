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
	got, err := c.Transcribe(context.Background(), "tmp/rec-1.wav", "pt", "auto")
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
	_, err := c.Transcribe(context.Background(), "tmp/x.wav", "pt", "auto")
	if !errors.Is(err, audio.ErrAudioGenericError) {
		t.Errorf("expected ErrAudioGenericError, got %v", err)
	}
}

func TestTranscribe_SendsDeviceAndParsesEffective(t *testing.T) {
	var receivedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Write([]byte(`{"transcript":"olá","language":"pt","duration_seconds":1.0,"model":"medium","device":"cuda"}`))
	}))
	defer srv.Close()
	c := audio.NewHTTPClient(srv.URL)
	got, err := c.Transcribe(context.Background(), "tmp/rec-1.wav", "pt", "auto")
	if err != nil {
		t.Fatal(err)
	}
	if receivedBody["device"] != "auto" {
		t.Fatalf("device no request = %q, want auto", receivedBody["device"])
	}
	if got.Device != "cuda" {
		t.Fatalf("Device = %q, want cuda", got.Device)
	}
}

func TestHealth_ParsesGPUFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","state":"idle","loopback_available":true,"model_loaded":true,"model_name":"medium","device":"vulkan","gpu_available":true,"gpu_name":"AMD Radeon RX 7600","gpu_vram_mb":8192,"gpu_vendor":"amd","gpu_backend":"vulkan","vulkan_model_ready":true}`))
	}))
	defer srv.Close()
	c := audio.NewHTTPClient(srv.URL)
	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !h.GPUAvailable || h.GPUName != "AMD Radeon RX 7600" || h.GPUVRAMMB != 8192 {
		t.Fatalf("gpu fields: %+v", h)
	}
	if h.GPUVendor != "amd" || h.GPUBackend != "vulkan" || !h.VulkanModelReady || h.Device != "vulkan" {
		t.Fatalf("vulkan fields: %+v", h)
	}
}

func TestHealth_MissingVulkanFieldsAreZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","gpu_available":true,"gpu_vendor":null,"gpu_backend":null}`))
	}))
	defer srv.Close()
	h, err := audio.NewHTTPClient(srv.URL).Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.GPUVendor != "" || h.GPUBackend != "" || h.VulkanModelReady {
		t.Fatalf("expected zero values, got %+v", h)
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
	if _, err := c.Transcribe(context.Background(), "tmp/x.wav", "pt", "auto"); err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
}
