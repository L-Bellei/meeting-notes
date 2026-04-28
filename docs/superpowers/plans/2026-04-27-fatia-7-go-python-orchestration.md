# Fatia 7 — Go-Python Orchestration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 4 new Go endpoints (`/start`, `/stop`, `/process`, `/transcript`) on `/api/meetings/{id}/...` that orchestrate the full pipeline: capture audio via the Python service, transcribe via Whisper, run the 3 AI generations, persist the meeting status throughout. Pipeline runs asynchronously in goroutines; clients poll `GET /api/meetings/{id}` for progress.

**Architecture:** New `internal/audio/` package with an HTTP `Client` interface to the Python service. New `internal/services/Orchestrator` coordinates the pipeline with sync methods (`RunCapturePipeline`, `RunAIPipeline`) and async wrappers that fire goroutines. `MeetingHandler` gains 4 thin handlers that delegate to the orchestrator. The handler depends on a `MeetingOrchestrator` interface so tests can inject fakes.

**Tech Stack:** Go 1.26, chi v5, modernc.org/sqlite, faster-whisper (via Python service on port 8765), Anthropic API (already wired in Fatia 4).

---

## Project conventions

- Working directory: `F:\dev\meeting-notes`. Branch: `master`.
- Go tests run via `go test ./...` from the project root.
- Existing patterns: tests use real SQLite via `database.Open(t.TempDir() + "/test.db")`; pre-seed FK rows; `repository.ErrNotFound` and `services.ValidationError` are the sentinel/typed errors.
- All commits prefixed: `feat(audio):`, `feat(services):`, `feat(handlers):`, `test(...):`, `docs(...):`.
- Existing tests must keep passing throughout.

---

## Task 1: Audio HTTP Client

**Goal:** Build the HTTP client for the Python audio service. Pure I/O — no business logic. Sentinel errors map HTTP status to typed errors so the orchestrator can react.

**Files:**
- Create: `internal/audio/client.go`
- Create: `internal/audio/client_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/audio/client_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/audio/ -v
```

Expected: build error — `audio` package empty.

- [ ] **Step 3: Implement `client.go`**

Create `internal/audio/client.go`:

```go
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
		transcribeClient: &http.Client{Timeout: 10 * time.Minute},
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
```

- [ ] **Step 4: Run tests to verify they pass**

```
go test ./internal/audio/ -v
```

Expected: all 9 tests pass.

- [ ] **Step 5: Verify no regressions**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```
git add internal/audio/
git commit -m "feat(audio): add HTTP client for Python audio service"
```

---

## Task 2: Orchestrator Service

**Goal:** Implement the `Orchestrator` that coordinates `audio.Client` + Summary/KeyPoint/Task services. Pipeline methods are synchronous (testable); async wrappers spawn goroutines.

**Files:**
- Create: `internal/services/orchestrator.go`
- Create: `internal/services/orchestrator_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/services/orchestrator_test.go`:

```go
package services_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/audio"
	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeAudioClient struct {
	healthResp     *audio.HealthResponse
	healthErr      error
	startResp      *audio.StartResponse
	startErr       error
	stopResp       *audio.StopResponse
	stopErr        error
	transcribeResp *audio.TranscribeResponse
	transcribeErr  error

	startCalls, stopCalls, transcribeCalls int
}

func (f *fakeAudioClient) Health(ctx context.Context) (*audio.HealthResponse, error) {
	return f.healthResp, f.healthErr
}
func (f *fakeAudioClient) StartRecording(ctx context.Context) (*audio.StartResponse, error) {
	f.startCalls++
	return f.startResp, f.startErr
}
func (f *fakeAudioClient) StopRecording(ctx context.Context) (*audio.StopResponse, error) {
	f.stopCalls++
	return f.stopResp, f.stopErr
}
func (f *fakeAudioClient) Transcribe(ctx context.Context, path, language string) (*audio.TranscribeResponse, error) {
	f.transcribeCalls++
	return f.transcribeResp, f.transcribeErr
}

func newOrchTest(t *testing.T, audioClient audio.Client, aiClient ai.AIClient) (*services.Orchestrator, *repository.MeetingRepository, string) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mr := repository.NewMeetingRepository(db)
	sr := repository.NewSummaryRepository(db)
	kpr := repository.NewKeyPointRepository(db)
	tr := repository.NewTaskRepository(db)

	summarySvc := services.NewSummaryService(sr, aiClient)
	keyPointSvc := services.NewKeyPointService(kpr, aiClient)
	taskSvc := services.NewTaskService(tr, aiClient)

	orch := services.NewOrchestrator(mr, summarySvc, keyPointSvc, taskSvc, audioClient, "pt")

	now := time.Now().UTC()
	m := &models.Meeting{ID: "m-1", Title: "R", StartedAt: &now, Status: models.StatusPending}
	if err := mr.Create(context.Background(), m); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return orch, mr, m.ID
}

func TestOrchestrator_StartRecording_Success(t *testing.T) {
	fa := &fakeAudioClient{startResp: &audio.StartResponse{RecordingID: "r-1", StartedAt: time.Now().UTC()}}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})

	if err := orch.StartRecording(context.Background(), id); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if fa.startCalls != 1 {
		t.Errorf("startCalls = %d, want 1", fa.startCalls)
	}
	m, _ := mr.GetByID(context.Background(), id)
	if m.Status != models.StatusRecording {
		t.Errorf("status = %q, want recording", m.Status)
	}
}

func TestOrchestrator_StartRecording_AudioUnavailable(t *testing.T) {
	fa := &fakeAudioClient{startErr: audio.ErrAudioServiceUnavailable}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})

	err := orch.StartRecording(context.Background(), id)
	if !errors.Is(err, audio.ErrAudioServiceUnavailable) {
		t.Errorf("expected ErrAudioServiceUnavailable, got %v", err)
	}
	m, _ := mr.GetByID(context.Background(), id)
	if m.Status != models.StatusPending {
		t.Errorf("status = %q, want unchanged (pending)", m.Status)
	}
}

func TestOrchestrator_StartRecording_AlreadyRecording(t *testing.T) {
	fa := &fakeAudioClient{startResp: &audio.StartResponse{RecordingID: "r-1", StartedAt: time.Now().UTC()}}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})

	// First start succeeds
	orch.StartRecording(context.Background(), id)
	// Second should fail
	err := orch.StartRecording(context.Background(), id)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
	_ = mr
}

func TestOrchestrator_StartRecording_NotFound(t *testing.T) {
	fa := &fakeAudioClient{}
	orch, _, _ := newOrchTest(t, fa, &fakeAI{})

	err := orch.StartRecording(context.Background(), "nope")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestOrchestrator_RunCapturePipeline_Success(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	if err := os.WriteFile(wavPath, []byte("fake"), 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{RecordingID: "r-1", Path: wavPath, DurationSeconds: 12.5},
		transcribeResp: &audio.TranscribeResponse{Transcript: "olá mundo", Language: "pt", DurationSeconds: 12.5, Model: "medium"},
	}
	fai := &fakeAI{
		summaryText: "resumo",
		keyPoints:   []string{"ponto 1"},
		tasks:       []ai.TaskSuggestion{{Description: "fazer x", Priority: "medium"}},
	}
	orch, mr, id := newOrchTest(t, fa, fai)

	// Pre-condition: meeting must be in recording state
	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	if err := orch.RunCapturePipeline(context.Background(), id); err != nil {
		t.Fatalf("RunCapturePipeline: %v", err)
	}

	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
	if got.Transcript == nil || *got.Transcript != "olá mundo" {
		t.Errorf("transcript = %v", got.Transcript)
	}
	if _, err := os.Stat(wavPath); !os.IsNotExist(err) {
		t.Errorf("WAV should be deleted, but exists: %v", err)
	}
}

func TestOrchestrator_RunCapturePipeline_TranscribeFails(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	os.WriteFile(wavPath, []byte("fake"), 0o644)

	fa := &fakeAudioClient{
		stopResp:      &audio.StopResponse{Path: wavPath},
		transcribeErr: audio.ErrAudioGenericError,
	}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})

	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	err := orch.RunCapturePipeline(context.Background(), id)
	if !errors.Is(err, audio.ErrAudioGenericError) {
		t.Errorf("expected ErrAudioGenericError, got %v", err)
	}
	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusFailed {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		t.Errorf("WAV should be preserved on transcribe failure")
	}
}

func TestOrchestrator_RunCapturePipeline_AIFails(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	os.WriteFile(wavPath, []byte("fake"), 0o644)

	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{Path: wavPath, DurationSeconds: 12.5},
		transcribeResp: &audio.TranscribeResponse{Transcript: "olá", Language: "pt", DurationSeconds: 12.5},
	}
	fai := &fakeAI{err: errors.New("anthropic boom")}
	orch, mr, id := newOrchTest(t, fa, fai)

	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	err := orch.RunCapturePipeline(context.Background(), id)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusFailed {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.Transcript == nil || *got.Transcript != "olá" {
		t.Errorf("transcript should be persisted even when AI fails, got %v", got.Transcript)
	}
}

func TestOrchestrator_RunAIPipeline_Success(t *testing.T) {
	fa := &fakeAudioClient{}
	fai := &fakeAI{
		summaryText: "resumo",
		keyPoints:   []string{"ponto 1"},
		tasks:       []ai.TaskSuggestion{{Description: "fazer x", Priority: "low"}},
	}
	orch, mr, id := newOrchTest(t, fa, fai)

	m, _ := mr.GetByID(context.Background(), id)
	tr := "transcript existente"
	m.Transcript = &tr
	mr.Update(context.Background(), m)

	if err := orch.RunAIPipeline(context.Background(), id); err != nil {
		t.Fatalf("RunAIPipeline: %v", err)
	}
	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
}

func TestOrchestrator_RunAIPipeline_NoTranscript(t *testing.T) {
	orch, _, id := newOrchTest(t, &fakeAudioClient{}, &fakeAI{})
	err := orch.RunAIPipeline(context.Background(), id)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestOrchestrator_StopRecording_FiresGoroutine(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	os.WriteFile(wavPath, []byte("fake"), 0o644)

	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{Path: wavPath, DurationSeconds: 5.0},
		transcribeResp: &audio.TranscribeResponse{Transcript: "ok", Language: "pt", DurationSeconds: 5.0},
	}
	fai := &fakeAI{summaryText: "s", keyPoints: []string{"k"}, tasks: []ai.TaskSuggestion{{Description: "t", Priority: "low"}}}
	orch, mr, id := newOrchTest(t, fa, fai)

	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	if err := orch.StopRecording(context.Background(), id); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	orch.WaitPipelines()

	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
}

func TestOrchestrator_StopRecording_NotRecording(t *testing.T) {
	orch, _, id := newOrchTest(t, &fakeAudioClient{}, &fakeAI{})
	err := orch.StopRecording(context.Background(), id)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestOrchestrator_Reprocess_NoTranscript(t *testing.T) {
	orch, _, id := newOrchTest(t, &fakeAudioClient{}, &fakeAI{})
	err := orch.Reprocess(context.Background(), id)
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestOrchestrator_Reprocess_FiresGoroutine(t *testing.T) {
	fai := &fakeAI{summaryText: "s", keyPoints: []string{"k"}, tasks: []ai.TaskSuggestion{{Description: "t", Priority: "low"}}}
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, fai)

	m, _ := mr.GetByID(context.Background(), id)
	tr := "transcript"
	m.Transcript = &tr
	m.Status = models.StatusFailed
	mr.Update(context.Background(), m)

	if err := orch.Reprocess(context.Background(), id); err != nil {
		t.Fatalf("Reprocess: %v", err)
	}
	orch.WaitPipelines()

	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
}

func TestOrchestrator_SetTranscriptAndProcess_Empty(t *testing.T) {
	orch, _, id := newOrchTest(t, &fakeAudioClient{}, &fakeAI{})
	err := orch.SetTranscriptAndProcess(context.Background(), id, "")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestOrchestrator_SetTranscriptAndProcess_WhileRecording(t *testing.T) {
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, &fakeAI{})
	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	err := orch.SetTranscriptAndProcess(context.Background(), id, "x")
	var ve *services.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestOrchestrator_SetTranscriptAndProcess_Success(t *testing.T) {
	fai := &fakeAI{summaryText: "s", keyPoints: []string{"k"}, tasks: []ai.TaskSuggestion{{Description: "t", Priority: "low"}}}
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, fai)

	if err := orch.SetTranscriptAndProcess(context.Background(), id, "manual transcript"); err != nil {
		t.Fatalf("SetTranscriptAndProcess: %v", err)
	}
	orch.WaitPipelines()

	got, _ := mr.GetByID(context.Background(), id)
	if got.Transcript == nil || *got.Transcript != "manual transcript" {
		t.Errorf("transcript = %v", got.Transcript)
	}
	if got.Status != models.StatusCompleted {
		t.Errorf("status = %q, want completed", got.Status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./internal/services/ -run TestOrchestrator -v
```

Expected: build error — `services.Orchestrator` does not exist.

- [ ] **Step 3: Implement `orchestrator.go`**

Create `internal/services/orchestrator.go`:

```go
package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"meeting-notes/internal/audio"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

const pipelineTimeout = 15 * time.Minute

type Orchestrator struct {
	repo        *repository.MeetingRepository
	summarySvc  *SummaryService
	keyPointSvc *KeyPointService
	taskSvc     *TaskService
	audio       audio.Client
	language    string
	pipelineWG  sync.WaitGroup
}

func NewOrchestrator(
	repo *repository.MeetingRepository,
	summarySvc *SummaryService,
	keyPointSvc *KeyPointService,
	taskSvc *TaskService,
	audioClient audio.Client,
	language string,
) *Orchestrator {
	return &Orchestrator{
		repo:        repo,
		summarySvc:  summarySvc,
		keyPointSvc: keyPointSvc,
		taskSvc:     taskSvc,
		audio:       audioClient,
		language:    language,
	}
}

func (o *Orchestrator) WaitPipelines() {
	o.pipelineWG.Wait()
}

func (o *Orchestrator) StartRecording(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Status == models.StatusRecording {
		return &ValidationError{"meeting is already recording"}
	}
	if _, err := o.audio.StartRecording(ctx); err != nil {
		return err
	}
	m.Status = models.StatusRecording
	return o.repo.Update(ctx, m)
}

func (o *Orchestrator) StopRecording(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Status != models.StatusRecording {
		return &ValidationError{"meeting is not recording"}
	}
	o.spawnPipeline(meetingID, o.RunCapturePipeline)
	return nil
}

func (o *Orchestrator) Reprocess(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Transcript == nil || *m.Transcript == "" {
		return &ValidationError{"transcript is required for processing"}
	}
	o.spawnPipeline(meetingID, o.RunAIPipeline)
	return nil
}

func (o *Orchestrator) SetTranscriptAndProcess(ctx context.Context, meetingID, transcript string) error {
	if transcript == "" {
		return &ValidationError{"transcript is required"}
	}
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Status == models.StatusRecording || m.Status == models.StatusTranscribing {
		return &ValidationError{"cannot set transcript while recording or transcribing"}
	}
	m.Transcript = &transcript
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}
	o.spawnPipeline(meetingID, o.RunAIPipeline)
	return nil
}

func (o *Orchestrator) spawnPipeline(meetingID string, fn func(context.Context, string) error) {
	o.pipelineWG.Add(1)
	go func() {
		defer o.pipelineWG.Done()
		bgCtx, cancel := context.WithTimeout(context.Background(), pipelineTimeout)
		defer cancel()
		if err := fn(bgCtx, meetingID); err != nil {
			log.Printf("pipeline %s: %v", meetingID, err)
		}
	}()
}

func (o *Orchestrator) RunCapturePipeline(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}

	m.Status = models.StatusTranscribing
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}

	stopResp, err := o.audio.StopRecording(ctx)
	if err != nil {
		o.markFailed(ctx, m)
		return err
	}

	trResp, err := o.audio.Transcribe(ctx, stopResp.Path, o.language)
	if err != nil {
		o.markFailed(ctx, m)
		return err
	}

	m.Transcript = &trResp.Transcript
	dur := int(stopResp.DurationSeconds)
	m.DurationSeconds = &dur
	m.Status = models.StatusProcessing
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}

	if err := os.Remove(stopResp.Path); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: delete WAV %s: %v", stopResp.Path, err)
	}

	if err := o.runAIGeneration(ctx, m); err != nil {
		o.markFailed(ctx, m)
		return err
	}

	m.Status = models.StatusCompleted
	return o.repo.Update(ctx, m)
}

func (o *Orchestrator) RunAIPipeline(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Transcript == nil || *m.Transcript == "" {
		return &ValidationError{"transcript is required"}
	}

	m.Status = models.StatusProcessing
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}

	if err := o.runAIGeneration(ctx, m); err != nil {
		o.markFailed(ctx, m)
		return err
	}

	m.Status = models.StatusCompleted
	return o.repo.Update(ctx, m)
}

func (o *Orchestrator) runAIGeneration(ctx context.Context, m *models.Meeting) error {
	if _, err := o.summarySvc.Generate(ctx, m); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	if _, err := o.keyPointSvc.Generate(ctx, m); err != nil {
		return fmt.Errorf("key_points: %w", err)
	}
	if _, err := o.taskSvc.Generate(ctx, m); err != nil {
		return fmt.Errorf("tasks: %w", err)
	}
	return nil
}

func (o *Orchestrator) markFailed(ctx context.Context, m *models.Meeting) {
	m.Status = models.StatusFailed
	if err := o.repo.Update(ctx, m); err != nil {
		log.Printf("warning: mark failed %s: %v", m.ID, err)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```
go test ./internal/services/ -run TestOrchestrator -v
```

Expected: 14 passing tests.

- [ ] **Step 5: Verify no regressions**

```
go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```
git add internal/services/orchestrator.go internal/services/orchestrator_test.go
git commit -m "feat(services): add Orchestrator coordinating audio + AI pipeline"
```

---

## Task 3: Meeting Handlers + main.go wiring

**Goal:** Add `Start`, `Stop`, `Process`, `SetTranscript` handlers to `MeetingHandler`. Define `MeetingOrchestrator` interface in handlers package for mockability. Update `MeetingHandler` constructor to accept the interface. Update `cmd/api/main.go` to wire the audio client and orchestrator.

**Files:**
- Modify: `internal/handlers/meeting_handler.go`
- Modify: `internal/handlers/meeting_handler_test.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Update `meeting_handler.go` — add interface, struct field, 4 handlers**

Replace the top portion of `internal/handlers/meeting_handler.go` (everything before the `type createMeetingRequest` line) with:

```go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"meeting-notes/internal/audio"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type MeetingOrchestrator interface {
	StartRecording(ctx context.Context, meetingID string) error
	StopRecording(ctx context.Context, meetingID string) error
	Reprocess(ctx context.Context, meetingID string) error
	SetTranscriptAndProcess(ctx context.Context, meetingID, transcript string) error
}

type MeetingHandler struct {
	svc          *services.MeetingService
	summaryRepo  *repository.SummaryRepository
	keyPointRepo *repository.KeyPointRepository
	taskRepo     *repository.TaskRepository
	orch         MeetingOrchestrator
}

func NewMeetingHandler(
	svc *services.MeetingService,
	summaryRepo *repository.SummaryRepository,
	keyPointRepo *repository.KeyPointRepository,
	taskRepo *repository.TaskRepository,
	orch MeetingOrchestrator,
) *MeetingHandler {
	return &MeetingHandler{
		svc:          svc,
		summaryRepo:  summaryRepo,
		keyPointRepo: keyPointRepo,
		taskRepo:     taskRepo,
		orch:         orch,
	}
}
```

Then at the bottom of the file (after `Delete`), add the 4 new handlers and the DTO:

```go
type setTranscriptRequest struct {
	Transcript string `json:"transcript"`
}

func (h *MeetingHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.StartRecording(r.Context(), id); err != nil {
		h.writeOrchError(w, err, "failed to start recording")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *MeetingHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.StopRecording(r.Context(), id); err != nil {
		h.writeOrchError(w, err, "failed to stop recording")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *MeetingHandler) Process(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.Reprocess(r.Context(), id); err != nil {
		h.writeOrchError(w, err, "failed to start processing")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *MeetingHandler) SetTranscript(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req setTranscriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.orch.SetTranscriptAndProcess(r.Context(), id, req.Transcript); err != nil {
		h.writeOrchError(w, err, "failed to set transcript")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *MeetingHandler) writeOrchError(w http.ResponseWriter, err error, fallback string) {
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "meeting not found")
		return
	}
	var ve *services.ValidationError
	if errors.As(err, &ve) {
		writeError(w, http.StatusUnprocessableEntity, ve.Message)
		return
	}
	if errors.Is(err, audio.ErrAudioServiceUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "audio service unavailable")
		return
	}
	if errors.Is(err, audio.ErrAudioServiceConflict) {
		writeError(w, http.StatusConflict, "audio service conflict")
		return
	}
	writeError(w, http.StatusInternalServerError, fallback)
}
```

- [ ] **Step 2: Add tests for the 4 new handlers**

Append to `internal/handlers/meeting_handler_test.go`. First, update `newTestMeetingHandler` and `newTestMeetingAndThemeHandlers` to pass a fake orchestrator.

Add after the existing imports in `meeting_handler_test.go`:

```go
type fakeOrchestrator struct {
	startErr            error
	stopErr             error
	reprocessErr        error
	setTranscriptErr    error

	startCalls          int
	stopCalls           int
	reprocessCalls      int
	setTranscriptCalls  int
	lastSetTranscript   string
}

func (f *fakeOrchestrator) StartRecording(ctx context.Context, meetingID string) error {
	f.startCalls++
	return f.startErr
}
func (f *fakeOrchestrator) StopRecording(ctx context.Context, meetingID string) error {
	f.stopCalls++
	return f.stopErr
}
func (f *fakeOrchestrator) Reprocess(ctx context.Context, meetingID string) error {
	f.reprocessCalls++
	return f.reprocessErr
}
func (f *fakeOrchestrator) SetTranscriptAndProcess(ctx context.Context, meetingID, transcript string) error {
	f.setTranscriptCalls++
	f.lastSetTranscript = transcript
	return f.setTranscriptErr
}
```

Note: this requires `"context"` in the imports of `meeting_handler_test.go` (it's likely already there).

Update the existing `newTestMeetingHandler` to pass a default fake orchestrator. Replace it with:

```go
func newTestMeetingHandler(t *testing.T) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db)),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
		&fakeOrchestrator{},
	)
}
```

And update `newTestMeetingAndThemeHandlers`:

```go
func newTestMeetingAndThemeHandlers(t *testing.T) (*handlers.MeetingHandler, *handlers.ThemeHandler) {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	mh := handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db)),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
		&fakeOrchestrator{},
	)
	th := handlers.NewThemeHandler(services.NewThemeService(repository.NewThemeRepository(db)))
	return mh, th
}
```

Also update `TestMeetingHandler_GetByID_PopulatesNestedData` (which constructs `MeetingHandler` directly). Replace the line:

```go
mh := handlers.NewMeetingHandler(services.NewMeetingService(mr), sr, kpr, tr)
```

with:

```go
mh := handlers.NewMeetingHandler(services.NewMeetingService(mr), sr, kpr, tr, &fakeOrchestrator{})
```

Now add the 9 new tests at the bottom of the file. These tests need a way to construct a `MeetingHandler` with a custom `fakeOrchestrator`. Add a helper:

```go
func newTestMeetingHandlerWithOrch(t *testing.T, orch *fakeOrchestrator) *handlers.MeetingHandler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return handlers.NewMeetingHandler(
		services.NewMeetingService(repository.NewMeetingRepository(db)),
		repository.NewSummaryRepository(db),
		repository.NewKeyPointRepository(db),
		repository.NewTaskRepository(db),
		orch,
	)
}
```

And the 9 tests:

```go
func TestMeetingHandler_Start_Success(t *testing.T) {
	orch := &fakeOrchestrator{}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/start", nil), "abc")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
	if orch.startCalls != 1 {
		t.Errorf("startCalls = %d, want 1", orch.startCalls)
	}
}

func TestMeetingHandler_Start_AudioServiceDown(t *testing.T) {
	orch := &fakeOrchestrator{startErr: audio.ErrAudioServiceUnavailable}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/start", nil), "abc")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestMeetingHandler_Start_NotFound(t *testing.T) {
	orch := &fakeOrchestrator{startErr: repository.ErrNotFound}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/nope/start", nil), "nope")
	w := httptest.NewRecorder()
	h.Start(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestMeetingHandler_Stop_Success(t *testing.T) {
	orch := &fakeOrchestrator{}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/stop", nil), "abc")
	w := httptest.NewRecorder()
	h.Stop(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
}

func TestMeetingHandler_Stop_NotRecording(t *testing.T) {
	orch := &fakeOrchestrator{stopErr: &services.ValidationError{Message: "not recording"}}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/stop", nil), "abc")
	w := httptest.NewRecorder()
	h.Stop(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_Process_Success(t *testing.T) {
	orch := &fakeOrchestrator{}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/process", nil), "abc")
	w := httptest.NewRecorder()
	h.Process(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
}

func TestMeetingHandler_Process_NoTranscript(t *testing.T) {
	orch := &fakeOrchestrator{reprocessErr: &services.ValidationError{Message: "transcript is required"}}
	h := newTestMeetingHandlerWithOrch(t, orch)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/process", nil), "abc")
	w := httptest.NewRecorder()
	h.Process(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}

func TestMeetingHandler_SetTranscript_Success(t *testing.T) {
	orch := &fakeOrchestrator{}
	h := newTestMeetingHandlerWithOrch(t, orch)
	body := bytes.NewBufferString(`{"transcript":"texto manual"}`)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/transcript", body), "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SetTranscript(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
	if orch.lastSetTranscript != "texto manual" {
		t.Errorf("lastSetTranscript = %q", orch.lastSetTranscript)
	}
}

func TestMeetingHandler_SetTranscript_EmptyBody(t *testing.T) {
	orch := &fakeOrchestrator{setTranscriptErr: &services.ValidationError{Message: "transcript is required"}}
	h := newTestMeetingHandlerWithOrch(t, orch)
	body := bytes.NewBufferString(`{"transcript":""}`)
	req := withChiID(httptest.NewRequest(http.MethodPost, "/api/meetings/abc/transcript", body), "abc")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SetTranscript(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
}
```

The tests use `audio.ErrAudioServiceUnavailable` — add `"meeting-notes/internal/audio"` to the imports of `meeting_handler_test.go` (along with `"context"` if not already there).

- [ ] **Step 3: Run handler tests to verify they pass**

```
go test ./internal/handlers/ -v
```

Expected: all existing tests still pass + 9 new pass.

- [ ] **Step 4: Update `cmd/api/main.go`**

Read the current `cmd/api/main.go`. Replace the wiring section between `cfg := config.Load()` and `log.Printf("server listening...`)` with:

```go
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	var aiClient ai.AIClient
	if cfg.AnthropicAPIKey != "" {
		aiClient = ai.NewAnthropicClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
	}

	audioClient := audio.NewHTTPClient(cfg.AudioServiceURL)

	themeRepo := repository.NewThemeRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	themeSvc := services.NewThemeService(themeRepo)
	meetingSvc := services.NewMeetingService(meetingRepo)
	summarySvc := services.NewSummaryService(summaryRepo, aiClient)
	keyPointSvc := services.NewKeyPointService(keyPointRepo, aiClient)
	taskSvc := services.NewTaskService(taskRepo, aiClient)

	whisperLanguage := os.Getenv("WHISPER_LANGUAGE")
	if whisperLanguage == "" {
		whisperLanguage = "pt"
	}
	orch := services.NewOrchestrator(meetingRepo, summarySvc, keyPointSvc, taskSvc, audioClient, whisperLanguage)

	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo, orch)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", healthHandler(db))

	r.Route("/api/themes", func(r chi.Router) {
		r.Get("/", themeHandler.List)
		r.Post("/", themeHandler.Create)
		r.Get("/{id}", themeHandler.GetByID)
		r.Put("/{id}", themeHandler.Update)
		r.Delete("/{id}", themeHandler.Delete)
	})

	r.Route("/api/meetings", func(r chi.Router) {
		r.Get("/", meetingHandler.List)
		r.Post("/", meetingHandler.Create)
		r.Get("/{id}", meetingHandler.GetByID)
		r.Put("/{id}", meetingHandler.Update)
		r.Delete("/{id}", meetingHandler.Delete)

		r.Post("/{id}/start", meetingHandler.Start)
		r.Post("/{id}/stop", meetingHandler.Stop)
		r.Post("/{id}/process", meetingHandler.Process)
		r.Post("/{id}/transcript", meetingHandler.SetTranscript)

		r.Route("/{id}/summary", func(r chi.Router) {
			r.Get("/", summaryHandler.Get)
			r.Post("/", summaryHandler.Create)
			r.Put("/", summaryHandler.Update)
			r.Delete("/", summaryHandler.Delete)
			r.Post("/generate", summaryHandler.Generate)
		})

		r.Route("/{id}/key_points", func(r chi.Router) {
			r.Get("/", keyPointHandler.List)
			r.Post("/", keyPointHandler.Create)
			r.Post("/generate", keyPointHandler.Generate)
			r.Put("/{kpId}", keyPointHandler.Update)
			r.Delete("/{kpId}", keyPointHandler.Delete)
		})

		r.Route("/{id}/tasks", func(r chi.Router) {
			r.Get("/", taskHandler.List)
			r.Post("/", taskHandler.Create)
			r.Post("/generate", taskHandler.Generate)
			r.Put("/{taskId}", taskHandler.Update)
			r.Delete("/{taskId}", taskHandler.Delete)
		})
	})
```

The imports must include `"meeting-notes/internal/audio"` and `"os"` (for the WHISPER_LANGUAGE env lookup, if not already imported).

The full `cmd/api/main.go` import block should be:

```go
import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/audio"
	"meeting-notes/internal/config"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)
```

- [ ] **Step 5: Verify the project compiles**

```
go build ./cmd/api/...
```

Expected: no errors.

- [ ] **Step 6: Run full test suite**

```
go test ./...
```

Expected: all tests pass across all packages (audio, services, handlers, repository, database).

- [ ] **Step 7: Commit**

```
git add internal/handlers/meeting_handler.go internal/handlers/meeting_handler_test.go cmd/api/main.go
git commit -m "feat: add /start /stop /process /transcript meeting endpoints"
```

---

## Task 4: Smoke test (manual, by user)

**Goal:** End-to-end verification that the full pipeline works against the real Python audio service and the real Anthropic API.

This task has no code or commits. The user runs the smoke test on their Windows machine.

**Files:** none.

- [ ] **Step 1: Start the audio service**

In one terminal:
```
cd audio-service
.venv\Scripts\activate
uvicorn main:app --port 8765
```

Wait for "Application startup complete".

- [ ] **Step 2: Start the Go backend**

In another terminal:
```
cd F:\dev\meeting-notes
go run ./cmd/api
```

Wait for "server listening on :8080".

- [ ] **Step 3: Create a meeting**

```
curl -X POST http://localhost:8080/api/meetings ^
     -H "Content-Type: application/json" ^
     -d "{\"title\":\"Smoke test E2E\"}"
```

Note the `id` in the response.

- [ ] **Step 4: Start recording**

```
curl -X POST http://localhost:8080/api/meetings/<id>/start
```

Expected: `202 Accepted` (empty body).

- [ ] **Step 5: Speak and play music for ~15 seconds**

(Same as the audio-service smoke test from Fatia 5.)

- [ ] **Step 6: Stop recording**

```
curl -X POST http://localhost:8080/api/meetings/<id>/stop
```

Expected: `202 Accepted`.

- [ ] **Step 7: Poll meeting status**

```
curl http://localhost:8080/api/meetings/<id>
```

Expected progression over ~30-60s:
- `status: "transcribing"` (during Whisper)
- `status: "processing"` (during AI generation)
- `status: "completed"` with non-empty `transcript`, `summary`, `key_points` (array), `tasks` (array)

- [ ] **Step 8: Verify the WAV was deleted**

```
ls audio-service/tmp/
```

Expected: only `.gitkeep`. No `rec-*.wav`.

- [ ] **Step 9: Test /transcript (manual transcript path)**

Create another meeting:
```
curl -X POST http://localhost:8080/api/meetings ^
     -H "Content-Type: application/json" ^
     -d "{\"title\":\"Manual transcript test\"}"
```

Submit a transcript directly:
```
curl -X POST http://localhost:8080/api/meetings/<id>/transcript ^
     -H "Content-Type: application/json" ^
     -d "{\"transcript\":\"Hoje discutimos o roadmap do produto e definimos prioridades para o próximo trimestre.\"}"
```

Expected: `202`. Then `GET /api/meetings/<id>` shows progression `processing` → `completed` with summary/key_points/tasks generated from the manual text.

- [ ] **Step 10: Test /process (re-run AI)**

Using the meeting from Step 9:
```
curl -X POST http://localhost:8080/api/meetings/<id>/process
```

Expected: `202`. The summary/key_points/tasks get regenerated (overwritten in DB).

- [ ] **Step 11: Test 503 when audio service is down**

Stop uvicorn (`Ctrl+C` in the audio-service terminal). Then:
```
curl -X POST http://localhost:8080/api/meetings/<other-id>/start -i
```

Expected: `HTTP/1.1 503 Service Unavailable` with body `{"error":"audio service unavailable"}`.

---

## Final verification checklist

- [ ] `go test ./...` passes (audio, services, handlers tests all green)
- [ ] `go build ./cmd/api/...` succeeds
- [ ] Smoke test produces a meeting with `status: "completed"`, populated transcript + summary + key_points + tasks
- [ ] WAVs are deleted from `audio-service/tmp/` after successful transcription
- [ ] `/start` returns 503 when audio service is down
- [ ] `/transcript` accepts text without audio and runs the full AI pipeline
- [ ] `/process` regenerates AI from existing transcript without re-recording
