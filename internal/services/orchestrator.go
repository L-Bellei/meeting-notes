package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"meeting-notes/internal/ai"
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
	logRepo      *repository.LogRepository
	operations   *MeetingOperationCoordinator
	pipelineCtx  context.Context
	cancel       context.CancelFunc
	shutdownOnce sync.Once
	shutdownErr  error
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
	coordinators ...*MeetingOperationCoordinator,
) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	operations := NewMeetingOperationCoordinator()
	if len(coordinators) > 0 && coordinators[0] != nil {
		operations = coordinators[0]
	}
	return &Orchestrator{
		repo:         repo,
		themeRepo:    themeRepo,
		summarySvc:   summarySvc,
		keyPointSvc:  keyPointSvc,
		taskSvc:      taskSvc,
		boardCardSvc: boardCardSvc,
		audio:        audioClient,
		settings:     settings,
		operations:   operations,
		pipelineCtx:  ctx,
		cancel:       cancel,
	}
}

func (o *Orchestrator) SetParentContext(ctx context.Context) {
	o.cancel()
	o.pipelineCtx, o.cancel = context.WithCancel(ctx)
}

func (o *Orchestrator) OperationCoordinator() *MeetingOperationCoordinator {
	return o.operations
}

func (o *Orchestrator) SetNotifyFn(fn func(meetingID, status string)) {
	o.notifyFn = fn
}

func (o *Orchestrator) SetLogRepo(repo *repository.LogRepository) {
	o.logRepo = repo
}

func (o *Orchestrator) persistLog(level, component, message string) {
	if o.logRepo == nil {
		return
	}
	_ = o.logRepo.Insert(context.Background(), level, component, message, nil)
}

func (o *Orchestrator) notify(meetingID string, status models.MeetingStatus) {
	if o.notifyFn != nil {
		o.notifyFn(meetingID, string(status))
	}
}

func (o *Orchestrator) WaitPipelines() {
	o.pipelineWG.Wait()
}

func (o *Orchestrator) Shutdown() error {
	o.shutdownOnce.Do(func() {
		o.operations.Close()
		o.cancel()
		o.pipelineWG.Wait()
		o.shutdownErr = o.RecoverInterruptedMeetings(context.Background())
	})
	return o.shutdownErr
}

func (o *Orchestrator) RecoverInterruptedMeetings(ctx context.Context) error {
	for _, status := range []models.MeetingStatus{
		models.StatusRecording,
		models.StatusTranscribing,
		models.StatusProcessing,
		models.StatusFailed,
	} {
		meetings, err := o.repo.List(ctx, repository.ListFilters{Status: string(status)})
		if err != nil {
			return fmt.Errorf("list interrupted meetings: %w", err)
		}
		for i := range meetings {
			meeting := &meetings[i]
			var stopErr error
			needsRecordingRecovery := (status == models.StatusRecording || status == models.StatusTranscribing) &&
				(meeting.RecordingID != nil || meeting.AudioPath == nil || *meeting.AudioPath == "")
			if status == models.StatusFailed && meeting.RecordingID != nil {
				needsRecordingRecovery = true
			}
			if needsRecordingRecovery {
				stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				var stopResp *audio.StopResponse
				var err error
				if meeting.RecordingID != nil {
					stopResp, err = o.audio.RecoverRecording(stopCtx, *meeting.RecordingID)
				} else {
					stopResp, err = o.audio.StopRecording(stopCtx)
				}
				cancel()
				if err != nil {
					stopErr = err
				} else if stopResp == nil || stopResp.Path == "" {
					stopErr = errors.New("audio service returned no recording path")
				} else {
					keepAudio := true
					if meeting.RecordingID != nil && stopResp.RecordingID != *meeting.RecordingID {
						return fmt.Errorf("recovered recording identity mismatch for meeting %s", meeting.ID)
					}
					if meeting.RecordingID != nil {
						err = o.repo.SetRecordingResult(ctx, meeting.ID, *meeting.RecordingID, &stopResp.Path, keepAudio)
					} else {
						err = o.repo.SetAudioPath(ctx, meeting.ID, &stopResp.Path, &keepAudio)
					}
					if err != nil {
						return fmt.Errorf("preserve interrupted recording %s: %w", meeting.ID, err)
					}
				}
			}
			message := "processamento interrompido pelo encerramento do aplicativo"
			if stopErr != nil {
				message = fmt.Sprintf("%s; falha ao preservar áudio: %v", message, stopErr)
			}
			if status == models.StatusFailed && !needsRecordingRecovery {
				continue
			}
			if err := o.repo.SetStatus(ctx, meeting.ID, models.StatusFailed, &message); err != nil {
				return fmt.Errorf("mark interrupted meeting %s: %w", meeting.ID, err)
			}
			o.notify(meeting.ID, models.StatusFailed)
		}
	}
	return nil
}

func isMeetingOperationStatus(status models.MeetingStatus) bool {
	return status == models.StatusRecording || status == models.StatusTranscribing || status == models.StatusProcessing
}

func mapTransitionError(err error) error {
	if errors.Is(err, repository.ErrConflict) {
		return ErrMeetingBusy
	}
	return err
}

func (o *Orchestrator) StartRecording(ctx context.Context, meetingID string) error {
	if err := o.operations.Begin(meetingID, "recording"); err != nil {
		return err
	}
	release := true
	defer func() {
		if release {
			o.operations.End(meetingID)
		}
	}()
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if isMeetingOperationStatus(m.Status) {
		return ErrMeetingBusy
	}
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.TransitionStatus(ctx, meetingID, []models.MeetingStatus{m.Status}, models.StatusRecording, nil, nil)
	}); err != nil {
		return mapTransitionError(err)
	}
	startResp, err := o.audio.StartRecording(ctx)
	if err != nil {
		if rollbackErr := o.repo.TransitionStatus(ctx, meetingID, []models.MeetingStatus{models.StatusRecording}, m.Status, m.ErrorMessage, nil); rollbackErr != nil {
			return fmt.Errorf("start recording: %v; restore meeting status: %w", err, rollbackErr)
		}
		return err
	}
	if startResp == nil || startResp.RecordingID == "" {
		release = false
		return fmt.Errorf("audio service started recording without a recording id")
	}
	recordingID := startResp.RecordingID
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	err = o.withMeetingWrite(meetingID, func() error {
		return o.repo.SetRecordingID(persistCtx, meetingID, &recordingID)
	})
	persistCancel()
	if err != nil {
		release = false
		return fmt.Errorf("persist recording identity: %w", err)
	}
	m.Status = models.StatusRecording
	m.RecordingID = &recordingID
	o.notify(m.ID, m.Status)
	release = false
	return nil
}

func (o *Orchestrator) StopRecording(ctx context.Context, meetingID string, keepAudio bool) error {
	if err := o.operations.BeginStopRecording(meetingID); err != nil {
		return err
	}
	release := true
	defer func() {
		if release {
			o.operations.End(meetingID)
		}
	}()
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Status != models.StatusRecording {
		return &ValidationError{"meeting is not recording"}
	}
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.TransitionStatus(ctx, meetingID, []models.MeetingStatus{models.StatusRecording}, models.StatusTranscribing, nil, &keepAudio)
	}); err != nil {
		return mapTransitionError(err)
	}
	o.notify(meetingID, models.StatusTranscribing)
	o.spawnReservedPipeline(meetingID, o.RunCapturePipeline)
	release = false
	return nil
}

func (o *Orchestrator) Reprocess(ctx context.Context, meetingID string) error {
	if err := o.operations.Begin(meetingID, "pipeline"); err != nil {
		return err
	}
	release := true
	defer func() {
		if release {
			o.operations.End(meetingID)
		}
	}()
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if isMeetingOperationStatus(m.Status) {
		return ErrMeetingBusy
	}
	if m.Transcript == nil || *m.Transcript == "" {
		return &ValidationError{"transcript is required for processing"}
	}
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.TransitionStatus(ctx, meetingID, []models.MeetingStatus{m.Status}, models.StatusProcessing, nil, nil)
	}); err != nil {
		return mapTransitionError(err)
	}
	o.notify(meetingID, models.StatusProcessing)
	o.spawnReservedPipeline(meetingID, o.RunAIPipeline)
	release = false
	return nil
}

func (o *Orchestrator) SetTranscriptAndProcess(ctx context.Context, meetingID, transcript string) error {
	if transcript == "" {
		return &ValidationError{"transcript is required"}
	}
	if err := o.operations.Begin(meetingID, "pipeline"); err != nil {
		return err
	}
	release := true
	defer func() {
		if release {
			o.operations.End(meetingID)
		}
	}()
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if isMeetingOperationStatus(m.Status) {
		return ErrMeetingBusy
	}
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.SetTranscriptAndStatus(ctx, meetingID, []models.MeetingStatus{m.Status}, transcript, models.StatusProcessing, nil)
	}); err != nil {
		return mapTransitionError(err)
	}
	o.notify(meetingID, models.StatusProcessing)
	o.spawnReservedPipeline(meetingID, o.RunAIPipeline)
	release = false
	return nil
}

func (o *Orchestrator) spawnReservedPipeline(meetingID string, fn func(context.Context, string) error) {
	o.pipelineWG.Add(1)
	go func() {
		defer o.pipelineWG.Done()
		defer o.operations.End(meetingID)
		bgCtx, cancel := context.WithTimeout(o.pipelineCtx, pipelineTimeout)
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

	if m.Status == models.StatusRecording {
		if err := o.withMeetingWrite(meetingID, func() error {
			return o.repo.TransitionStatus(ctx, meetingID, []models.MeetingStatus{models.StatusRecording}, models.StatusTranscribing, nil, nil)
		}); err != nil {
			return mapTransitionError(err)
		}
	} else if m.Status != models.StatusTranscribing {
		return ErrMeetingBusy
	}

	stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	stopResp, err := o.audio.StopRecording(stopCtx)
	stopCancel()
	if err != nil {
		if ctx.Err() == nil {
			o.markFailed(ctx, m, fmt.Sprintf("falha ao parar gravação: %v", err))
		}
		return err
	}
	if stopResp == nil || stopResp.Path == "" {
		err := fmt.Errorf("audio service returned no recording path")
		o.markFailed(ctx, m, err.Error())
		return err
	}
	if m.RecordingID != nil && stopResp.RecordingID != *m.RecordingID {
		err := fmt.Errorf("audio service returned recording %q, want %q", stopResp.RecordingID, *m.RecordingID)
		o.markFailed(ctx, m, err.Error())
		return err
	}

	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	if err := o.withMeetingWrite(meetingID, func() error {
		if m.RecordingID != nil {
			return o.repo.SetRecordingResult(persistCtx, meetingID, *m.RecordingID, &stopResp.Path, m.KeepAudio)
		}
		return o.repo.SetAudioPath(persistCtx, meetingID, &stopResp.Path, nil)
	}); err != nil {
		persistCancel()
		return err
	}
	persistCancel()
	m.AudioPath = &stopResp.Path
	m.RecordingID = nil

	if err := CheckModelLoaded(ctx, o.audio); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}
	if err := ValidateWAVFile(stopResp.Path); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}

	whisperLang := ""
	whisperDevice := "auto"
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		whisperLang = s["whisper_language"]
		if d := s["whisper_device"]; d != "" {
			whisperDevice = d
		}
	}
	trResp, err := o.audio.Transcribe(ctx, stopResp.Path, whisperLang, whisperDevice)
	if err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("transcrição falhou: %v", err))
		return err
	}

	dur := int(stopResp.DurationSeconds)
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.SetTranscriptionResult(ctx, meetingID, trResp.Transcript, trResp.Language, &dur)
	}); err != nil {
		return err
	}
	m.Transcript = &trResp.Transcript
	m.Language = &trResp.Language
	m.DurationSeconds = &dur
	m.Status = models.StatusProcessing
	m.ErrorMessage = nil
	o.notify(m.ID, models.StatusProcessing)

	if !m.KeepAudio {
		if err := o.clearAudioReferenceAndRemove(ctx, m, stopResp.Path); err != nil {
			return err
		}
	}

	if err := o.maybeGenerate(ctx, m); err != nil {
		return err
	}

	if err := o.setStatus(ctx, meetingID, models.StatusCompleted, nil); err != nil {
		return err
	}
	o.notify(m.ID, models.StatusCompleted)
	return nil
}

// maybeGenerate runs AI generation when auto_generate is on and the AI is
// configured. When the AI is not configured it skips generation gracefully
// (the meeting keeps its transcript and completes) instead of failing.
func (o *Orchestrator) maybeGenerate(ctx context.Context, m *models.Meeting) error {
	s, _ := o.settings.GetAll(ctx)
	autoGen := s["auto_generate"] != "false"
	if autoGen && !ai.Configured(s) {
		o.persistLog("warn", "orchestrator", "auto-generate pulado: IA não configurada")
		autoGen = false
	}
	if !autoGen {
		return nil
	}
	if err := o.runAIGeneration(ctx, m); err != nil {
		o.markFailed(ctx, m, friendlyAIError(err))
		return err
	}
	return nil
}

// friendlyAIError converts a raw generation error into a user-facing message.
func friendlyAIError(err error) string {
	if ai.IsAuthError(err) {
		return "Chave de IA inválida ou expirada — verifique em Configurações → IA"
	}
	return fmt.Sprintf("geração de IA falhou: %v", err)
}

func (o *Orchestrator) RunAIPipeline(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.Transcript == nil || *m.Transcript == "" {
		return &ValidationError{"transcript is required"}
	}

	if err := o.setStatus(ctx, meetingID, models.StatusProcessing, nil); err != nil {
		return err
	}
	o.notify(m.ID, models.StatusProcessing)

	if err := o.runAIGeneration(ctx, m); err != nil {
		o.markFailed(ctx, m, friendlyAIError(err))
		return err
	}

	if err := o.setStatus(ctx, meetingID, models.StatusCompleted, nil); err != nil {
		return err
	}
	o.notify(m.ID, models.StatusCompleted)
	return nil
}

func (o *Orchestrator) runAIGeneration(ctx context.Context, m *models.Meeting) error {
	var theme *models.Theme
	if m.ThemeID != nil {
		if t, err := o.themeRepo.GetByID(ctx, *m.ThemeID); err == nil {
			theme = t
		}
	}
	customPrompt := ""
	if theme != nil {
		customPrompt = theme.CustomPrompt
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
	return nil
}

func (o *Orchestrator) markFailed(ctx context.Context, m *models.Meeting, errMsg string) {
	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	message := errMsg
	if err := o.withMeetingWrite(m.ID, func() error {
		return o.repo.SetStatus(persistCtx, m.ID, models.StatusFailed, &message)
	}); err != nil {
		log.Printf("warning: mark failed %s: %v", m.ID, err)
		o.persistLog("warn", "orchestrator", fmt.Sprintf("mark failed %s: %v", m.ID, err))
		return
	}
	m.Status = models.StatusFailed
	m.ErrorMessage = &message
	o.notify(m.ID, models.StatusFailed)
}

func (o *Orchestrator) withMeetingWrite(meetingID string, fn func() error) error {
	return o.operations.WithMeetingLock(meetingID, func(string) error {
		return fn()
	})
}

func (o *Orchestrator) clearAudioReferenceAndRemove(ctx context.Context, m *models.Meeting, audioPath string) error {
	if err := o.withMeetingWrite(m.ID, func() error {
		return o.repo.SetAudioPath(ctx, m.ID, nil, nil)
	}); err != nil {
		return err
	}
	m.AudioPath = nil
	if err := os.Remove(audioPath); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: delete WAV %s: %v", audioPath, err)
	}
	return nil
}

func (o *Orchestrator) setStatus(ctx context.Context, meetingID string, status models.MeetingStatus, errorMessage *string) error {
	return o.withMeetingWrite(meetingID, func() error {
		return o.repo.SetStatus(ctx, meetingID, status, errorMessage)
	})
}

func (o *Orchestrator) RetranscribeRecording(ctx context.Context, meetingID string) error {
	if err := o.operations.Begin(meetingID, "pipeline"); err != nil {
		return err
	}
	release := true
	defer func() {
		if release {
			o.operations.End(meetingID)
		}
	}()
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.AudioPath == nil || *m.AudioPath == "" {
		return &ValidationError{Message: "nenhum arquivo de áudio disponível para transcrição"}
	}
	if isMeetingOperationStatus(m.Status) {
		return ErrMeetingBusy
	}
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.TransitionStatus(ctx, meetingID, []models.MeetingStatus{m.Status}, models.StatusTranscribing, nil, nil)
	}); err != nil {
		return mapTransitionError(err)
	}
	o.notify(meetingID, models.StatusTranscribing)
	o.spawnReservedPipeline(meetingID, o.RunRetranscribePipeline)
	release = false
	return nil
}

func (o *Orchestrator) RunRetranscribePipeline(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.AudioPath == nil {
		return &ValidationError{Message: "audio_path is nil"}
	}
	audioPath := *m.AudioPath

	if err := o.setStatus(ctx, meetingID, models.StatusTranscribing, nil); err != nil {
		return err
	}
	m.Status = models.StatusTranscribing
	m.ErrorMessage = nil
	o.notify(m.ID, models.StatusTranscribing)

	if err := CheckModelLoaded(ctx, o.audio); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}
	if err := ValidateWAVFile(audioPath); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}

	whisperLang := ""
	whisperDevice := "auto"
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		whisperLang = s["whisper_language"]
		if d := s["whisper_device"]; d != "" {
			whisperDevice = d
		}
	}
	trResp, err := o.audio.Transcribe(ctx, audioPath, whisperLang, whisperDevice)
	if err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("transcrição falhou: %v", err))
		return err
	}

	duration := m.DurationSeconds
	if trResp.DurationSeconds > 0 {
		dur := int(trResp.DurationSeconds)
		duration = &dur
	}
	if err := o.withMeetingWrite(meetingID, func() error {
		return o.repo.SetTranscriptionResult(ctx, meetingID, trResp.Transcript, trResp.Language, duration)
	}); err != nil {
		return err
	}
	m.Transcript = &trResp.Transcript
	m.Language = &trResp.Language
	m.DurationSeconds = duration
	m.Status = models.StatusProcessing
	m.ErrorMessage = nil
	o.notify(m.ID, models.StatusProcessing)

	if !m.KeepAudio {
		if err := o.clearAudioReferenceAndRemove(ctx, m, audioPath); err != nil {
			return err
		}
	}

	if err := o.maybeGenerate(ctx, m); err != nil {
		return err
	}

	if err := o.setStatus(ctx, meetingID, models.StatusCompleted, nil); err != nil {
		return err
	}
	o.notify(m.ID, models.StatusCompleted)
	return nil
}
