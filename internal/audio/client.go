package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	ErrAudioServiceUnavailable = errors.New("audio service unavailable")
	ErrAudioServiceConflict    = errors.New("audio service conflict")
	ErrAudioGenericError       = errors.New("audio service error")
)

type HealthResponse struct {
	Status            string `json:"status"`
	State             string `json:"state"`
	LoopbackAvailable bool   `json:"loopback_available"`
	ModelLoaded       bool   `json:"model_loaded"`
	ModelName         string `json:"model_name"`
	Device            string `json:"device"`
}

type StartResponse struct {
	RecordingID string    `json:"recording_id"`
	StartedAt   time.Time `json:"started_at"`
}

type StopResponse struct {
	RecordingID     string  `json:"recording_id"`
	Path            string  `json:"path"`
	DurationSeconds float64 `json:"duration_seconds"`
	SizeBytes       int64   `json:"size_bytes"`
	Partial         bool    `json:"partial"`
}

type TranscribeResponse struct {
	Transcript      string  `json:"transcript"`
	Language        string  `json:"language"`
	DurationSeconds float64 `json:"duration_seconds"`
	Model           string  `json:"model"`
}

type Client interface {
	Health(ctx context.Context) (*HealthResponse, error)
	StartRecording(ctx context.Context) (*StartResponse, error)
	StopRecording(ctx context.Context) (*StopResponse, error)
	Transcribe(ctx context.Context, path, language string) (*TranscribeResponse, error)
}

type httpClient struct {
	baseURL          string
	defaultClient    *http.Client
	transcribeClient *http.Client
}

func NewHTTPClient(baseURL string) *httpClient {
	return &httpClient{
		baseURL:          strings.TrimRight(baseURL, "/"),
		defaultClient:    &http.Client{Timeout: 30 * time.Second},
		transcribeClient: &http.Client{Timeout: 60 * time.Minute},
	}
}

func (c *httpClient) Health(ctx context.Context) (*HealthResponse, error) {
	var out HealthResponse
	if err := c.do(ctx, c.defaultClient, http.MethodGet, "/health", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *httpClient) StartRecording(ctx context.Context) (*StartResponse, error) {
	var out StartResponse
	if err := c.do(ctx, c.defaultClient, http.MethodPost, "/recording/start", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *httpClient) StopRecording(ctx context.Context) (*StopResponse, error) {
	var out StopResponse
	if err := c.do(ctx, c.defaultClient, http.MethodPost, "/recording/stop", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *httpClient) Transcribe(ctx context.Context, path, language string) (*TranscribeResponse, error) {
	body, err := json.Marshal(map[string]string{"path": path, "language": language})
	if err != nil {
		return nil, fmt.Errorf("marshal transcribe request: %w", err)
	}
	var out TranscribeResponse
	if err := c.do(ctx, c.transcribeClient, http.MethodPost, "/transcribe", bytes.NewReader(body), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *httpClient) do(ctx context.Context, client *http.Client, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, ErrAudioServiceUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s (%d): %s: %w", method, path, resp.StatusCode, string(raw), ErrAudioServiceConflict)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s (%d): %s: %w", method, path, resp.StatusCode, string(raw), ErrAudioGenericError)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s: %w: %w", method, path, err, ErrAudioGenericError)
	}
	return nil
}
