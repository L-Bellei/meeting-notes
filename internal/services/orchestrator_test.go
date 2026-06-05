package services_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/audio"
	"meeting-notes/internal/database"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type fakeSettings struct {
	data map[string]string
}

func (f *fakeSettings) GetAll(ctx context.Context) (map[string]string, error) {
	if f.data == nil {
		return map[string]string{}, nil
	}
	return f.data, nil
}

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
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	if f.healthResp != nil {
		return f.healthResp, nil
	}
	return &audio.HealthResponse{Status: "ok", ModelLoaded: true}, nil
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

// configuredAISettings reflects the production invariant: when a working AI
// client is injected, the settings hold a usable provider + key.
func configuredAISettings() map[string]string {
	return map[string]string{"ai_provider": "anthropic", "anthropic_api_key": "sk-test"}
}

func newOrchTest(t *testing.T, audioClient audio.Client, aiClient ai.AIClient) (*services.Orchestrator, *repository.MeetingRepository, string) {
	return newOrchTestSettings(t, audioClient, aiClient, configuredAISettings())
}

func newOrchTestSettings(t *testing.T, audioClient audio.Client, aiClient ai.AIClient, settings map[string]string) (*services.Orchestrator, *repository.MeetingRepository, string) {
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
	thr := repository.NewThemeRepository(db)

	summarySvc := services.NewSummaryService(sr, aiClient)
	keyPointSvc := services.NewKeyPointService(kpr, aiClient)
	taskSvc := services.NewTaskService(tr, aiClient)

	orch := services.NewOrchestrator(mr, thr, summarySvc, keyPointSvc, taskSvc, audioClient, &fakeSettings{data: settings}, nil)

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

	orch.StartRecording(context.Background(), id)
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

func fakeWAVBytes() []byte {
	return make([]byte, 12*1024)
}

func TestOrchestrator_RunCapturePipeline_Success(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	if err := os.WriteFile(wavPath, fakeWAVBytes(), 0o644); err != nil {
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
	os.WriteFile(wavPath, fakeWAVBytes(), 0o644)

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
	os.WriteFile(wavPath, fakeWAVBytes(), 0o644)

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

func TestOrchestrator_RunCapturePipeline_SkipsAIWhenNotConfigured(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	if err := os.WriteFile(wavPath, fakeWAVBytes(), 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{Path: wavPath, DurationSeconds: 12.5},
		transcribeResp: &audio.TranscribeResponse{Transcript: "olá mundo", Language: "pt", DurationSeconds: 12.5},
	}
	// If AI were invoked despite missing config, this error would mark the meeting failed.
	fai := &fakeAI{err: errors.New("AI must not be called when not configured")}
	orch, mr, id := newOrchTestSettings(t, fa, fai, map[string]string{}) // no provider/key

	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	if err := orch.RunCapturePipeline(context.Background(), id); err != nil {
		t.Fatalf("RunCapturePipeline: %v", err)
	}
	got, _ := mr.GetByID(context.Background(), id)
	if got.Status != models.StatusCompleted {
		t.Errorf("status = %q, want completed (graceful skip)", got.Status)
	}
	if got.Transcript == nil || *got.Transcript != "olá mundo" {
		t.Errorf("transcript should be preserved, got %v", got.Transcript)
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
	os.WriteFile(wavPath, fakeWAVBytes(), 0o644)

	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{Path: wavPath, DurationSeconds: 5.0},
		transcribeResp: &audio.TranscribeResponse{Transcript: "ok", Language: "pt", DurationSeconds: 5.0},
	}
	fai := &fakeAI{summaryText: "s", keyPoints: []string{"k"}, tasks: []ai.TaskSuggestion{{Description: "t", Priority: "low"}}}
	orch, mr, id := newOrchTest(t, fa, fai)

	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	if err := orch.StopRecording(context.Background(), id, false); err != nil {
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
	err := orch.StopRecording(context.Background(), id, false)
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

func TestOrchestrator_NotifyFn_CalledOnStatusChange(t *testing.T) {
	transcript := "hello world"
	wavPath := t.TempDir() + "/rec.wav"
	if err := os.WriteFile(wavPath, fakeWAVBytes(), 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	fa := &fakeAudioClient{
		startResp:      &audio.StartResponse{RecordingID: "r-1", StartedAt: time.Now().UTC()},
		stopResp:       &audio.StopResponse{Path: wavPath, DurationSeconds: 10},
		transcribeResp: &audio.TranscribeResponse{Transcript: transcript},
	}
	orch, _, id := newOrchTest(t, fa, &fakeAI{
		summaryText: "s",
		keyPoints:   []string{"kp1"},
		tasks:       []ai.TaskSuggestion{{Description: "t1", Priority: "medium"}},
	})

	type call struct{ meetingID, status string }
	var mu sync.Mutex
	var calls []call
	orch.SetNotifyFn(func(meetingID, status string) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, call{meetingID, status})
	})

	if err := orch.StartRecording(context.Background(), id); err != nil {
		t.Fatalf("StartRecording: %v", err)
	}
	if err := orch.StopRecording(context.Background(), id, false); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}
	orch.WaitPipelines()

	mu.Lock()
	defer mu.Unlock()

	wantStatuses := []string{"recording", "transcribing", "processing", "completed"}
	if len(calls) != len(wantStatuses) {
		t.Fatalf("expected %d notify calls, got %d: %+v", len(wantStatuses), len(calls), calls)
	}
	for i, want := range wantStatuses {
		if calls[i].meetingID != id {
			t.Errorf("call[%d] meetingID = %q, want %q", i, calls[i].meetingID, id)
		}
		if calls[i].status != want {
			t.Errorf("call[%d] status = %q, want %q", i, calls[i].status, want)
		}
	}
}

func newOrchTestWithDB(t *testing.T, audioClient audio.Client, aiClient ai.AIClient) (*services.Orchestrator, *repository.MeetingRepository, *sql.DB, string) {
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
	thr := repository.NewThemeRepository(db)

	summarySvc := services.NewSummaryService(sr, aiClient)
	keyPointSvc := services.NewKeyPointService(kpr, aiClient)
	taskSvc := services.NewTaskService(tr, aiClient)

	orch := services.NewOrchestrator(mr, thr, summarySvc, keyPointSvc, taskSvc, audioClient, &fakeSettings{data: configuredAISettings()}, nil)

	now := time.Now().UTC()
	m := &models.Meeting{ID: "m-orch", Title: "Orch Test", StartedAt: &now, Status: models.StatusPending}
	if err := mr.Create(context.Background(), m); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	return orch, mr, db, m.ID
}

func TestOrchestrator_RunAIPipeline_SyncsSearch(t *testing.T) {
	transcript := "sprint review completed successfully"
	fakeAIClient := &fakeAI{
		summaryText: "Sprint summary",
		keyPoints:   []string{"Point 1"},
		tasks:       []ai.TaskSuggestion{{Description: "Task 1", Priority: "low"}},
	}
	orch, mr, db, id := newOrchTestWithDB(t, &fakeAudioClient{}, fakeAIClient)
	ctx := context.Background()

	searchRepo := repository.NewSearchRepository(db)
	orch.SetSearchRepo(searchRepo)

	m, err := mr.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get meeting: %v", err)
	}
	m.Transcript = &transcript
	if err := mr.Update(ctx, m); err != nil {
		t.Fatalf("update meeting with transcript: %v", err)
	}

	if err := orch.RunAIPipeline(ctx, id); err != nil {
		t.Fatalf("RunAIPipeline: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	results, err := searchRepo.Search(ctx, "sprint")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected FTS result after pipeline, got none")
	}
}
