package services_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/audio"
	"meeting-notes/internal/models"
	"meeting-notes/internal/services"
)

type blockingPipelineAI struct {
	summaryStarted chan struct{}
	releaseSummary chan struct{}
	summaryCalls   atomic.Int32
}

func (f *blockingPipelineAI) GenerateSummary(ctx context.Context, _, _, _ string) (string, int, int, error) {
	f.summaryCalls.Add(1)
	f.summaryStarted <- struct{}{}
	select {
	case <-f.releaseSummary:
	case <-ctx.Done():
		return "", 0, 0, ctx.Err()
	}
	return "summary", 1, 1, nil
}

func (f *blockingPipelineAI) GenerateKeyPoints(context.Context, string, string, string) ([]string, int, int, error) {
	return []string{"point"}, 1, 1, nil
}

func (f *blockingPipelineAI) GenerateTasks(context.Context, string, string, string) ([]ai.TaskSuggestion, int, int, error) {
	return []ai.TaskSuggestion{{Description: "task", Priority: "low"}}, 1, 1, nil
}

func TestOrchestrator_ReprocessRejectsDuplicateWhilePipelineIsRunning(t *testing.T) {
	fai := &blockingPipelineAI{summaryStarted: make(chan struct{}, 2), releaseSummary: make(chan struct{})}
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, fai)
	transcript := "original"
	meeting, _ := mr.GetByID(context.Background(), id)
	meeting.Transcript = &transcript
	meeting.Status = models.StatusCompleted
	if err := mr.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}
	if err := orch.Reprocess(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	awaitPipelineSignal(t, fai.summaryStarted)
	if err := orch.Reprocess(context.Background(), id); !errors.Is(err, services.ErrMeetingBusy) {
		close(fai.releaseSummary)
		orch.WaitPipelines()
		t.Fatalf("second Reprocess error = %v, want ErrMeetingBusy", err)
	}

	close(fai.releaseSummary)
	orch.WaitPipelines()
	if got := fai.summaryCalls.Load(); got != 1 {
		t.Fatalf("summary started %d times, want once", got)
	}
}

func TestMeetingService_RejectsGenerationInputEditsWhilePipelineRuns(t *testing.T) {
	fai := &blockingPipelineAI{summaryStarted: make(chan struct{}, 1), releaseSummary: make(chan struct{})}
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, fai)
	transcript := "original"
	meeting, _ := mr.GetByID(context.Background(), id)
	meeting.Transcript = &transcript
	meeting.Status = models.StatusCompleted
	if err := mr.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}
	meetingSvc := services.NewMeetingService(mr, nil, orch.OperationCoordinator())
	if err := orch.Reprocess(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	awaitPipelineSignal(t, fai.summaryStarted)

	editedTranscript := "edited"
	if _, err := meetingSvc.Update(context.Background(), id, "R", nil, "", nil, nil, &editedTranscript, nil); !errors.Is(err, services.ErrMeetingBusy) {
		t.Fatalf("transcript edit error = %v, want ErrMeetingBusy", err)
	}
	editedNotes := "new notes"
	if _, err := meetingSvc.Update(context.Background(), id, "R", nil, "", nil, nil, nil, &editedNotes); !errors.Is(err, services.ErrMeetingBusy) {
		t.Fatalf("notes edit error = %v, want ErrMeetingBusy", err)
	}

	close(fai.releaseSummary)
	orch.WaitPipelines()
}

func TestOrchestrator_ShutdownCancelsAndPreservesRetryableState(t *testing.T) {
	fai := &blockingPipelineAI{summaryStarted: make(chan struct{}, 1), releaseSummary: make(chan struct{})}
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, fai)
	transcript := "original"
	meeting, _ := mr.GetByID(context.Background(), id)
	meeting.Transcript = &transcript
	meeting.Status = models.StatusCompleted
	if err := mr.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}
	if err := orch.Reprocess(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	awaitPipelineSignal(t, fai.summaryStarted)

	if err := orch.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	got, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.StatusFailed || got.Transcript == nil || *got.Transcript != transcript {
		t.Fatalf("shutdown state = status %q transcript %v; want failed with original transcript", got.Status, got.Transcript)
	}
}

func TestOrchestrator_ShutdownFinalizesRecordingForRetry(t *testing.T) {
	wavPath := t.TempDir() + "/shutdown.wav"
	fa := &fakeAudioClient{stopResp: &audio.StopResponse{Path: wavPath, DurationSeconds: 4}}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})
	if err := orch.StartRecording(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	if err := orch.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	got, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.StatusFailed || got.AudioPath == nil || *got.AudioPath != wavPath || !got.KeepAudio {
		t.Fatalf("shutdown state = %+v; want failed meeting with preserved audio", got)
	}
}

func TestOrchestrator_RecoversInterruptedRecordingByPersistedIdentity(t *testing.T) {
	recordingID := "123e4567-e89b-12d3-a456-426614174000"
	wavPath := t.TempDir() + "/recovered.wav"
	fa := &fakeAudioClient{recoverResp: &audio.StopResponse{RecordingID: recordingID, Path: wavPath, DurationSeconds: 4}}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})
	meeting, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	meeting.Status = models.StatusRecording
	meeting.RecordingID = &recordingID
	if err := mr.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}

	if err := orch.RecoverInterruptedMeetings(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.StatusFailed || got.RecordingID != nil || got.AudioPath == nil || *got.AudioPath != wavPath || !got.KeepAudio {
		t.Fatalf("recovered meeting = %+v", got)
	}
}

func TestOrchestrator_CancelledCaptureKeepsRecordingRecoverable(t *testing.T) {
	stopStarted := make(chan struct{})
	releaseStop := make(chan struct{})
	var stopCalls atomic.Int32
	recoveredPath := filepath.Join(t.TempDir(), "recovered.wav")
	if err := os.WriteFile(recoveredPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	fa := &fakeAudioClient{
		stopFn: func(context.Context) (*audio.StopResponse, error) {
			if stopCalls.Add(1) == 1 {
				close(stopStarted)
				<-releaseStop
				return nil, errors.New("stop interrupted")
			}
			return &audio.StopResponse{Path: recoveredPath}, nil
		},
	}
	orch, mr, id := newOrchTest(t, fa, &fakeAI{})
	meeting, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	meeting.Status = models.StatusTranscribing
	if err := mr.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	pipelineDone := make(chan error, 1)
	go func() {
		pipelineDone <- orch.RunCapturePipeline(ctx, id)
	}()
	awaitPipelineSignal(t, stopStarted)
	cancel()
	close(releaseStop)
	if err := <-pipelineDone; err == nil {
		t.Fatal("RunCapturePipeline returned nil after interrupted stop")
	}

	got, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.StatusTranscribing {
		t.Fatalf("status after interrupted stop = %q, want transcribing for recovery", got.Status)
	}
	if err := orch.RecoverInterruptedMeetings(context.Background()); err != nil {
		t.Fatalf("RecoverInterruptedMeetings: %v", err)
	}
	got, err = mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.StatusFailed || got.AudioPath == nil || *got.AudioPath == "" {
		t.Fatalf("recovered state = status %q audio %v; want failed with retryable audio", got.Status, got.AudioPath)
	}
}

func TestOrchestrator_PipelinePreservesMeetingEditsMadeDuringGeneration(t *testing.T) {
	fai := &blockingPipelineAI{summaryStarted: make(chan struct{}, 1), releaseSummary: make(chan struct{})}
	orch, mr, id := newOrchTest(t, &fakeAudioClient{}, fai)
	transcript := "original"
	meeting, _ := mr.GetByID(context.Background(), id)
	meeting.Transcript = &transcript
	meeting.Status = models.StatusCompleted
	if err := mr.Update(context.Background(), meeting); err != nil {
		t.Fatal(err)
	}

	meetingSvc := services.NewMeetingService(mr, nil, orch.OperationCoordinator())
	if err := orch.Reprocess(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	awaitPipelineSignal(t, fai.summaryStarted)

	if _, err := meetingSvc.Update(context.Background(), id, "edited title", nil, "", nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	close(fai.releaseSummary)
	orch.WaitPipelines()
	got, err := mr.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "edited title" || got.Transcript == nil || *got.Transcript != "original" {
		t.Fatalf("pipeline did not preserve the current meeting data: title=%q transcript=%v", got.Title, got.Transcript)
	}
}

func awaitPipelineSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pipeline")
	}
}

var _ ai.AIClient = (*blockingPipelineAI)(nil)
