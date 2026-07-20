# Whisper Auto Language Detection + Display — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `whisper_language = "auto"` actually auto-detect the spoken language per recording, persist the detected language on the meeting, and show it as a badge in the UI.

**Architecture:** Normalize the "auto" intent in Python (the model owner): `"auto"`/`""`/`None` → `language=None` to faster-whisper, which detects and returns `info.language`. Go forwards the raw setting value (including `"auto"`) and persists the returned language on the `Meeting`. The React UI renders a language badge. AI prompts are already language-agnostic, so no AI changes.

**Tech Stack:** Python (faster-whisper, pytest), Go 1.22+ (modernc/sqlite, database/sql), React 19 + TypeScript.

## Global Constraints

- Sem comentários no código, salvo quando o WHY é não-óbvio.
- Sem mocks em testes de repositório — SQLite via `t.TempDir()`.
- Dois entry points (`cmd/api`, `cmd/desktop`) usam os mesmos services/repository — mudanças de backend valem para ambos automaticamente.
- Migrations são embed e aplicadas automaticamente ao abrir o banco; numeração sequencial (próxima: `014`).
- Setting `whisper_language` ∈ `{pt, en, es, auto}`.

---

### Task 1: Python — auto-detection normalization

**Files:**
- Modify: `audio-service/transcriber.py:107` (the `transcribe` method)
- Modify: `audio-service/main.py:16` (`WHISPER_LANGUAGE` standalone default)
- Test: `audio-service/tests/test_transcriber.py`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: HTTP contract unchanged — `POST /transcribe` still accepts `{"path", "language"}` and returns `{"transcript", "language", "duration_seconds", "model"}`. Behavior change only: `language` in `{"", "auto", null}` now triggers detection; the returned `language` is the detected/used code.

- [ ] **Step 1: Replace the default-language test with auto-detection tests**

In `audio-service/tests/test_transcriber.py`, DELETE `test_transcribe_uses_default_language_when_none_provided` (lines 79-91 — it locks the buggy behavior) and add these three tests in its place:

```python
def test_transcribe_auto_passes_none_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "en"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    result = transcriber.transcribe(wav, language="auto")

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] is None
    assert result.language == "en"


def test_transcribe_empty_string_passes_none_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "es"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav, language="")

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] is None


def test_transcribe_none_passes_none_language(transcriber, tmp_path):
    wav = tmp_path / "rec.wav"
    wav.write_bytes(b"fake")
    info = MagicMock()
    info.language = "pt"
    info.duration = 5.0
    transcriber._fake_model.transcribe.return_value = (iter([]), info)

    transcriber.transcribe(wav)

    args, kwargs = transcriber._fake_model.transcribe.call_args
    assert kwargs["language"] is None
```

(`test_transcribe_uses_provided_language` stays as-is — it already asserts an explicit code is forwarded.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd audio-service && python -m pytest tests/test_transcriber.py -v`
Expected: the three new tests FAIL — current code produces `kwargs["language"] == "pt"` (from `language or self.default_language`), not `None`.

- [ ] **Step 3: Normalize the language in `transcribe`**

In `audio-service/transcriber.py`, replace line 107:

```python
        lang = language or self.default_language
```

with:

```python
        # "", "auto" and None all mean "detect": faster-whisper auto-detects
        # when language is None and returns the result in info.language.
        lang = None if language in (None, "", "auto") else language
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd audio-service && python -m pytest tests/test_transcriber.py -v`
Expected: PASS (all transcriber tests, including `test_transcribe_uses_provided_language` and `test_transcribe_concatenates_segments`).

- [ ] **Step 5: Update the standalone env default so the bug can't resurface**

In `audio-service/main.py`, change line 16 from:

```python
WHISPER_LANGUAGE = os.getenv("WHISPER_LANGUAGE", "pt")
```

to:

```python
WHISPER_LANGUAGE = os.getenv("WHISPER_LANGUAGE", "auto")
```

- [ ] **Step 6: Run the full audio-service test suite**

Run: `cd audio-service && python -m pytest -v`
Expected: PASS (no regressions in `test_main.py` / `test_recorder.py`).

- [ ] **Step 7: Commit**

```bash
git add audio-service/transcriber.py audio-service/main.py audio-service/tests/test_transcriber.py
git commit -m "fix: whisper 'auto'/empty language triggers real auto-detection"
```

---

### Task 2: Migration 014 + Meeting.Language + repository

**Files:**
- Create: `internal/database/migrations/014_meeting_language.sql`
- Modify: `internal/models/meeting.go:27-40` (the `Meeting` struct)
- Modify: `internal/repository/meeting_repository.go` (3 SELECTs at lines 32, 110, 162; `Update` at 128-131; `scanMeeting` at 179-234)
- Test: `internal/repository/meeting_repository_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `models.Meeting.Language *string` (JSON `"language"`), persisted via `MeetingRepository.Update` and read by `List`/`GetByID`/`GetRecording`. Nil when the column is NULL.

- [ ] **Step 1: Write the failing repository round-trip test**

Add to `internal/repository/meeting_repository_test.go`:

```go
func TestMeetingRepository_Language_RoundTrip(t *testing.T) {
	repo := openMeetingTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC()
	m := &models.Meeting{ID: "m-lang", Title: "R", StartedAt: &now, Status: models.StatusPending}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, _ := repo.GetByID(ctx, "m-lang")
	if got.Language != nil {
		t.Errorf("new meeting language = %v, want nil", got.Language)
	}

	lang := "en"
	got.Language = &lang
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	reloaded, _ := repo.GetByID(ctx, "m-lang")
	if reloaded.Language == nil || *reloaded.Language != "en" {
		t.Errorf("language = %v, want \"en\"", reloaded.Language)
	}
}
```

(`openMeetingTestDB(t)` is the existing helper at the top of `meeting_repository_test.go`; it opens an in-memory-style DB via `database.Open(t.TempDir()+"/test.db")` and returns a `*repository.MeetingRepository`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/repository/ -run TestMeetingRepository_Language_RoundTrip -v`
Expected: FAIL — compile error `got.Language undefined (type *models.Meeting has no field or method Language)`.

- [ ] **Step 3: Create migration 014**

Create `internal/database/migrations/014_meeting_language.sql`:

```sql
ALTER TABLE meetings ADD COLUMN language TEXT;
```

- [ ] **Step 4: Add the `Language` field to the model**

In `internal/models/meeting.go`, add the field after `ErrorMessage` (keep the existing order otherwise):

```go
	ErrorMessage    *string       `json:"error_message"`
	Language        *string       `json:"language"`
	KeepAudio       bool          `json:"keep_audio"`
```

- [ ] **Step 5: Add `language` to the three SELECT column lists**

In `internal/repository/meeting_repository.go`, in `List` (line 32), `GetByID` (line 110), and `GetRecording` (line 162), change the column list from:

```
... audio_path, error_message, keep_audio, created_at FROM meetings ...
```

to:

```
... audio_path, error_message, keep_audio, language, created_at FROM meetings ...
```

(Insert `language` immediately before `created_at` in all three queries.)

- [ ] **Step 6: Add `language` to the UPDATE statement**

In `internal/repository/meeting_repository.go` `Update` (lines 128-131), change the SET clause and args:

```go
	result, err := r.db.ExecContext(ctx,
		`UPDATE meetings SET theme_id = ?, title = ?, started_at = ?, duration_seconds = ?, status = ?, transcript = ?, notes = ?, audio_path = ?, error_message = ?, keep_audio = ?, language = ? WHERE id = ?`,
		m.ThemeID, m.Title, startedAt, m.DurationSeconds, string(m.Status), m.Transcript, m.Notes, m.AudioPath, m.ErrorMessage, m.KeepAudio, m.Language, m.ID,
	)
```

- [ ] **Step 7: Scan `language` in `scanMeeting`**

In `internal/repository/meeting_repository.go` `scanMeeting` (lines 179-234): add a `language` null holder, add it to the `Scan` call in the new column position (after `keepAudio`, before `createdAt`), and map it onto the struct.

Add the declaration near the others:

```go
	var errorMessage sql.NullString
	var language sql.NullString
	var keepAudio int
```

Change the `Scan` call to include `&language` in the correct position:

```go
	err := row.Scan(&m.ID, &themeID, &m.Title, &startedAt, &duration, &status,
		&transcript, &notes, &audioPath, &errorMessage, &keepAudio, &language, &createdAt)
```

Add the mapping after the `errorMessage` block:

```go
	if language.Valid {
		v := language.String
		m.Language = &v
	}
```

- [ ] **Step 8: Run the test to verify it passes**

Run: `go test ./internal/repository/ -run TestMeetingRepository_Language_RoundTrip -v`
Expected: PASS.

- [ ] **Step 9: Run the full repository suite (no regressions)**

Run: `go test ./internal/repository/`
Expected: `ok  meeting-notes/internal/repository`.

- [ ] **Step 10: Commit**

```bash
git add internal/database/migrations/014_meeting_language.sql internal/models/meeting.go internal/repository/meeting_repository.go internal/repository/meeting_repository_test.go
git commit -m "feat: persist meeting language (migration 014 + repository)"
```

---

### Task 3: Orchestrator — forward "auto" and persist detected language

**Files:**
- Modify: `internal/services/orchestrator.go` (capture pipeline block 204-216; retranscribe block 408-420)
- Modify: `internal/services/orchestrator_test.go` (`fakeAudioClient`, lines 31-64)
- Test: `internal/services/orchestrator_test.go`

**Interfaces:**
- Consumes: `models.Meeting.Language` (Task 2); `audio.TranscribeResponse.Language` (existing).
- Produces: after any transcription, `meeting.language` equals whatever the audio-service returned; the audio-service receives the raw `whisper_language` setting (`"auto"` included).

- [ ] **Step 1: Capture the forwarded language in the fake audio client**

In `internal/services/orchestrator_test.go`, add a capture field to `fakeAudioClient` (after `transcribeCalls`, line 41):

```go
	startCalls, stopCalls, transcribeCalls int
	lastLanguage                           string
```

and record it in `Transcribe` (lines 61-64):

```go
func (f *fakeAudioClient) Transcribe(ctx context.Context, path, language string) (*audio.TranscribeResponse, error) {
	f.transcribeCalls++
	f.lastLanguage = language
	return f.transcribeResp, f.transcribeErr
}
```

- [ ] **Step 2: Write the failing test**

Add to `internal/services/orchestrator_test.go`:

```go
func TestOrchestrator_RunCapturePipeline_AutoLanguageForwardedAndPersisted(t *testing.T) {
	wavPath := t.TempDir() + "/rec-1.wav"
	if err := os.WriteFile(wavPath, fakeWAVBytes(), 0o644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	fa := &fakeAudioClient{
		stopResp:       &audio.StopResponse{RecordingID: "r-1", Path: wavPath, DurationSeconds: 8.0},
		transcribeResp: &audio.TranscribeResponse{Transcript: "hello world", Language: "en", DurationSeconds: 8.0, Model: "medium"},
	}
	fai := &fakeAI{summaryText: "s", keyPoints: []string{"k"}, tasks: []ai.TaskSuggestion{{Description: "d", Priority: "medium"}}}
	settings := map[string]string{"ai_provider": "anthropic", "anthropic_api_key": "sk-test", "whisper_language": "auto"}
	orch, mr, id := newOrchTestSettings(t, fa, fai, settings)

	m, _ := mr.GetByID(context.Background(), id)
	m.Status = models.StatusRecording
	mr.Update(context.Background(), m)

	if err := orch.RunCapturePipeline(context.Background(), id); err != nil {
		t.Fatalf("RunCapturePipeline: %v", err)
	}

	if fa.lastLanguage != "auto" {
		t.Errorf("forwarded language = %q, want \"auto\"", fa.lastLanguage)
	}
	got, _ := mr.GetByID(context.Background(), id)
	if got.Language == nil || *got.Language != "en" {
		t.Errorf("persisted language = %v, want \"en\"", got.Language)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/services/ -run TestOrchestrator_RunCapturePipeline_AutoLanguageForwardedAndPersisted -v`
Expected: FAIL — `forwarded language = "" want "auto"` (current code zeroes it on "auto") and `persisted language = <nil>`.

- [ ] **Step 4: Forward the raw setting + persist language in the capture pipeline**

In `internal/services/orchestrator.go`, replace the block at lines 204-209:

```go
	whisperLang := ""
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		if v := s["whisper_language"]; v != "" && v != "auto" {
			whisperLang = v
		}
	}
```

with:

```go
	whisperLang := ""
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		whisperLang = s["whisper_language"]
	}
```

Then, right after `m.Transcript = &trResp.Transcript` (line 216), add:

```go
	m.Transcript = &trResp.Transcript
	m.Language = &trResp.Language
```

- [ ] **Step 5: Apply the same two changes to the retranscribe pipeline**

In `internal/services/orchestrator.go`, replace the block at lines 408-413:

```go
	whisperLang := ""
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		if v := s["whisper_language"]; v != "" && v != "auto" {
			whisperLang = v
		}
	}
```

with:

```go
	whisperLang := ""
	if s, err2 := o.settings.GetAll(ctx); err2 == nil {
		whisperLang = s["whisper_language"]
	}
```

Then, right after `m.Transcript = &trResp.Transcript` (line 420), add:

```go
	m.Transcript = &trResp.Transcript
	m.Language = &trResp.Language
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/services/ -run TestOrchestrator_RunCapturePipeline_AutoLanguageForwardedAndPersisted -v`
Expected: PASS.

- [ ] **Step 7: Run the full services suite (no regressions)**

Run: `go test ./internal/services/`
Expected: `ok  meeting-notes/internal/services`. (Existing pipeline tests still pass — they use `Language: "pt"` in `transcribeResp`, which now also gets persisted, but they don't assert language.)

- [ ] **Step 8: Commit**

```bash
git add internal/services/orchestrator.go internal/services/orchestrator_test.go
git commit -m "feat: forward whisper_language verbatim and persist detected language"
```

---

### Task 4: Frontend — Meeting type, languageLabel helper, badge

**Files:**
- Create: `frontend/src/lib/language.ts`
- Modify: `frontend/src/hooks/useMeetings.ts:4-17` (the `Meeting` interface)
- Modify: `frontend/src/components/layout/MeetingDetail.tsx` (badge cluster, lines 198-199)

**Interfaces:**
- Consumes: `meeting.language` (string | null) from the backend JSON (Task 2/3).
- Produces: `languageLabel(code: string): string` in `frontend/src/lib/language.ts`.

> **Note (deviation from spec):** the spec's testing section proposed a unit test for `languageLabel`. The frontend has **no test runner** (no `test` script, no vitest/jest, no `*.test.*` files). Rather than introduce test infra for one pure function (YAGNI), `languageLabel` is covered by `tsc --noEmit`, the build, and the manual check in Final verification. Add a runner later if the frontend grows a test suite.

- [ ] **Step 1: Create the languageLabel helper**

Create `frontend/src/lib/language.ts`:

```ts
const LABELS: Record<string, string> = {
  pt: "Português",
  en: "Inglês",
  es: "Espanhol",
  fr: "Francês",
  de: "Alemão",
  it: "Italiano",
  ja: "Japonês",
  zh: "Chinês",
}

export function languageLabel(code: string): string {
  return LABELS[code.toLowerCase()] ?? code.toUpperCase()
}
```

- [ ] **Step 2: Add `language` to the Meeting interface**

In `frontend/src/hooks/useMeetings.ts`, add the field to the `Meeting` interface (after `error_message`, line 14):

```ts
  error_message: string | null
  language: string | null
  keep_audio: boolean
```

- [ ] **Step 3: Import the helper and render the badge**

In `frontend/src/components/layout/MeetingDetail.tsx`:

Add the import next to the other `../../lib` imports (after line 19, `import { highlightText } from "../../lib/highlight"`):

```ts
import { languageLabel } from "../../lib/language"
```

Then, in the badge cluster (line 198-199), add a language badge right after the status badge:

```tsx
        <div className="flex items-center gap-1.5 flex-shrink-0">
          <Badge variant={statusVariant(meeting.status)}>{meeting.status}</Badge>
          {meeting.language && (
            <Badge variant="secondary">{languageLabel(meeting.language)}</Badge>
          )}
```

(`secondary` is a valid Badge variant — `bg-muted text-muted-foreground` in `frontend/src/components/ui/badge.tsx`.)

- [ ] **Step 4: Typecheck the frontend**

Run: `cd frontend && npx tsc --noEmit`
Expected: no type errors.

- [ ] **Step 5: Build the frontend (sanity)**

Run: `cd frontend && npm run build`
Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/language.ts frontend/src/hooks/useMeetings.ts frontend/src/components/layout/MeetingDetail.tsx
git commit -m "feat: show detected meeting language badge in MeetingDetail"
```

---

## Final verification

- [ ] `go vet ./...` clean; `go test ./...` green.
- [ ] `cd audio-service && python -m pytest` green.
- [ ] `cd frontend && npx tsc --noEmit` clean.
- [ ] Manual (optional, via `wails dev`): set Transcrição → idioma = "auto", record a short clip in English, confirm the transcript is English and the MeetingDetail shows an "Inglês" badge.
