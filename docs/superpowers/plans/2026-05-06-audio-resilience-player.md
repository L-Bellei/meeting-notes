# Audio Resilience, Player & Transcription Diagnostics — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tornar o pipeline de transcrição resiliente (nunca perder o WAV, sempre permitir retry), exibir erros reais na UI, e adicionar player de áudio flutuante com visualizador de espectro.

**Architecture:** Approach A — WAV permanece no dir do audio-service. O path é persistido em `meetings.audio_path` imediatamente após `StopRecording`. Falhas na transcrição nunca deletam o WAV. O Go backend serve o arquivo via `GET /api/meetings/{id}/audio`. O frontend usa um widget flutuante (`fixed bottom-4 right-4`) com Web Audio API para o espectro.

**Tech Stack:** Go 1.22, chi v5, modernc/sqlite, React 19 + TypeScript, Tailwind CSS, Web Audio API, Canvas API

---

## File Map

| Arquivo | Ação |
|---|---|
| `audio-service/transcriber.py` | Modificar — remover `vad_filter` |
| `internal/database/migrations/011_audio_fields.sql` | Criar |
| `internal/database/migrations/012_keep_audio_setting.sql` | Criar |
| `internal/models/models.go` | Modificar — `AudioPath`, `ErrorMessage` em `Meeting` |
| `internal/repository/meeting_repository.go` | Modificar — queries e scan |
| `internal/services/transcription_checks.go` | Criar |
| `internal/services/transcription_checks_test.go` | Criar |
| `internal/services/orchestrator.go` | Modificar — `markFailed`, `RunCapturePipeline`, `RunAIPipeline`, `RetranscribeRecording`, `RunRetranscribePipeline` |
| `internal/handlers/audio_serve_handler.go` | Criar |
| `internal/handlers/retranscribe_handler.go` | Criar |
| `internal/handlers/retranscribe_handler_test.go` | Criar |
| `cmd/desktop/app.go` | Modificar — registrar rotas |
| `cmd/api/main.go` | Modificar — registrar rotas |
| `frontend/src/hooks/useApi.ts` | Modificar — exportar `getApiBase` |
| `frontend/src/hooks/useMeetings.ts` | Modificar — `Meeting` type |
| `frontend/src/hooks/useSettings.ts` | Modificar — `Settings` type |
| `frontend/src/hooks/useMeeting.ts` | Modificar — `useRetranscribe` hook |
| `frontend/src/components/layout/MeetingDetail.tsx` | Modificar — ícone, erro, retry |
| `frontend/src/components/ui/AudioSpectrumVisualizer.tsx` | Criar |
| `frontend/src/components/ui/AudioPlayer.tsx` | Criar |

---

## Task 1 — Fix: remover `vad_filter` do transcriber.py

**Files:**
- Modify: `audio-service/transcriber.py`

O `vad_filter=True` falha no bundle PyInstaller porque os arquivos do modelo Silero VAD não estão incluídos no `.spec`. Remover é a correção imediata; os demais parâmetros (`condition_on_previous_text=False`, `repetition_penalty=1.1`, `compression_ratio_threshold=1.8`) são mantidos.

- [ ] **Step 1: Remover `vad_filter` do dict de parâmetros**

Em `audio-service/transcriber.py`, localizar o bloco `transcribe_kwargs` e substituir:

```python
        transcribe_kwargs = dict(
            language=lang,
            # Prevents hallucination feedback loops: each 30s chunk is decoded
            # independently, so a bad segment can't poison subsequent ones.
            condition_on_previous_text=False,
            # Discard segments that are already highly repetitive internally.
            compression_ratio_threshold=1.8,
            # Small penalty for token repetition within a segment.
            repetition_penalty=1.1,
        )
```

(Remover as linhas `vad_filter=True,` e o comentário associado.)

- [ ] **Step 2: Rebuild PyInstaller**

```powershell
cd F:\dev\meeting-notes\audio-service
.venv\Scripts\python.exe -m PyInstaller build\pyinstaller\audio-service.spec --distpath build\dist --workpath build\work --noconfirm
```

Esperado: "Build complete!"

- [ ] **Step 3: Commit**

```bash
git add audio-service/transcriber.py
git commit -m "fix: remove vad_filter from transcriber (fails in PyInstaller bundle)"
```

---

## Task 2 — DB Migrations 011 e 012

**Files:**
- Create: `internal/database/migrations/011_audio_fields.sql`
- Create: `internal/database/migrations/012_keep_audio_setting.sql`

- [ ] **Step 1: Criar migration 011**

```sql
-- internal/database/migrations/011_audio_fields.sql
ALTER TABLE meetings ADD COLUMN audio_path TEXT;
ALTER TABLE meetings ADD COLUMN error_message TEXT;
```

- [ ] **Step 2: Criar migration 012**

```sql
-- internal/database/migrations/012_keep_audio_setting.sql
INSERT OR IGNORE INTO settings (key, value) VALUES ('keep_audio', 'false');
```

- [ ] **Step 3: Verificar que as migrations são embedadas e aplicadas**

```bash
cd F:/dev/meeting-notes
go test ./internal/database/... -v
```

Esperado: PASS (o sistema de migrations já aplica automaticamente todos os arquivos `*.sql` embedados).

- [ ] **Step 4: Commit**

```bash
git add internal/database/migrations/011_audio_fields.sql internal/database/migrations/012_keep_audio_setting.sql
git commit -m "feat: add audio_path, error_message to meetings; keep_audio setting (migrations 011-012)"
```

---

## Task 3 — Atualizar models Go + repository

**Files:**
- Modify: `internal/models/models.go`
- Modify: `internal/repository/meeting_repository.go`

- [ ] **Step 1: Adicionar campos ao struct Meeting**

Em `internal/models/models.go`, substituir o struct `Meeting`:

```go
type Meeting struct {
	ID              string        `json:"id"`
	ThemeID         *string       `json:"theme_id"`
	Title           string        `json:"title"`
	StartedAt       *time.Time    `json:"started_at"`
	DurationSeconds *int          `json:"duration_seconds"`
	Status          MeetingStatus `json:"status"`
	Transcript      *string       `json:"transcript"`
	Notes           *string       `json:"notes"`
	AudioPath       *string       `json:"audio_path"`
	ErrorMessage    *string       `json:"error_message"`
	CreatedAt       time.Time     `json:"created_at"`
}
```

- [ ] **Step 2: Atualizar query de List**

Em `internal/repository/meeting_repository.go`, substituir:

```go
query := `SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, created_at FROM meetings`
```

por:

```go
query := `SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, audio_path, error_message, created_at FROM meetings`
```

- [ ] **Step 3: Atualizar query de GetByID**

Substituir:

```go
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, created_at FROM meetings WHERE id = ?`, id,
	)
```

por:

```go
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, audio_path, error_message, created_at FROM meetings WHERE id = ?`, id,
	)
```

- [ ] **Step 4: Atualizar query de GetRecording**

Substituir:

```go
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, created_at
		 FROM meetings WHERE status = 'recording' LIMIT 1`,
	)
```

por:

```go
	row := r.db.QueryRowContext(ctx,
		`SELECT id, theme_id, title, started_at, duration_seconds, status, transcript, notes, audio_path, error_message, created_at
		 FROM meetings WHERE status = 'recording' LIMIT 1`,
	)
```

- [ ] **Step 5: Atualizar scanMeeting**

Substituir a função `scanMeeting` inteira:

```go
func scanMeeting(row meetingScanner) (*models.Meeting, error) {
	var m models.Meeting
	var themeID sql.NullString
	var startedAt sql.NullString
	var duration sql.NullInt64
	var transcript sql.NullString
	var notes sql.NullString
	var audioPath sql.NullString
	var errorMessage sql.NullString
	var createdAt string
	var status string

	err := row.Scan(&m.ID, &themeID, &m.Title, &startedAt, &duration, &status,
		&transcript, &notes, &audioPath, &errorMessage, &createdAt)
	if err != nil {
		return nil, err
	}

	if themeID.Valid {
		v := themeID.String
		m.ThemeID = &v
	}
	if startedAt.Valid {
		t, err := parseTime(startedAt.String)
		if err != nil {
			return nil, err
		}
		m.StartedAt = &t
	}
	if duration.Valid {
		d := int(duration.Int64)
		m.DurationSeconds = &d
	}
	if transcript.Valid {
		v := transcript.String
		m.Transcript = &v
	}
	if notes.Valid {
		v := notes.String
		m.Notes = &v
	}
	if audioPath.Valid {
		v := audioPath.String
		m.AudioPath = &v
	}
	if errorMessage.Valid {
		v := errorMessage.String
		m.ErrorMessage = &v
	}
	m.Status = models.MeetingStatus(status)
	if m.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	return &m, nil
}
```

- [ ] **Step 6: Atualizar Update**

Substituir a query de Update:

```go
	result, err := r.db.ExecContext(ctx,
		`UPDATE meetings SET theme_id = ?, title = ?, started_at = ?, duration_seconds = ?, status = ?, transcript = ?, notes = ?, audio_path = ?, error_message = ? WHERE id = ?`,
		m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript, m.Notes, m.AudioPath, m.ErrorMessage, m.ID,
	)
```

- [ ] **Step 7: Verificar compilação e testes**

```bash
cd F:/dev/meeting-notes
go build ./...
go test ./internal/repository/... -v
```

Esperado: compilação sem erros, testes PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/models/models.go internal/repository/meeting_repository.go
git commit -m "feat: add AudioPath, ErrorMessage to Meeting model and repository"
```

---

## Task 4 — Pre-flight checks de transcrição

**Files:**
- Create: `internal/services/transcription_checks.go`
- Create: `internal/services/transcription_checks_test.go`

- [ ] **Step 1: Escrever os testes (TDD)**

Criar `internal/services/transcription_checks_test.go`:

```go
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

// stub audio client
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
```

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
cd F:/dev/meeting-notes
go test ./internal/services/ -run "TestCheckModel|TestValidateWAV" -v
```

Esperado: FAIL com "undefined: services.CheckModelLoaded"

- [ ] **Step 3: Criar `internal/services/transcription_checks.go`**

```go
package services

import (
	"context"
	"fmt"
	"os"

	"meeting-notes/internal/audio"
)

func CheckModelLoaded(ctx context.Context, client audio.Client) error {
	h, err := client.Health(ctx)
	if err != nil {
		return fmt.Errorf("serviço de áudio indisponível: %w", err)
	}
	if !h.ModelLoaded {
		return fmt.Errorf("modelo de transcrição ainda está carregando, tente novamente em alguns segundos")
	}
	return nil
}

func ValidateWAVFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("arquivo de áudio não encontrado: %s", path)
		}
		return fmt.Errorf("erro ao verificar arquivo de áudio: %w", err)
	}
	const minBytes = 10 * 1024
	if info.Size() < minBytes {
		return fmt.Errorf("arquivo de áudio muito pequeno (%d bytes), a gravação pode estar vazia", info.Size())
	}
	return nil
}
```

- [ ] **Step 4: Rodar os testes para confirmar PASS**

```bash
go test ./internal/services/ -run "TestCheckModel|TestValidateWAV" -v
```

Esperado: 5 testes PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/transcription_checks.go internal/services/transcription_checks_test.go
git commit -m "feat: add transcription pre-flight checks (model health + WAV validation)"
```

---

## Task 5 — Atualizar orchestrator: markFailed, RunCapturePipeline, RunAIPipeline

**Files:**
- Modify: `internal/services/orchestrator.go`

- [ ] **Step 1: Atualizar `markFailed` para aceitar mensagem de erro**

Substituir o método `markFailed`:

```go
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
```

- [ ] **Step 2: Substituir `RunCapturePipeline` completo**

```go
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
```

- [ ] **Step 3: Atualizar `RunAIPipeline` — call site do markFailed**

Substituir a linha `o.markFailed(ctx, m)` em `RunAIPipeline`:

```go
	if err := o.runAIGeneration(ctx, m); err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("geração de IA falhou: %v", err))
		return err
	}
```

- [ ] **Step 4: Build + testes**

```bash
cd F:/dev/meeting-notes
go build ./...
go test ./internal/... -v
```

Esperado: sem erros de compilação, todos os testes PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/services/orchestrator.go
git commit -m "feat: store audio_path, pre-flight checks, error_message in capture pipeline"
```

---

## Task 6 — Adicionar RetranscribeRecording ao orchestrator

**Files:**
- Modify: `internal/services/orchestrator.go`

- [ ] **Step 1: Adicionar `RetranscribeRecording` e `RunRetranscribePipeline` ao orchestrator**

Adicionar ao final de `internal/services/orchestrator.go`:

```go
func (o *Orchestrator) RetranscribeRecording(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.AudioPath == nil || *m.AudioPath == "" {
		return &ValidationError{"nenhum arquivo de áudio disponível para transcrição"}
	}
	if m.Status == models.StatusRecording || m.Status == models.StatusTranscribing || m.Status == models.StatusProcessing {
		return &ValidationError{"a reunião já está sendo processada"}
	}
	o.spawnPipeline(meetingID, o.RunRetranscribePipeline)
	return nil
}

func (o *Orchestrator) RunRetranscribePipeline(ctx context.Context, meetingID string) error {
	m, err := o.repo.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.AudioPath == nil {
		return &ValidationError{"audio_path is nil"}
	}
	audioPath := *m.AudioPath

	m.Status = models.StatusTranscribing
	m.ErrorMessage = nil
	if err := o.repo.Update(ctx, m); err != nil {
		return err
	}
	o.notify(m.ID, m.Status)

	if err := CheckModelLoaded(ctx, o.audio); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}
	if err := ValidateWAVFile(audioPath); err != nil {
		o.markFailed(ctx, m, err.Error())
		return err
	}

	whisperLang := "pt"
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		if v := s["whisper_language"]; v != "" {
			whisperLang = v
		}
	}
	trResp, err := o.audio.Transcribe(ctx, audioPath, whisperLang)
	if err != nil {
		o.markFailed(ctx, m, fmt.Sprintf("transcrição falhou: %v", err))
		return err
	}

	m.Transcript = &trResp.Transcript
	if trResp.DurationSeconds > 0 {
		dur := int(trResp.DurationSeconds)
		m.DurationSeconds = &dur
	}
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
		if err := os.Remove(audioPath); err != nil && !os.IsNotExist(err) {
			log.Printf("warning: delete WAV %s: %v", audioPath, err)
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
```

- [ ] **Step 2: Build**

```bash
cd F:/dev/meeting-notes
go build ./...
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
git add internal/services/orchestrator.go
git commit -m "feat: add RetranscribeRecording and RunRetranscribePipeline to orchestrator"
```

---

## Task 7 — Handlers de áudio + retry + rotas

**Files:**
- Create: `internal/handlers/audio_serve_handler.go`
- Create: `internal/handlers/retranscribe_handler.go`
- Create: `internal/handlers/retranscribe_handler_test.go`
- Modify: `cmd/desktop/app.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Escrever testes do RetranscribeHandler**

Criar `internal/handlers/retranscribe_handler_test.go`:

```go
package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/services"
)

type mockRetranscribeOrch struct {
	err error
}

func (m *mockRetranscribeOrch) RetranscribeRecording(_ context.Context, _ string) error {
	return m.err
}

func newRetranscribeRequest(meetingID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/meetings/"+meetingID+"/retranscribe", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", meetingID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	return r
}

func TestRetranscribeHandler_Success(t *testing.T) {
	h := handlers.NewRetranscribeHandler(&mockRetranscribeOrch{})
	w := httptest.NewRecorder()
	h.Retranscribe(w, newRetranscribeRequest("meet-1"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", w.Code)
	}
}

func TestRetranscribeHandler_NoAudioPath(t *testing.T) {
	h := handlers.NewRetranscribeHandler(&mockRetranscribeOrch{
		err: &services.ValidationError{Message: "nenhum arquivo de áudio disponível para transcrição"},
	})
	w := httptest.NewRecorder()
	h.Retranscribe(w, newRetranscribeRequest("meet-1"))
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Rodar testes para confirmar falha**

```bash
cd F:/dev/meeting-notes
go test ./internal/handlers/ -run TestRetranscribe -v
```

Esperado: FAIL com "undefined: handlers.NewRetranscribeHandler"

- [ ] **Step 3: Criar `internal/handlers/retranscribe_handler.go`**

```go
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type retranscribeOrchestrator interface {
	RetranscribeRecording(ctx context.Context, meetingID string) error
}

type RetranscribeHandler struct {
	orch retranscribeOrchestrator
}

func NewRetranscribeHandler(orch retranscribeOrchestrator) *RetranscribeHandler {
	return &RetranscribeHandler{orch: orch}
}

func (h *RetranscribeHandler) Retranscribe(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.orch.RetranscribeRecording(r.Context(), id); err != nil {
		var ve *services.ValidationError
		if errors.As(err, &ve) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reunião não encontrada"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "transcribing"})
}
```

- [ ] **Step 4: Rodar testes para confirmar PASS**

```bash
go test ./internal/handlers/ -run TestRetranscribe -v
```

Esperado: 2 testes PASS.

- [ ] **Step 5: Criar `internal/handlers/audio_serve_handler.go`**

```go
package handlers

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"meeting-notes/internal/repository"
)

type audioMeetingRepo interface {
	GetByID(ctx interface{ Deadline() (interface{}, bool); Done() <-chan struct{}; Err() error; Value(interface{}) interface{} }, id string) (interface{ GetAudioPath() *string }, error)
}

type AudioServeHandler struct {
	meetingRepo *repository.MeetingRepository
}

func NewAudioServeHandler(repo *repository.MeetingRepository) *AudioServeHandler {
	return &AudioServeHandler{meetingRepo: repo}
}

func (h *AudioServeHandler) ServeAudio(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.meetingRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			http.Error(w, "meeting not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if m.AudioPath == nil || *m.AudioPath == "" {
		http.Error(w, "no audio file", http.StatusNotFound)
		return
	}
	f, err := os.Open(*m.AudioPath)
	if err != nil {
		http.Error(w, "audio file not found on disk", http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "audio.wav", info.ModTime(), f)
}
```

- [ ] **Step 6: Registrar rotas em `cmd/desktop/app.go`**

Logo após a linha `r.Get("/api/ai/health", aiHealthHandler.Check)`, adicionar:

```go
	audioServeHandler := handlers.NewAudioServeHandler(meetingRepo)
	retranscribeHandler := handlers.NewRetranscribeHandler(orch)
	r.Get("/api/meetings/{id}/audio", audioServeHandler.ServeAudio)
	r.Post("/api/meetings/{id}/retranscribe", retranscribeHandler.Retranscribe)
```

- [ ] **Step 7: Registrar rotas em `cmd/api/main.go`**

Logo após a linha `r.Get("/api/ai/health", aiHealthHandler.Check)`, adicionar:

```go
	audioServeHandler := handlers.NewAudioServeHandler(meetingRepo)
	retranscribeHandler := handlers.NewRetranscribeHandler(orchestrator)
	r.Get("/api/meetings/{id}/audio", audioServeHandler.ServeAudio)
	r.Post("/api/meetings/{id}/retranscribe", retranscribeHandler.Retranscribe)
```

- [ ] **Step 8: Build + testes**

```bash
cd F:/dev/meeting-notes
go build ./...
go test ./internal/... -v
```

Esperado: sem erros, todos os testes PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/handlers/audio_serve_handler.go internal/handlers/retranscribe_handler.go internal/handlers/retranscribe_handler_test.go cmd/desktop/app.go cmd/api/main.go
git commit -m "feat: add GET /api/meetings/{id}/audio and POST /api/meetings/{id}/retranscribe"
```

---

## Task 8 — Frontend: types + getApiBase + useRetranscribe

**Files:**
- Modify: `frontend/src/hooks/useApi.ts`
- Modify: `frontend/src/hooks/useMeetings.ts`
- Modify: `frontend/src/hooks/useSettings.ts`
- Modify: `frontend/src/hooks/useMeeting.ts`

- [ ] **Step 1: Exportar `getApiBase` em useApi.ts**

Em `frontend/src/hooks/useApi.ts`, adicionar após `let baseURL = ""`:

```ts
export function getApiBase(): string {
  return baseURL
}
```

- [ ] **Step 2: Adicionar `audio_path` e `error_message` ao tipo Meeting**

Em `frontend/src/hooks/useMeetings.ts`, substituir a interface `Meeting`:

```ts
export interface Meeting {
  id: string
  theme_id: string | null
  title: string
  started_at: string | null
  duration_seconds: number | null
  status: "pending" | "recording" | "transcribing" | "processing" | "completed" | "failed"
  transcript: string | null
  notes: string | null
  audio_path: string | null
  error_message: string | null
  created_at: string
}
```

- [ ] **Step 3: Adicionar `keep_audio` ao tipo Settings**

Em `frontend/src/hooks/useSettings.ts`, adicionar `keep_audio: string` à interface `Settings`:

```ts
export interface Settings {
  user_name: string
  ai_provider: "anthropic" | "openai"
  anthropic_api_key: string
  anthropic_model: string
  openai_api_key: string
  openai_model: string
  auto_generate: string
  whisper_language: string
  whisper_model: string
  recording_hotkey: string
  meeting_name_template: string
  keep_audio: string
}
```

- [ ] **Step 4: Adicionar `useRetranscribe` ao useMeeting.ts**

Em `frontend/src/hooks/useMeeting.ts`, adicionar ao final do arquivo:

```ts
export function useRetranscribe(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api(`/api/meetings/${meetingId}/retranscribe`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}
```

- [ ] **Step 5: Verificar TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/hooks/useApi.ts frontend/src/hooks/useMeetings.ts frontend/src/hooks/useSettings.ts frontend/src/hooks/useMeeting.ts
git commit -m "feat: add audio_path/error_message to Meeting type, keep_audio to Settings, useRetranscribe hook"
```

---

## Task 9 — Frontend: SettingsModal toggle keep_audio

**Files:**
- Modify: `frontend/src/components/settings/SettingsModal.tsx`

- [ ] **Step 1: Ler o arquivo atual para entender onde adicionar**

Ler `frontend/src/components/settings/SettingsModal.tsx` completo para identificar onde ficam as seções de settings (Transcrição, IA, etc.).

- [ ] **Step 2: Adicionar seção "Áudio" com toggle keep_audio**

Localizar a última seção de settings no render do modal (normalmente antes do botão de fechar ou no final do formulário) e adicionar:

```tsx
{/* Seção Áudio */}
<div className="space-y-3">
  <h3 className="text-sm font-semibold text-foreground/70 uppercase tracking-wider">Áudio</h3>
  <label className="flex items-start gap-3 cursor-pointer group">
    <div className="relative mt-0.5 flex-shrink-0">
      <input
        type="checkbox"
        className="sr-only peer"
        checked={form.keep_audio === "true"}
        onChange={e => setForm(f => ({ ...f, keep_audio: e.target.checked ? "true" : "false" }))}
      />
      <div className="w-9 h-5 rounded-full bg-input peer-checked:bg-primary transition-colors" />
      <div className="absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform peer-checked:translate-x-4 shadow" />
    </div>
    <div>
      <p className="text-sm font-medium leading-none">Guardar áudio das reuniões</p>
      <p className="text-xs text-muted-foreground mt-1">
        Preserva o arquivo de áudio após transcrição para reprodução posterior.
        Desativado: áudio é deletado após transcrição bem-sucedida.
      </p>
    </div>
  </label>
</div>
```

Observação: o componente usa um `form` local (state) para edição. Localizar onde `form` é inicializado a partir de `settings` e certificar-se que `keep_audio` está incluído com fallback `"false"`:

```tsx
const [form, setForm] = useState<Settings>({
  ...settings,
  keep_audio: settings?.keep_audio ?? "false",
  // ... outros campos
})
```

- [ ] **Step 3: Verificar TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/settings/SettingsModal.tsx
git commit -m "feat: add keep_audio toggle to SettingsModal"
```

---

## Task 10 — Frontend: MeetingDetail — ícone de áudio, erro e retry

**Files:**
- Modify: `frontend/src/components/layout/MeetingDetail.tsx`

- [ ] **Step 1: Ler o arquivo completo**

Ler `frontend/src/components/layout/MeetingDetail.tsx` inteiro para mapear exatamente onde está o header e os botões de ação.

- [ ] **Step 2: Adicionar import de Volume2 + useRetranscribe + useState para player**

No topo do arquivo, adicionar `Volume2` aos imports do lucide-react e importar `useRetranscribe`:

```tsx
import { Play, Square, RefreshCw, Wand2, Trash2, Volume2 } from "lucide-react"
import {
  useMeeting, useUpdateMeeting, useStartRecording, useStopRecording,
  useReprocess, useGenerateSummary, useGenerateKeyPoints, useGenerateTasks,
  useUpdateTask, useRetranscribe,
} from "../../hooks/useMeeting"
```

E dentro do componente `MeetingDetail`, adicionar estado:

```tsx
const [playerOpen, setPlayerOpen] = useState(false)
const retranscribe = useRetranscribe(meetingId ?? "")
```

- [ ] **Step 3: Adicionar ícone de áudio no header**

No header da reunião, localizar onde estão os botões de ação (Start/Stop) e adicionar — logo antes deles — o botão de áudio, visível somente quando `meeting.audio_path` não é nulo:

```tsx
{meeting.audio_path && (
  <Button
    variant="ghost"
    size="icon"
    className={cn("h-8 w-8", playerOpen && "text-primary")}
    title="Reproduzir áudio da reunião"
    onClick={() => setPlayerOpen(o => !o)}
  >
    <Volume2 className="h-4 w-4" />
  </Button>
)}
```

- [ ] **Step 4: Adicionar exibição de error_message**

Logo após o header (antes das tabs), adicionar:

```tsx
{meeting.status === "failed" && meeting.error_message && (
  <div className="mx-4 mt-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
    <span className="font-semibold">Erro: </span>{meeting.error_message}
  </div>
)}
```

- [ ] **Step 5: Adicionar botão "Tentar transcrever novamente"**

No bloco que exibe os botões de ação para `status === "failed"`, adicionar o botão de retry de transcrição:

```tsx
{meeting.status === "failed" && meeting.audio_path && (
  <Button
    variant="outline"
    size="sm"
    disabled={retranscribe.isPending}
    onClick={() => retranscribe.mutate()}
    className="gap-1.5"
  >
    {retranscribe.isPending
      ? <Spinner size={14} />
      : <RefreshCw className="h-3.5 w-3.5" />}
    Tentar transcrever novamente
  </Button>
)}
```

- [ ] **Step 6: Montar AudioPlayer quando playerOpen = true**

No return do componente, antes do `</div>` final, adicionar:

```tsx
{playerOpen && meeting.audio_path && meetingId && (
  <AudioPlayer
    meetingId={meetingId}
    meetingTitle={meeting.title}
    onClose={() => setPlayerOpen(false)}
  />
)}
```

E adicionar o import no topo:

```tsx
import { AudioPlayer } from "../ui/AudioPlayer"
```

- [ ] **Step 7: Verificar TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros (AudioPlayer ainda não existe — esperar erro de module not found; aceitável nesta etapa).

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/layout/MeetingDetail.tsx
git commit -m "feat: audio icon, error_message display and retry button in MeetingDetail"
```

---

## Task 11 — Frontend: AudioSpectrumVisualizer

**Files:**
- Create: `frontend/src/components/ui/AudioSpectrumVisualizer.tsx`

- [ ] **Step 1: Criar o componente**

```tsx
import { useRef, useEffect, RefObject } from "react"

interface Props {
  audioRef: RefObject<HTMLAudioElement>
  playing: boolean
}

export function AudioSpectrumVisualizer({ audioRef, playing }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const ctxRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const rafRef = useRef(0)

  // Wire up Web Audio API once
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || ctxRef.current) return
    const audioCtx = new AudioContext()
    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 64 // 32 frequency buckets
    const source = audioCtx.createMediaElementSource(audio)
    source.connect(analyser)
    analyser.connect(audioCtx.destination)
    ctxRef.current = audioCtx
    analyserRef.current = analyser
  }, [audioRef])

  // Draw loop
  useEffect(() => {
    const canvas = canvasRef.current
    const analyser = analyserRef.current
    if (!canvas || !analyser) return

    if (!playing) {
      cancelAnimationFrame(rafRef.current)
      // Clear to flat bars
      const g = canvas.getContext("2d")
      if (g) {
        g.clearRect(0, 0, canvas.width, canvas.height)
        const barW = 4
        const gap = 3
        const count = Math.floor(canvas.width / (barW + gap))
        g.fillStyle = "rgba(99,102,241,0.25)"
        for (let i = 0; i < count; i++) {
          g.beginPath()
          g.roundRect(i * (barW + gap), canvas.height - 4, barW, 4, 2)
          g.fill()
        }
      }
      return
    }

    ctxRef.current?.resume()

    const draw = () => {
      if (!analyser || !canvas) return
      const g = canvas.getContext("2d")
      if (!g) return

      const data = new Uint8Array(analyser.frequencyBinCount)
      analyser.getByteFrequencyData(data)

      g.clearRect(0, 0, canvas.width, canvas.height)

      const barW = 4
      const gap = 3
      const count = Math.min(data.length, Math.floor(canvas.width / (barW + gap)))

      for (let i = 0; i < count; i++) {
        const ratio = data[i] / 255
        const h = Math.max(4, ratio * canvas.height)
        const y = canvas.height - h

        const grad = g.createLinearGradient(0, y, 0, canvas.height)
        grad.addColorStop(0, "rgba(129,140,248,0.9)")
        grad.addColorStop(1, "rgba(79,70,229,0.9)")
        g.fillStyle = grad

        g.beginPath()
        g.roundRect(i * (barW + gap), y, barW, h, 2)
        g.fill()
      }

      rafRef.current = requestAnimationFrame(draw)
    }

    draw()
    return () => cancelAnimationFrame(rafRef.current)
  }, [playing])

  return (
    <canvas
      ref={canvasRef}
      width={180}
      height={32}
      className="w-full"
      style={{ height: 32 }}
    />
  )
}
```

- [ ] **Step 2: Verificar TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ui/AudioSpectrumVisualizer.tsx
git commit -m "feat: add AudioSpectrumVisualizer component (Web Audio API + Canvas)"
```

---

## Task 12 — Frontend: AudioPlayer (widget flutuante)

**Files:**
- Create: `frontend/src/components/ui/AudioPlayer.tsx`

- [ ] **Step 1: Criar o componente**

```tsx
import { useRef, useState, useEffect, useCallback } from "react"
import { X, Play, Pause } from "lucide-react"
import { getApiBase } from "../../hooks/useApi"
import { AudioSpectrumVisualizer } from "./AudioSpectrumVisualizer"
import { cn } from "../../lib/utils"

interface Props {
  meetingId: string
  meetingTitle: string
  onClose: () => void
}

function formatTime(s: number): string {
  if (!isFinite(s)) return "0:00"
  const m = Math.floor(s / 60)
  const sec = Math.floor(s % 60)
  return `${m}:${sec.toString().padStart(2, "0")}`
}

export function AudioPlayer({ meetingId, meetingTitle, onClose }: Props) {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)

  const src = `${getApiBase()}/api/meetings/${meetingId}/audio`

  const toggle = useCallback(() => {
    const a = audioRef.current
    if (!a) return
    if (playing) { a.pause() } else { a.play() }
  }, [playing])

  const skip = useCallback((delta: number) => {
    const a = audioRef.current
    if (!a) return
    a.currentTime = Math.max(0, Math.min(a.duration || 0, a.currentTime + delta))
  }, [])

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const a = audioRef.current
    if (!a || !a.duration) return
    const rect = e.currentTarget.getBoundingClientRect()
    const ratio = (e.clientX - rect.left) / rect.width
    a.currentTime = ratio * a.duration
  }, [])

  const progress = duration > 0 ? currentTime / duration : 0

  return (
    <div className="fixed bottom-4 right-4 z-40 w-52 rounded-2xl border border-border bg-card shadow-2xl shadow-black/50 p-4 select-none">
      {/* hidden audio element */}
      <audio
        ref={audioRef}
        src={src}
        onTimeUpdate={() => setCurrentTime(audioRef.current?.currentTime ?? 0)}
        onLoadedMetadata={() => setDuration(audioRef.current?.duration ?? 0)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => setPlaying(false)}
      />

      {/* close */}
      <button
        onClick={onClose}
        className="absolute top-3 right-3 text-muted-foreground hover:text-foreground transition-colors"
      >
        <X className="h-3.5 w-3.5" />
      </button>

      {/* meeting name */}
      <p className="text-[11px] text-muted-foreground font-medium mb-3 pr-4 truncate">
        {meetingTitle}
      </p>

      {/* seek bar */}
      <div
        className="h-1 w-full rounded-full bg-muted cursor-pointer mb-1.5 relative"
        onClick={seek}
      >
        <div
          className="h-full rounded-full bg-primary transition-none"
          style={{ width: `${progress * 100}%` }}
        />
        <div
          className="absolute top-1/2 -translate-y-1/2 w-3 h-3 rounded-full bg-primary/80 shadow -ml-1.5"
          style={{ left: `${progress * 100}%` }}
        />
      </div>
      <div className="flex justify-between text-[10px] text-muted-foreground mb-3">
        <span>{formatTime(currentTime)}</span>
        <span>{formatTime(duration)}</span>
      </div>

      {/* controls — centered */}
      <div className="flex items-center justify-center gap-3 mb-3">
        <button
          onClick={() => skip(-15)}
          className="text-[11px] font-semibold text-muted-foreground hover:text-foreground bg-muted px-2 py-1 rounded-md transition-colors"
        >
          −15s
        </button>
        <button
          onClick={toggle}
          className={cn(
            "w-9 h-9 rounded-full bg-primary flex items-center justify-center text-primary-foreground shadow-md",
            "hover:bg-primary/90 transition-colors"
          )}
        >
          {playing
            ? <Pause className="h-4 w-4" />
            : <Play className="h-4 w-4 ml-0.5" />}
        </button>
        <button
          onClick={() => skip(15)}
          className="text-[11px] font-semibold text-muted-foreground hover:text-foreground bg-muted px-2 py-1 rounded-md transition-colors"
        >
          +15s
        </button>
      </div>

      {/* spectrum */}
      <div className="border-t border-border/50 pt-3">
        <AudioSpectrumVisualizer audioRef={audioRef} playing={playing} />
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verificar TypeScript**

```bash
cd F:/dev/meeting-notes/frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 3: Testar visualmente com `wails dev`**

```bash
cd F:/dev/meeting-notes/cmd/desktop
wails dev
```

Verificar:
- Reunião com `status=failed` mostra mensagem de erro real (não genérica)
- Reunião com `status=failed` e `audio_path != null` mostra botão "Tentar transcrever novamente"
- Clicar no botão dispara retranscrição (status muda para `transcribing`)
- Reunião com `audio_path != null` mostra ícone 🔊 no header
- Clicar no ícone abre o widget flutuante no canto inferior direito
- Player tem seek bar clicável, botões −15s/▶/+15s centralizados
- Espectro de barras anima ao tocar
- Botão X fecha o widget
- SettingsModal tem toggle "Guardar áudio das reuniões"

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/ui/AudioPlayer.tsx
git commit -m "feat: add AudioPlayer floating widget with spectrum visualizer"
```

---

## Verificação Final

```bash
# Go
cd F:/dev/meeting-notes
go test ./internal/...

# TypeScript
cd frontend && npx tsc --noEmit

# Build desktop
cd ../cmd/desktop && go build ./...
```

Rebuild do installer v2.2.5 após validação completa:
```powershell
cd F:\dev\meeting-notes\audio-service
.venv\Scripts\python.exe -m PyInstaller build\pyinstaller\audio-service.spec --distpath build\dist --workpath build\work --noconfirm

$env:PATH += ";C:\Program Files (x86)\NSIS"
cd F:\dev\meeting-notes
.\build.ps1 -Version 2.2.5
```
