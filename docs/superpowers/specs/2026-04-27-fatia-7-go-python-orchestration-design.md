# Spec: Fatia 7 — Integração Go ↔ Python (orquestração end-to-end)

## Objetivo

Implementar 4 novos endpoints no backend Go que orquestram o pipeline completo: captura → transcrição → geração AI. Comunicação com o audio-service Python via HTTP. Pipeline assíncrono com estado refletido em `meetings.status`.

---

## Arquitetura

Pacote novo `internal/audio/` com cliente HTTP. Serviço novo `internal/services/orchestrator.go` que coordena `audio.Client` + serviços existentes (MeetingService, SummaryService, KeyPointService, TaskService). Goroutines em background carregam o pipeline; o estado é persistido em `meetings.status` para que o cliente acompanhe via `GET /api/meetings/{id}`.

### File map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `internal/audio/client.go` | criar | Interface `Client` + `httpClient` com baseURL, timeouts, parsing de JSON |
| `internal/audio/client_test.go` | criar | Testes com `httptest.NewServer` |
| `internal/services/orchestrator.go` | criar | `Orchestrator` — pipelines síncronos + variantes goroutine |
| `internal/services/orchestrator_test.go` | criar | Testes com fake `audio.Client` + DB real |
| `internal/handlers/meeting_handler.go` | modificar | 4 novos métodos: Start/Stop/Process/SetTranscript |
| `internal/handlers/meeting_handler_test.go` | modificar | Testes dos 4 endpoints com Orchestrator mockado via interface |
| `cmd/api/main.go` | modificar | Construir audio.Client e Orchestrator; registrar rotas |

---

## Endpoints

```
POST /api/meetings/{id}/start       → 202 Accepted, status = "recording"
                                    → 503 se audio service down ou /health não OK
                                    → 409 se audio service em estado errado
                                    → 422 se meeting já está em "recording"
                                    → 404 se meeting não existe

POST /api/meetings/{id}/stop        → 202 Accepted; goroutine roda RunCapturePipeline
                                    → 422 se meeting não está em "recording"
                                    → 404 se meeting não existe

POST /api/meetings/{id}/process     → 202 Accepted; goroutine roda RunAIPipeline
                                    → 422 se transcript vazio
                                    → 404 se meeting não existe

POST /api/meetings/{id}/transcript  body: {"transcript":"..."}
                                    → 202 Accepted; salva transcript + dispara AI
                                    → 422 se transcript vazio ou status em {recording, transcribing}
                                    → 400 se JSON inválido
                                    → 404 se meeting não existe
```

Cliente acompanha o progresso via `GET /api/meetings/{id}` lendo o campo `status`.

---

## State machine de `meetings.status`

```
pending ──POST /start──► recording ──POST /stop──► transcribing
                                                       │
                            ┌──────────────────────────┤
                            │ (transcribe falhou)      │ (transcribe OK)
                            ▼                          ▼
                          failed                    processing
                                                       │
                            ┌──────────────────────────┤
                            │ (AI falhou)              │ (AI OK)
                            ▼                          ▼
                          failed                    completed
```

**`POST /process`:** requer `status ∈ {failed, completed}` e `transcript` não vazio. Transita `processing` → `completed`/`failed`.

**`POST /transcript`:** requer `status ∉ {recording, transcribing}`. Salva transcript, transita `processing` → `completed`/`failed`.

---

## `internal/audio/client.go`

```go
package audio

import (
    "context"
    "errors"
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
    transcribeClient *http.Client  // 10min timeout
    defaultClient    *http.Client  // 30s timeout
}

func NewHTTPClient(baseURL string) *httpClient
```

**Mapeamento de erros HTTP → sentinel:**
- erro de rede / timeout / connection refused → `ErrAudioServiceUnavailable`
- 409 → `ErrAudioServiceConflict`
- 4xx ou 5xx ou JSON inválido → `ErrAudioGenericError` (com `fmt.Errorf("...: %w", ErrAudioGenericError)` carregando contexto)

---

## `internal/services/orchestrator.go`

```go
type Orchestrator struct {
    repo         *repository.MeetingRepository  // pra atualizar status diretamente
    summarySvc   *SummaryService
    keyPointSvc  *KeyPointService
    taskSvc      *TaskService
    audio        audio.Client
    language     string  // do config (WHISPER_LANGUAGE)
}

func NewOrchestrator(repo, summarySvc, keyPointSvc, taskSvc, audio, language) *Orchestrator
```

**Métodos síncronos (testáveis diretamente):**

```go
// Pré-condição: meeting.Status == "recording"
// Faz: status=transcribing → audio.Stop → audio.Transcribe → save transcript+duration → status=processing → RunAIPipeline
// Cleanup do WAV ao final (best-effort, log warning se falhar)
// Se transcribe falhar antes de chegar à AI: status=failed
// Se AI falhar: status=failed, mas transcript permanece
func (o *Orchestrator) RunCapturePipeline(ctx context.Context, meetingID string) error

// Pré-condição: meeting tem transcript não vazio
// Faz: status=processing → SummaryService.Generate → KeyPointService.Generate → TaskService.Generate → status=completed
// Falha em qualquer um → status=failed
func (o *Orchestrator) RunAIPipeline(ctx context.Context, meetingID string) error
```

**Métodos públicos para handlers (disparam goroutine):**

```go
// Valida pré-condições, transita status=recording, chama audio.StartRecording
// Síncrono — handler retorna 202 só se isso retornar nil
func (o *Orchestrator) StartRecording(ctx context.Context, meetingID string) error

// Valida pré-condições, dispara goroutine que roda RunCapturePipeline com context.Background() + timeout 15min
// Retorna nil imediatamente se goroutine foi disparada
func (o *Orchestrator) StopRecording(ctx context.Context, meetingID string) error

// Valida pré-condições (transcript não vazio), dispara goroutine
func (o *Orchestrator) Reprocess(ctx context.Context, meetingID string) error

// Valida pré-condições, salva transcript, dispara goroutine RunAIPipeline
func (o *Orchestrator) SetTranscriptAndProcess(ctx context.Context, meetingID, transcript string) error
```

**Sentinel errors:**
- `*ValidationError` reutilizado de `theme_service.go` para erros de pré-condição
- Erros do `audio.Client` propagados (handler mapeia para 503/409)

**Sincronização para testes:**
A goroutine despachada por `StopRecording`/`Reprocess`/`SetTranscriptAndProcess` deve ser observável em testes. Solução: `Orchestrator` expõe um campo `pipelineDone chan struct{}` (ou `sync.WaitGroup`) que tests podem aguardar antes de checar o status final. Em produção, o canal é fechado ao final do pipeline; ninguém precisa ler.

Implementação concreta: `pipelineWG sync.WaitGroup`. Tests chamam `orch.WaitPipelines()` para bloquear até todas as goroutines em voo terminarem.

---

## Handlers

`MeetingHandler` ganha um campo `orch *services.Orchestrator` no construtor (consistente com como recebe `*services.MeetingService` etc.).

**Métodos:**

```go
func (h *MeetingHandler) Start(w, r) {
    id := chi.URLParam(r, "id")
    err := h.orch.StartRecording(r.Context(), id)
    // erro mapping:
    //  ErrAudioServiceUnavailable → 503
    //  ErrAudioServiceConflict → 409
    //  *ValidationError → 422
    //  repository.ErrNotFound → 404
    //  default → 500
    w.WriteHeader(http.StatusAccepted)
}

func (h *MeetingHandler) Stop(w, r) {
    err := h.orch.StopRecording(r.Context(), id)
    // *ValidationError → 422; ErrNotFound → 404; default → 500
    w.WriteHeader(http.StatusAccepted)
}

func (h *MeetingHandler) Process(w, r) {
    err := h.orch.Reprocess(r.Context(), id)
    // mesmo mapping de Stop
}

func (h *MeetingHandler) SetTranscript(w, r) {
    var req setTranscriptRequest
    json.NewDecoder(r.Body).Decode(&req)  // 400 se inválido
    err := h.orch.SetTranscriptAndProcess(r.Context(), id, req.Transcript)
    // mesmo mapping de Stop
}
```

DTO:
```go
type setTranscriptRequest struct {
    Transcript string `json:"transcript"`
}
```

---

## Cleanup do WAV

Após `audio.Transcribe()` retornar com sucesso, o orchestrator faz `os.Remove(wavPath)`. Falha em deletar gera log warning mas NÃO marca o meeting como failed (transcript já foi salvo).

Path do WAV é o que `audio.StopRecording()` retornou (relativo ao CWD do audio-service Python). O Go assume que tem acesso de escrita a esse path — os dois serviços rodam na mesma máquina.

---

## Configuração

`internal/config/config.go` já tem `AudioServiceURL` (default `http://localhost:8765`). Sem mudanças.

`cmd/api/main.go` instancia `audio.NewHTTPClient(cfg.AudioServiceURL)` e `services.NewOrchestrator(...)`.

---

## Testes

### `audio/client_test.go`

- `TestClient_Health_OK` — httptest server retorna JSON válido → resposta correta
- `TestClient_Health_NetworkError` — server fechado → `ErrAudioServiceUnavailable`
- `TestClient_StartRecording_OK`
- `TestClient_StartRecording_409` — server retorna 409 → `ErrAudioServiceConflict`
- `TestClient_StopRecording_OK`
- `TestClient_Transcribe_OK` — verifica que body POST inclui `path` e `language` corretos
- `TestClient_Transcribe_500` — `ErrAudioGenericError`
- `TestClient_GenericError_OnInvalidJSON`

### `services/orchestrator_test.go`

`fakeAudioClient` struct implementando `audio.Client` com campos canned + capturas de chamadas. SQLite real.

- `TestRunCapturePipeline_Success` — pipeline completo, status final = completed, transcript salvo, AI Generate chamados
- `TestRunCapturePipeline_TranscribeFails` — fakeAudio.Transcribe retorna erro → status = failed, sem AI calls
- `TestRunCapturePipeline_AIFails` — primeiro Generate retorna erro → status = failed, transcript persiste
- `TestRunAIPipeline_Success` — chama com transcript não vazio → status = completed
- `TestRunAIPipeline_NoTranscript` — `*ValidationError`
- `TestStartRecording_AudioOK` — fakeAudio.StartRecording OK → status = recording
- `TestStartRecording_AudioUnavailable` — fakeAudio retorna ErrAudioServiceUnavailable → erro propagado, status NÃO muda
- `TestStartRecording_AlreadyRecording` — meeting status já = recording → ValidationError
- `TestStopRecording_FiresGoroutine` — dispara, aguarda via WaitGroup, checa status final
- `TestSetTranscriptAndProcess_Empty` — ValidationError
- `TestSetTranscriptAndProcess_Success` — salva transcript + roda AI

### `handlers/meeting_handler_test.go`

Adiciona um fake `OrchestratorIface` (interface extraída no handler) e testa cada endpoint com mock:

- `TestMeetingHandler_Start_Success` (202)
- `TestMeetingHandler_Start_AudioServiceDown` (503)
- `TestMeetingHandler_Start_NotFound` (404)
- `TestMeetingHandler_Stop_Success` (202)
- `TestMeetingHandler_Stop_NotRecording` (422)
- `TestMeetingHandler_Process_Success` (202)
- `TestMeetingHandler_Process_NoTranscript` (422)
- `TestMeetingHandler_SetTranscript_Success` (202)
- `TestMeetingHandler_SetTranscript_EmptyBody` (422)

Para isso, `MeetingHandler` recebe uma interface `Orchestrator` em vez do tipo concreto. A interface lista apenas os 4 métodos públicos (`StartRecording`, `StopRecording`, `Reprocess`, `SetTranscriptAndProcess`).

---

## HTTP Status Codes

| Operação | Sucesso | Erros possíveis |
|---|---|---|
| POST /api/meetings/{id}/start | 202 | 404, 409, 422, 503, 500 |
| POST /api/meetings/{id}/stop | 202 | 404, 422, 500 |
| POST /api/meetings/{id}/process | 202 | 404, 422, 500 |
| POST /api/meetings/{id}/transcript | 202 | 400, 404, 422, 500 |

---

## Decisões de design

- **Pipeline assíncrono via goroutine + status no DB:** estado consultável via `GET /api/meetings/{id}`, evita timeouts HTTP em reuniões longas, permite UI mostrar progresso real, reuniões parciais são recuperáveis via `/process`.
- **AI generation sequencial:** ~30s total. Paralelo economizaria ~20s mas adiciona complexidade de erros parciais. YAGNI.
- **WAV deletado após transcribe OK (independente do resultado de AI):** transcrição é determinística — re-rodar Whisper produz o mesmo texto. Master spec: "Áudio descartado após transcrição".
- **`/transcript` dispara AI automaticamente:** UX simples — usuário tem texto, quer resultado. Quem quer só salvar usa `PUT /api/meetings/{id}`.
- **`/process` re-roda os 3 Generate (overwrite):** os métodos `Generate` dos services já são idempotentes (Upsert pra summary, DeleteByMeetingID + Create para key_points/tasks).
- **Orchestrator expõe `WaitPipelines()` (sync.WaitGroup):** elimina flakiness em testes async.
- **Goroutines usam `context.Background()` + timeout 15min:** pipeline não fica atrelado ao request HTTP que disparou (que retorna 202 imediatamente).
- **Audio service unavailable mid-pipeline:** status = failed; usuário re-tenta com `/process` (que só faz AI) ou regrava (se a transcrição não foi salva).
- **Stuck meetings em `recording`/`transcribing`/`processing` após crash do servidor Go:** NÃO tratado nesta fatia. Documentado como limitação conhecida; usuário usa `/process` ou edita via PUT manualmente. Recovery automático fica para fatia futura se necessário.
- **Sem auth:** localhost, single-user, igual às demais fatias.
