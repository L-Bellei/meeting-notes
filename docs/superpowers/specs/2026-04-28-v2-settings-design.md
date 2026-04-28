# Settings Feature — Design Spec (v2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to configure their name, AI provider (Anthropic or OpenAI), API keys, models, auto-generation preference, and Whisper transcription settings through a settings modal in the app toolbar — without editing `.env` files.

**Architecture:** Settings are stored in a SQLite `settings` table (key/value). A `SettingsRepository` and `SettingsService` expose `GET /settings` and `PUT /settings` REST endpoints. The existing static `AnthropicClient` is replaced by a `DynamicAIClient` that reads provider/key/model from the settings table at each AI call. A new `OpenAIClient` implements the same `AIClient` interface for GPT models. On the frontend, a `SettingsModal` opens from a gear icon in the `Toolbar`.

**Tech Stack:** Go (backend), React + React Query (frontend), SQLite (storage), Anthropic SDK (existing), OpenAI Go SDK (new)

---

## Settings Keys

All stored as `TEXT` in `settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`:

| Key | Default | Description |
|---|---|---|
| `user_name` | `""` | User's display name |
| `ai_provider` | `"anthropic"` | Active provider: `"anthropic"` or `"openai"` |
| `anthropic_api_key` | `""` | Anthropic API key |
| `anthropic_model` | `"claude-sonnet-4-6"` | Anthropic model ID |
| `openai_api_key` | `""` | OpenAI API key |
| `openai_model` | `"gpt-4o"` | OpenAI model ID |
| `auto_generate` | `"false"` | Auto-generate summary/points/tasks after recording |
| `whisper_language` | `"pt"` | Language code passed to Whisper (`pt`, `en`, `es`, `auto`) |
| `whisper_model` | `"medium"` | Whisper model size (`tiny`, `base`, `small`, `medium`, `large`) |

---

## Backend

### Migration: `internal/database/migrations/005_settings.sql`

```sql
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('user_name',          ''),
    ('ai_provider',        'anthropic'),
    ('anthropic_api_key',  ''),
    ('anthropic_model',    'claude-sonnet-4-6'),
    ('openai_api_key',     ''),
    ('openai_model',       'gpt-4o'),
    ('auto_generate',      'false'),
    ('whisper_language',   'pt'),
    ('whisper_model',      'medium');
```

### `internal/repository/settings_repository.go`

```go
type SettingsRepository interface {
    GetAll(ctx context.Context) (map[string]string, error)
    Set(ctx context.Context, key, value string) error
}
```

- `GetAll` scans all rows into a `map[string]string`
- `Set` does `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`
- Sentinel error: `ErrSettingNotFound` for unknown keys (used for validation in service)

### `internal/services/settings_service.go`

```go
type SettingsService struct { repo SettingsRepository }

func (s *SettingsService) GetAll(ctx context.Context) (map[string]string, error)
func (s *SettingsService) Update(ctx context.Context, updates map[string]string) error
```

`Update` validates:
- `ai_provider` must be `"anthropic"` or `"openai"`
- `whisper_language` must be one of `"pt"`, `"en"`, `"es"`, `"auto"`
- `whisper_model` must be one of `"tiny"`, `"base"`, `"small"`, `"medium"`, `"large"`
- `anthropic_model` must be one of `"claude-sonnet-4-6"`, `"claude-opus-4-7"`, `"claude-haiku-4-5"`
- `openai_model` must be one of `"gpt-4o"`, `"gpt-4o-mini"`, `"gpt-4-turbo"`
- `auto_generate` must be `"true"` or `"false"`
- Unknown keys are rejected with a 400 error

### `internal/handlers/settings_handler.go`

```
GET  /settings       → returns full settings map as JSON object
PUT  /settings       → accepts partial map, updates only provided keys
```

Request/response shape (both directions):
```json
{
  "user_name": "Leonardo Bellei",
  "ai_provider": "anthropic",
  "anthropic_api_key": "sk-ant-...",
  "anthropic_model": "claude-sonnet-4-6",
  "openai_api_key": "",
  "openai_model": "gpt-4o",
  "auto_generate": "false",
  "whisper_language": "pt",
  "whisper_model": "medium"
}
```

### `internal/ai/openai_client.go`

New `OpenAIClient` struct implementing `AIClient` interface, using `github.com/openai/openai-go` SDK. Prompts are identical to `AnthropicClient` — same JSON response shapes expected.

```go
type OpenAIClient struct {
    client openai.Client
    model  string
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient
```

### `internal/ai/dynamic_client.go`

Replaces the static `AnthropicClient` initialized at startup. Reads settings from DB on every AI call to pick the right provider, key, and model.

```go
type DynamicAIClient struct {
    settings SettingsRepository
}

func NewDynamicAIClient(settings SettingsRepository) *DynamicAIClient

func (d *DynamicAIClient) GenerateSummary(ctx, transcript, notes string) (...)
// delegates to AnthropicClient or OpenAIClient based on settings
```

Internal helper:
```go
func (d *DynamicAIClient) resolve(ctx context.Context) (AIClient, error)
// reads GetAll, checks ai_provider, constructs AnthropicClient or OpenAIClient
```

### `internal/audio/client.go` — update `Transcribe` signature

```go
Transcribe(ctx context.Context, path, language, model string) (*TranscribeResponse, error)
```

The `model` parameter is included in the JSON body sent to the Python audio service: `{"path": "...", "language": "pt", "model": "medium"}`. The Python service uses it as an optional override (falls back to its configured default if omitted or empty).

### `internal/services/orchestrator.go` — remove static `language` field

Instead of storing `language` at construction time, the orchestrator receives a `SettingsRepository` and reads `whisper_language` and `whisper_model` from settings at transcription time:

```go
type Orchestrator struct {
    // remove: language string
    // add:
    settings repository.SettingsRepository
    // ... rest unchanged
}
```

At transcription call:
```go
all, _ := o.settings.GetAll(ctx)
trResp, err := o.audio.Transcribe(ctx, stopResp.Path, all["whisper_language"], all["whisper_model"])
```

### `cmd/api/main.go` — wiring changes

```go
settingsRepo := repository.NewSettingsRepository(db)
settingsSvc  := services.NewSettingsService(settingsRepo)
settingsH    := handlers.NewSettingsHandler(settingsSvc)

aiClient     := ai.NewDynamicAIClient(settingsRepo)  // replaces AnthropicClient

orchestrator := services.NewOrchestrator(meetingRepo, summarySvc, keyPointSvc, taskSvc, audioClient, settingsRepo)
```

Route: `router.HandleFunc("GET /settings", settingsH.Get)` and `router.HandleFunc("PUT /settings", settingsH.Update)`.

---

## Frontend

### `frontend/src/hooks/useSettings.ts`

```ts
export interface Settings {
  user_name: string
  ai_provider: "anthropic" | "openai"
  anthropic_api_key: string
  anthropic_model: string
  openai_api_key: string
  openai_model: string
  auto_generate: string   // "true" | "false"
  whisper_language: string
  whisper_model: string
}

export function useSettings(): UseQueryResult<Settings>
export function useUpdateSettings(): UseMutationResult<Settings, Error, Partial<Settings>>
```

`useUpdateSettings` sends `PUT /settings` with only the changed keys.

### `frontend/src/components/settings/SettingsModal.tsx`

Modal triggered from Toolbar. Three sections:

**Perfil**
- Nome do usuário — text input, placeholder: `"Seu nome completo (ex: Leonardo Bellei)"`

**Inteligência Artificial**
- Provedor — toggle button group: Anthropic / OpenAI
- Chave de API — password input (type="password" with show/hide toggle), placeholder changes per provider:
  - Anthropic: `"sk-ant-api03-..."`, hint: `"Obtenha em console.anthropic.com → API Keys"`
  - OpenAI: `"sk-proj-..."`, hint: `"Obtenha em platform.openai.com → API Keys"`
- Modelo — select, options change per provider:
  - Anthropic: `claude-sonnet-4-6`, `claude-opus-4-7`, `claude-haiku-4-5`
  - OpenAI: `gpt-4o`, `gpt-4o-mini`, `gpt-4-turbo`
- Auto-gerar ao terminar gravação — toggle switch (boolean stored as `"true"`/`"false"`)

**Transcrição (Whisper)**
- Idioma do áudio — select: `pt`, `en`, `es`, `auto`
- Modelo Whisper — select: `tiny`, `base`, `small`, `medium`, `large`

On "Salvar": calls `useUpdateSettings` with the full form state. Closes modal on success.

### `frontend/src/components/layout/Toolbar.tsx`

Add a `Settings` (gear) icon button on the right side of the toolbar. Clicking it opens `SettingsModal`. The modal state (`open`/`closed`) is local to `Toolbar` or lifted to `AppInner` if needed.

### `frontend/src/components/recording/RecordingModal.tsx` — auto-generate

After a recording is processed (status becomes `"completed"`), check the `auto_generate` setting:

```ts
const { data: settings } = useSettings()

// when meeting status transitions to "completed":
useEffect(() => {
  if (meeting?.status === "completed" && settings?.auto_generate === "true") {
    generateSummary.mutate(meeting.id)
    generateKeyPoints.mutate(meeting.id)
    generateTasks.mutate(meeting.id)
  }
}, [meeting?.status])
```

---

## Files to Create / Modify

| File | Action |
|---|---|
| `internal/database/migrations/005_settings.sql` | Create |
| `internal/repository/settings_repository.go` | Create |
| `internal/services/settings_service.go` | Create |
| `internal/handlers/settings_handler.go` | Create |
| `internal/ai/openai_client.go` | Create |
| `internal/ai/dynamic_client.go` | Create |
| `internal/audio/client.go` | Modify — add `model` param to `Transcribe` |
| `internal/services/orchestrator.go` | Modify — use `SettingsRepository` for whisper params |
| `cmd/api/main.go` | Modify — wire new repos, handlers, DynamicAIClient |
| `frontend/src/hooks/useSettings.ts` | Create |
| `frontend/src/components/settings/SettingsModal.tsx` | Create |
| `frontend/src/components/layout/Toolbar.tsx` | Modify — add gear icon + SettingsModal |
| `frontend/src/components/recording/RecordingModal.tsx` | Modify — auto-generate on completion |

---

## Verification

```bash
cd F:/dev/meeting-notes && go test ./internal/...   # all tests pass
cd F:/dev/meeting-notes/frontend && npm run build   # no TS errors
```

Manual checks:
- [ ] Gear icon aparece na toolbar
- [ ] Modal abre com valores atuais carregados
- [ ] Trocar de Anthropic → OpenAI atualiza placeholder da API key e opções do modelo
- [ ] Salvar persiste no banco (verificar com `sqlite3 %AppData%\Meeting Notes\meeting-notes.db "SELECT * FROM settings"`)
- [ ] Gerar resumo com provider Anthropic funciona
- [ ] Trocar para OpenAI e gerar resumo funciona
- [ ] Com `auto_generate = true`, ao parar gravação os três itens são gerados automaticamente
- [ ] Mudar idioma do whisper e transcrever — áudio é transcrito no idioma correto
