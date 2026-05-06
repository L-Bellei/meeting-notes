package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"meeting-notes/internal/audio"
	"meeting-notes/internal/models"
	"meeting-notes/internal/repository"
)

const pipelineTimeout = 90 * time.Minute

type orchestratorSettings interface {
	GetAll(ctx context.Context) (map[string]string, error)
}

type Orchestrator struct {
	repo         *repository.MeetingRepository
	themeRepo    *repository.ThemeRepository
	summarySvc   *SummaryService
	keyPointSvc  *KeyPointService
	taskSvc      *TaskService
	boardCardSvc *BoardCardService
	audio        audio.Client
	settings     orchestratorSettings
	pipelineWG   sync.WaitGroup
	notifyFn     func(meetingID, status string)
	searchRepo   *repository.SearchRepository
	logRepo      *repository.LogRepository
}

func NewOrchestrator(
	repo *repository.MeetingRepository,
	themeRepo *repository.ThemeRepository,
	summarySvc *SummaryService,
	keyPointSvc *KeyPointService,
	taskSvc *TaskService,
	audioClient audio.Client,
	settings orchestratorSettings,
	boardCardSvc *BoardCardService,
) *Orchestrator {
	return &Orchestrator{
		repo:         repo,
		themeRepo:    themeRepo,
		summarySvc:   summarySvc,
		keyPointSvc:  keyPointSvc,
		taskSvc:      taskSvc,
		boardCardSvc: boardCardSvc,
		audio:        audioClient,
		settings:     settings,
	}
}

func (o *Orchestrator) SetNotifyFn(fn func(meetingID, status string)) {
	o.notifyFn = fn
}

func (o *Orchestrator) SetSearchRepo(repo *repository.SearchRepository) {
	o.searchRepo = repo
}

func (o *Orchestrator) SetLogRepo(repo *repository.LogRepository) {
	o.logRepo = repo
}

func (o *Orchestrator) persistLog(level, component, message string) {
	if o.logRepo == nil {
		return
	}
	go func() {
		_ = o.logRepo.Insert(context.Background(), level, component, message, nil)
	}()
}

func (o *Orchestrator) notify(meetingID string, status models.MeetingStatus) {
	if o.notifyFn != nil {
		o.notifyFn(meetingID, string(status))
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
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}
	o.notify(m.ID, m.Status)
	return nil
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
			o.persistLog("error", "orchestrator", fmt.Sprintf("pipeline %s failed: %v", meetingID, err))
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
	o.notify(m.ID, m.Status)

	stopResp, err := o.audio.StopRecording(ctx)
	if err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("falha ao parar gravação: %v", err))
		return err
	}

	// Persist audio path immediately — before any failure that might follow
	m.AudioPath = &stopResp.Path
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}

	if err := CheckModelLoaded(ctx, o.audio); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}
	if err := ValidateWAVFile(stopResp.Path); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}

	whisperLang := "pt"
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		if v := s["whisper_language"]; v != "" {
			whisperLang = v
		}
	}
	trResp, err := o.audio.Transcribe(ctx, stopResp.Path, whisperLang)
	if err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("transcrição falhou: %v", err))
		return err
	}

	m.Transcript = &trResp.Transcript
	dur := int(stopResp.DurationSeconds)
	m.DurationSeconds = &dur
	m.Status = models.StatusProcessing
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}
	o.notify(m.ID, m.Status)

	keepAudio := false
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		keepAudio = s["keep_audio"] == "true"
	}
	if !keepAudio {
		if err := os.Remove(stopResp.Path); err != nil && !os.IsNotExist(err) {
			log.Printf("warning: delete WAV %s: %v", stopResp.Path, err)
		} else {
			m.AudioPath = nil
			_ = o.repo.Update(ctx, m)
		}
	}

	autoGen := true
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		autoGen = s["auto_generate"] != "false"
	}
	if autoGen {
		if err := o.runAIGeneration(ctx, m); err != nil {
			o.markFailed(ctx, m, fmt.Sprintf("geração de IA falhou: %v", err))
			return err
		}
	}

	m.Status = models.StatusCompleted
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}
	o.notify(m.ID, m.Status)
	return nil
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
	o.notify(m.ID, m.Status)

	if err := o.runAIGeneration(ctx, m); err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("geração de IA falhou: %v", err))
		return err
	}

	m.Status = models.StatusCompleted
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}
	o.notify(m.ID, m.Status)
	return nil
}

func (o *Orchestrator) runAIGeneration(ctx context.Context, m *models.Meeting) error {
	customPrompt := ""
	var theme *models.Theme
	if m.ThemeID != nil {
		if t, err := o.themeRepo.GetByID(ctx, *m.ThemeID); err == nil {
			theme = t
			customPrompt = t.CustomPrompt
		}
	}
	if _, err := o.summarySvc.Generate(ctx, m, customPrompt); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	if _, err := o.keyPointSvc.Generate(ctx, m, customPrompt); err != nil {
		return fmt.Errorf("key_points: %w", err)
	}
	if _, err := o.taskSvc.Generate(ctx, m, customPrompt); err != nil {
		return fmt.Errorf("tasks: %w", err)
	}
	if theme != nil && o.boardCardSvc != nil && theme.AutoAddToBoard {
		if _, err := o.boardCardSvc.Create(ctx, m.ID, ""); err != nil {
			log.Printf("auto-add to board %s: %v", m.ID, err)
			o.persistLog("warn", "orchestrator", fmt.Sprintf("auto-add board %s: %v", m.ID, err))
		}
	}
	if o.searchRepo != nil {
		go func() {
			bgCtx := context.Background()
			transcript := ""
			if m.Transcript != nil {
				transcript = *m.Transcript
			}
			summary := ""
			if sm, err2 := o.summarySvc.Get(bgCtx, m.ID); err2 == nil && sm != nil {
				summary = sm.Content
			}
			kps, _ := o.keyPointSvc.List(bgCtx, m.ID)
			var kpContents []string
			for _, kp := range kps {
				kpContents = append(kpContents, kp.Content)
			}
			tasks, _ := o.taskSvc.List(bgCtx, m.ID)
			var taskContents []string
			for _, tk := range tasks {
				taskContents = append(taskContents, tk.Description)
			}
			_ = o.searchRepo.UpsertMeeting(bgCtx, m.ID, m.Title, transcript, summary,
				strings.Join(kpContents, "\n"), strings.Join(taskContents, "\n"))
		}()
	}
	return nil
}

func (o *Orchestrator) markFailed(ctx context.Context, m *models.Meeting, errMsg string) {
	m.Status = models.StatusFailed
	if errMsg != "" {
		m.ErrorMessage = &errMsg
	}
	if err := o.repo.Update(ctx, m); err != nil {
		log.Printf("warning: mark failed %s: %v", m.ID, err)
		o.persistLog("warn", "orchestrator", fmt.Sprintf("mark failed %s: %v", m.ID, err))
		return
	}
	o.notify(m.ID, m.Status)
}
