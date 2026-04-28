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
