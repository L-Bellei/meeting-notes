# Fatia 8 — UI Wails Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Wails v2 desktop app with React frontend that consumes the existing Go HTTP API internally.

**Architecture:** cmd/desktop/ entrypoint starts the HTTP server on a random port and exposes GetPort() binding; React uses fetch() against that port with React Query for caching; Wails runtime events push pipeline status changes from Go to React in real time.

**Tech Stack:** Go 1.26, Wails v2.12, React 18, TypeScript, Vite, Tailwind CSS, Shadcn/ui, React Query v5, lucide-react

---

## Task 1: Add Wails dependency + create wails.json + scaffold frontend

- [ ] Run `go get github.com/wailsapp/wails/v2@latest` in project root
- [ ] Create `wails.json` at project root with the following content:
```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "Meeting Notes",
  "outputfilename": "meeting-notes",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "frontend:dev:watcher": "npm run dev",
  "frontend:dev:serverUrl": "auto",
  "wailsjsdir": "./frontend/src/wailsjs",
  "version": "2",
  "info": {
    "companyName": "",
    "productName": "Meeting Notes",
    "productVersion": "0.1.0"
  }
}
```
- [ ] Scaffold frontend with Vite React TS (run from project root `F:/dev/meeting-notes`):
```
npm create vite@latest frontend -- --template react-ts
```
- [ ] Verify `frontend/package.json` exists
- [ ] Commit: `feat: add Wails dependency, wails.json config, and Vite React TS frontend scaffold`

---

## Task 2: Modify Orchestrator to support notifyFn

**Files:** `internal/services/orchestrator.go` (modify), `internal/services/orchestrator_test.go` (add test)

### Changes to `internal/services/orchestrator.go`

- [ ] Add `notifyFn func(meetingID, status string)` field to the `Orchestrator` struct (after the existing `pipelineWG sync.WaitGroup` field)
- [ ] Add `SetNotifyFn` method:
```go
func (o *Orchestrator) SetNotifyFn(fn func(meetingID, status string)) {
    o.notifyFn = fn
}
```
- [ ] Add private `notify` helper:
```go
func (o *Orchestrator) notify(meetingID string, status models.MeetingStatus) {
    if o.notifyFn != nil {
        o.notifyFn(meetingID, string(status))
    }
}
```
- [ ] In `RunCapturePipeline`: call `o.notify(m.ID, m.Status)` after EACH successful `o.repo.Update(ctx, m)` — there are three such calls:
  1. After setting `m.Status = models.StatusTranscribing` and updating (line ~128): add `o.notify(m.ID, m.Status)` after the `if err := o.repo.Update(ctx, m)` block succeeds
  2. After setting `m.Status = models.StatusProcessing` and updating (line ~148): add `o.notify(m.ID, m.Status)` after the `if err := o.repo.Update(ctx, m)` block succeeds
  3. After setting `m.Status = models.StatusCompleted` and the final `o.repo.Update(ctx, m)` (line ~161): add `o.notify(m.ID, m.Status)` — note the final `return o.repo.Update(ctx, m)` must be split into two lines to call notify on success
- [ ] In `RunAIPipeline`: call `o.notify(m.ID, m.Status)` after the successful `o.repo.Update(ctx, m)` for StatusProcessing, and after the final successful update for StatusCompleted
- [ ] In `markFailed`: call `o.notify(m.ID, m.Status)` after the successful `o.repo.Update(ctx, m)` for StatusFailed

The final `RunCapturePipeline` notify pattern for the last update (currently `return o.repo.Update(ctx, m)`):
```go
if err := o.repo.Update(ctx, m); err != nil {
    return err
}
o.notify(m.ID, m.Status)
return nil
```

### Add test to `internal/services/orchestrator_test.go`

- [ ] Add `TestOrchestrator_NotifyFn_CalledOnStatusChange` at the end of the file:
```go
func TestOrchestrator_NotifyFn_CalledOnStatusChange(t *testing.T) {
    transcript := "hello world"
    fa := &fakeAudioClient{
        startResp:      &audio.StartResponse{RecordingID: "r-1", StartedAt: time.Now().UTC()},
        stopResp:       &audio.StopResponse{Path: t.TempDir() + "/rec.wav", DurationSeconds: 10},
        transcribeResp: &audio.TranscribeResponse{Transcript: transcript},
    }
    orch, _, id := newOrchTest(t, fa, &fakeAI{summaryContent: "s", keyPoints: []string{"kp1"}, tasks: []string{"t1"}})

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
    if err := orch.StopRecording(context.Background(), id); err != nil {
        t.Fatalf("StopRecording: %v", err)
    }
    orch.WaitPipelines()

    mu.Lock()
    defer mu.Unlock()

    wantStatuses := []string{"transcribing", "processing", "completed"}
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
```

Note: This test requires `sync` import; verify it is already imported or add it. The `fakeAI` struct must have `summaryContent`, `keyPoints`, and `tasks` fields — check the existing test file to see the fakeAI definition and match its field names exactly.

- [ ] Run: `go test ./internal/services/... -run TestOrchestrator_NotifyFn -v`
- [ ] Expected: PASS
- [ ] Commit: `feat(services): add notifyFn to Orchestrator for pipeline status events`

---

## Task 3: cmd/desktop backend (app.go + main.go + tests)

- [ ] Create directory `cmd/desktop/`
- [ ] Create `cmd/desktop/app.go` with the following complete content:

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/audio"
	"meeting-notes/internal/config"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

type App struct {
	ctx    context.Context
	db     *sql.DB
	port   int
	server *http.Server
}

func NewApp() *App { return &App{} }

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		wailsruntime.LogErrorf(ctx, "open db: %v", err)
		return
	}
	a.db = db

	var aiClient ai.AIClient
	if cfg.AnthropicAPIKey != "" {
		aiClient = ai.NewAnthropicClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
	}

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

	audioClient := audio.NewHTTPClient(cfg.AudioServiceURL)
	orch := services.NewOrchestrator(meetingRepo, summarySvc, keyPointSvc, taskSvc, audioClient, cfg.WhisperLanguage)
	orch.SetNotifyFn(func(meetingID, status string) {
		type payload struct {
			MeetingID string `json:"meeting_id"`
			Status    string `json:"status"`
		}
		wailsruntime.EventsEmit(ctx, "pipeline:status", payload{MeetingID: meetingID, Status: status})
	})

	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo, orch)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

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

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		wailsruntime.LogErrorf(ctx, "listen: %v", err)
		return
	}
	a.port = ln.Addr().(*net.TCPAddr).Port
	a.server = &http.Server{Handler: r}
	go a.server.Serve(ln)
}

func (a *App) OnShutdown(ctx context.Context) {
	if a.server != nil {
		a.server.Shutdown(context.Background())
	}
	if a.db != nil {
		a.db.Close()
	}
}

func (a *App) GetPort() int { return a.port }
```

- [ ] Create `cmd/desktop/main.go` with the following complete content:

```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "Meeting Notes",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:        app.OnStartup,
		OnShutdown:       app.OnShutdown,
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] Create `cmd/desktop/app_test.go` with the following complete content:

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestApp_GetPort_AfterStartup(t *testing.T) {
	app := NewApp()
	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	if app.GetPort() == 0 {
		t.Fatal("expected non-zero port after startup")
	}
}

func TestApp_HTTPServerResponds(t *testing.T) {
	app := NewApp()
	app.OnStartup(context.Background())
	defer app.OnShutdown(context.Background())

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", app.GetPort()))
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestApp_Shutdown_NoError(t *testing.T) {
	app := NewApp()
	app.OnStartup(context.Background())
	app.OnShutdown(context.Background())
}
```

**Note on test compilation:** The `wailsruntime` package imported in `app.go` is only available after Task 1 adds the Wails dependency. The `wailsruntime.LogErrorf` and `wailsruntime.EventsEmit` calls will be no-ops when called with `context.Background()` (the Wails runtime checks for its own context type). Tests pass a plain `context.Background()`, which is safe — the runtime functions are nil-safe for unregistered contexts.

- [ ] Run: `go test ./cmd/desktop/ -v`
- [ ] Expected: all three tests PASS
- [ ] Commit: `feat(desktop): add Wails app backend with HTTP server and GetPort binding`

---

## Task 4: Frontend configuration — Tailwind + Shadcn + React Query

- [ ] In `frontend/`, install runtime dependencies:
```bash
cd frontend
npm install @tanstack/react-query lucide-react class-variance-authority clsx tailwind-merge
```
- [ ] Install dev dependencies:
```bash
cd frontend
npm install -D tailwindcss postcss autoprefixer @types/node
```
- [ ] Initialize Tailwind:
```bash
cd frontend
npx tailwindcss init -p
```
- [ ] Replace `frontend/tailwind.config.js` content with:
```js
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [],
}
```
- [ ] Replace `frontend/src/index.css` content with:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --border: 214.3 31.8% 91.4%;
    --primary: 222.2 47.4% 11.2%;
    --primary-foreground: 210 40% 98%;
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 210 40% 98%;
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;
    --radius: 0.5rem;
  }
  * { @apply border-border; }
  body { @apply bg-background text-foreground; }
}
```
- [ ] Create `frontend/src/lib/utils.ts`:
```typescript
import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```
- [ ] Create `frontend/src/components/ui/badge.tsx`:
```typescript
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "../../lib/utils"

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors",
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary text-primary-foreground",
        secondary: "border-transparent bg-muted text-muted-foreground",
        destructive: "border-transparent bg-destructive text-destructive-foreground",
        outline: "text-foreground",
        recording: "border-transparent bg-red-500 text-white",
        transcribing: "border-transparent bg-yellow-500 text-white",
        processing: "border-transparent bg-yellow-400 text-white",
        completed: "border-transparent bg-green-500 text-white",
        failed: "border-transparent bg-red-700 text-white",
        pending: "border-transparent bg-gray-400 text-white",
      },
    },
    defaultVariants: { variant: "default" },
  }
)

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}
```
- [ ] Create `frontend/src/components/ui/button.tsx`:
```typescript
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "../../lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
        outline: "border border-input bg-background hover:bg-accent hover:text-accent-foreground",
        ghost: "hover:bg-accent hover:text-accent-foreground",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 rounded-md px-3 text-xs",
        lg: "h-10 rounded-md px-8",
        icon: "h-9 w-9",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  }
)

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {}

export function Button({ className, variant, size, ...props }: ButtonProps) {
  return <button className={cn(buttonVariants({ variant, size }), className)} {...props} />
}
```
- [ ] Run `npm run build` in frontend to verify setup:
```bash
cd frontend && npm run build
```
- [ ] Expected: builds without errors (empty App placeholder is fine)
- [ ] Commit: `feat(frontend): configure Tailwind, Shadcn components, React Query`

---

## Task 5: API hooks

Create the following files in `frontend/src/hooks/`:

- [ ] Create `frontend/src/hooks/useApi.ts`:
```typescript
let baseURL = ""

export function initApi(port: number) {
  baseURL = `http://localhost:${port}`
}

export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${baseURL}${path}`, {
    headers: { "Content-Type": "application/json", ...options?.headers },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}
```

- [ ] Create `frontend/src/hooks/useThemes.ts`:
```typescript
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface Theme {
  id: string
  name: string
  description: string
  color: string
  created_at: string
}

export function useThemes() {
  return useQuery({ queryKey: ["themes"], queryFn: () => api<Theme[]>("/api/themes") })
}

export function useCreateTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description: string; color: string }) =>
      api<Theme>("/api/themes", { method: "POST", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["themes"] }),
  })
}

export function useDeleteTheme() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api<void>(`/api/themes/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["themes"] }),
  })
}
```

- [ ] Create `frontend/src/hooks/useMeetings.ts`:
```typescript
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"

export interface Meeting {
  id: string
  theme_id: string | null
  title: string
  started_at: string | null
  duration_seconds: number | null
  status: "pending" | "recording" | "transcribing" | "processing" | "completed" | "failed"
  transcript: string | null
  created_at: string
}

export function useMeetings(themeId?: string | null) {
  return useQuery({
    queryKey: ["meetings", themeId ?? "all"],
    queryFn: () => api<Meeting[]>("/api/meetings"),
    select: (data) => themeId ? data.filter(m => m.theme_id === themeId) : data,
  })
}

export function useCreateMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { title: string; theme_id?: string }) =>
      api<Meeting>("/api/meetings", { method: "POST", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meetings"] }),
  })
}

export function useDeleteMeeting() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api<void>(`/api/meetings/${id}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meetings"] }),
  })
}
```

- [ ] Create `frontend/src/hooks/useMeeting.ts`:
```typescript
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { api } from "./useApi"
import type { Meeting } from "./useMeetings"

export interface Summary {
  id: string
  meeting_id: string
  content: string
  model_used: string
  input_tokens: number
  output_tokens: number
  created_at: string
}

export interface KeyPoint {
  id: string
  meeting_id: string
  position: number
  content: string
}

export interface Task {
  id: string
  meeting_id: string
  description: string
  assignee: string | null
  due_date: string | null
  priority: "low" | "medium" | "high"
  completed: boolean
  created_at: string
}

export interface MeetingDetail extends Meeting {
  summary: Summary | null
  key_points: KeyPoint[]
  tasks: Task[]
}

export function useMeeting(id: string | null) {
  return useQuery({
    queryKey: ["meeting", id],
    queryFn: () => api<MeetingDetail>(`/api/meetings/${id}`),
    enabled: !!id,
  })
}

export function useUpdateMeeting(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<Meeting>) =>
      api<Meeting>(`/api/meetings/${id}`, { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["meeting", id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}

export function useStartRecording(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<void>(`/api/meetings/${id}/start`, { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["meeting", id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}

export function useStopRecording(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<void>(`/api/meetings/${id}/stop`, { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["meeting", id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    },
  })
}

export function useReprocess(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<void>(`/api/meetings/${id}/process`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", id] }),
  })
}

export function useGenerateSummary(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<Summary>(`/api/meetings/${meetingId}/summary/generate`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useGenerateKeyPoints(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<KeyPoint[]>(`/api/meetings/${meetingId}/key_points/generate`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useGenerateTasks(meetingId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => api<Task[]>(`/api/meetings/${meetingId}/tasks/generate`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}

export function useUpdateTask(meetingId: string, taskId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<Task>) =>
      api<Task>(`/api/meetings/${meetingId}/tasks/${taskId}`, { method: "PUT", body: JSON.stringify(data) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["meeting", meetingId] }),
  })
}
```

- [ ] Create `frontend/src/hooks/usePipeline.ts`:
```typescript
import { useEffect } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { EventsOn } from "../wailsjs/runtime/runtime"

interface PipelinePayload {
  meeting_id: string
  status: string
}

export function usePipeline() {
  const qc = useQueryClient()
  useEffect(() => {
    const unlisten = EventsOn("pipeline:status", (payload: PipelinePayload) => {
      qc.invalidateQueries({ queryKey: ["meeting", payload.meeting_id] })
      qc.invalidateQueries({ queryKey: ["meetings"] })
    })
    return () => { if (typeof unlisten === "function") unlisten() }
  }, [qc])
}
```

**Note on `usePipeline.ts`:** The import `../wailsjs/runtime/runtime` is generated by Wails during `wails dev` or `wails build`. Run `wails generate module` from project root if the file does not exist yet; or add a temporary stub at `frontend/src/wailsjs/runtime/runtime.ts` for local TypeScript compilation:
```typescript
// stub for local dev before wails generate
export function EventsOn(event: string, cb: (data: any) => void): () => void {
  if (typeof (window as any).runtime?.EventsOn === "function") {
    return (window as any).runtime.EventsOn(event, cb)
  }
  return () => {}
}
```

- [ ] Run `cd frontend && npm run build` — must succeed
- [ ] Commit: `feat(frontend): add API hooks for themes, meetings, and pipeline events`

---

## Task 6: App.tsx shell + three-column layout

- [ ] Replace `frontend/src/App.tsx` with:
```typescript
import { useEffect, useState } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { GetPort } from "./wailsjs/go/main/App"
import { initApi } from "./hooks/useApi"
import { usePipeline } from "./hooks/usePipeline"
import { Sidebar } from "./components/layout/Sidebar"
import { MeetingList } from "./components/layout/MeetingList"
import { MeetingDetail } from "./components/layout/MeetingDetail"
import { Toolbar } from "./components/layout/Toolbar"

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000 } },
})

function AppInner() {
  const [ready, setReady] = useState(false)
  const [selectedThemeId, setSelectedThemeId] = useState<string | null>(null)
  const [selectedMeetingId, setSelectedMeetingId] = useState<string | null>(null)
  const [recordingModalOpen, setRecordingModalOpen] = useState(false)

  useEffect(() => {
    GetPort().then(port => {
      initApi(port)
      setReady(true)
    })
  }, [])

  usePipeline()

  if (!ready) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground text-sm">
        Iniciando...
      </div>
    )
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      <Toolbar
        onRecord={() => setRecordingModalOpen(true)}
        recordingModalOpen={recordingModalOpen}
        onRecordingModalClose={() => setRecordingModalOpen(false)}
        onMeetingCreated={(id) => setSelectedMeetingId(id)}
      />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          selectedThemeId={selectedThemeId}
          onSelectTheme={setSelectedThemeId}
        />
        <MeetingList
          themeId={selectedThemeId}
          selectedMeetingId={selectedMeetingId}
          onSelectMeeting={setSelectedMeetingId}
        />
        <MeetingDetail
          meetingId={selectedMeetingId}
        />
      </div>
    </div>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppInner />
    </QueryClientProvider>
  )
}
```

**Note on `GetPort` import:** The file `frontend/src/wailsjs/go/main/App.ts` is generated by Wails. Add a temporary stub at `frontend/src/wailsjs/go/main/App.ts` for local TypeScript compilation:
```typescript
// stub for local dev before wails generate
export async function GetPort(): Promise<number> {
  return (window as any).go?.main?.App?.GetPort?.() ?? 0
}
```

- [ ] Update `frontend/src/main.tsx`:
```typescript
import React from "react"
import ReactDOM from "react-dom/client"
import App from "./App"
import "./index.css"

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)
```

- [ ] Create stub `frontend/src/components/layout/Sidebar.tsx`:
```typescript
export function Sidebar(props: any) { return <div className="w-48 border-r h-full" /> }
```
- [ ] Create stub `frontend/src/components/layout/MeetingList.tsx`:
```typescript
export function MeetingList(props: any) { return <div className="w-72 border-r h-full" /> }
```
- [ ] Create stub `frontend/src/components/layout/MeetingDetail.tsx`:
```typescript
export function MeetingDetail(props: any) { return <div className="flex-1 h-full" /> }
```
- [ ] Create stub `frontend/src/components/layout/Toolbar.tsx`:
```typescript
export function Toolbar(props: any) { return <div className="h-12 border-b" /> }
```

- [ ] Run `cd frontend && npm run build` — must succeed
- [ ] Commit: `feat(frontend): add App shell with three-column layout and boot sequence`

---

## Task 7: Sidebar component

- [ ] Replace `frontend/src/components/layout/Sidebar.tsx` with the full implementation:
```typescript
import { useState } from "react"
import { Plus, Tag } from "lucide-react"
import { useThemes, useCreateTheme } from "../../hooks/useThemes"
import { useMeetings } from "../../hooks/useMeetings"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"

interface SidebarProps {
  selectedThemeId: string | null
  onSelectTheme: (id: string | null) => void
}

export function Sidebar({ selectedThemeId, onSelectTheme }: SidebarProps) {
  const { data: themes = [] } = useThemes()
  const { data: allMeetings = [] } = useMeetings()
  const createTheme = useCreateTheme()
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState("")

  function countForTheme(id: string) {
    return allMeetings.filter(m => m.theme_id === id).length
  }

  async function handleCreate() {
    if (!newName.trim()) return
    await createTheme.mutateAsync({ name: newName.trim(), description: "", color: "#6366F1" })
    setNewName("")
    setCreating(false)
  }

  return (
    <div className="w-48 border-r h-full flex flex-col bg-muted/30">
      <div className="p-3 border-b">
        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Temas</span>
      </div>
      <div className="flex-1 overflow-y-auto">
        <button
          onClick={() => onSelectTheme(null)}
          className={cn(
            "w-full text-left px-3 py-2 text-sm flex items-center justify-between hover:bg-accent transition-colors",
            selectedThemeId === null && "bg-accent font-medium"
          )}
        >
          <span className="flex items-center gap-2"><Tag size={14} />Todos</span>
          <span className="text-xs text-muted-foreground">{allMeetings.length}</span>
        </button>
        {themes.map(theme => (
          <button
            key={theme.id}
            onClick={() => onSelectTheme(theme.id)}
            className={cn(
              "w-full text-left px-3 py-2 text-sm flex items-center justify-between hover:bg-accent transition-colors",
              selectedThemeId === theme.id && "bg-accent font-medium"
            )}
          >
            <span className="flex items-center gap-2 truncate">
              <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: theme.color }} />
              <span className="truncate">{theme.name}</span>
            </span>
            <span className="text-xs text-muted-foreground">{countForTheme(theme.id)}</span>
          </button>
        ))}
      </div>
      <div className="p-2 border-t">
        {creating ? (
          <div className="flex gap-1">
            <input
              autoFocus
              value={newName}
              onChange={e => setNewName(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleCreate(); if (e.key === "Escape") setCreating(false) }}
              placeholder="Nome do tema"
              className="flex-1 text-xs border rounded px-2 py-1 bg-background"
            />
            <Button size="sm" onClick={handleCreate} disabled={createTheme.isPending}>+</Button>
          </div>
        ) : (
          <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => setCreating(true)}>
            <Plus size={14} className="mr-1" /> Novo tema
          </Button>
        )}
      </div>
    </div>
  )
}
```

- [ ] Run `cd frontend && npm run build` — must succeed
- [ ] Commit: `feat(frontend): implement Sidebar with theme list and inline create`

---

## Task 8: MeetingList component

- [ ] Replace `frontend/src/components/layout/MeetingList.tsx` with the full implementation:
```typescript
import { useState } from "react"
import { Plus, Calendar } from "lucide-react"
import { useMeetings, useCreateMeeting } from "../../hooks/useMeetings"
import { useThemes } from "../../hooks/useThemes"
import { Badge } from "../ui/badge"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"
import type { Meeting } from "../../hooks/useMeetings"

interface MeetingListProps {
  themeId: string | null
  selectedMeetingId: string | null
  onSelectMeeting: (id: string) => void
}

function statusVariant(s: Meeting["status"]) {
  const map: Record<Meeting["status"], string> = {
    pending: "pending", recording: "recording", transcribing: "transcribing",
    processing: "processing", completed: "completed", failed: "failed",
  }
  return map[s] as any
}

function statusLabel(s: Meeting["status"]) {
  const map: Record<Meeting["status"], string> = {
    pending: "Pendente", recording: "Gravando", transcribing: "Transcrevendo",
    processing: "Processando", completed: "Concluído", failed: "Falhou",
  }
  return map[s]
}

function formatDate(iso: string | null) {
  if (!iso) return ""
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit", year: "2-digit" })
}

export function MeetingList({ themeId, selectedMeetingId, onSelectMeeting }: MeetingListProps) {
  const { data: meetings = [] } = useMeetings(themeId)
  const { data: themes = [] } = useThemes()
  const createMeeting = useCreateMeeting()
  const [creating, setCreating] = useState(false)
  const [newTitle, setNewTitle] = useState("")

  async function handleCreate() {
    if (!newTitle.trim()) return
    const m = await createMeeting.mutateAsync({ title: newTitle.trim(), theme_id: themeId ?? undefined })
    setNewTitle("")
    setCreating(false)
    onSelectMeeting(m.id)
  }

  function themeColor(id: string | null) {
    return themes.find(t => t.id === id)?.color ?? "#94a3b8"
  }

  return (
    <div className="w-72 border-r h-full flex flex-col">
      <div className="p-3 border-b flex items-center justify-between">
        <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Reuniões</span>
        <span className="text-xs text-muted-foreground">{meetings.length}</span>
      </div>
      <div className="flex-1 overflow-y-auto">
        {meetings.length === 0 && (
          <div className="p-4 text-center text-sm text-muted-foreground">Nenhuma reunião</div>
        )}
        {meetings.map(m => (
          <button
            key={m.id}
            onClick={() => onSelectMeeting(m.id)}
            className={cn(
              "w-full text-left px-3 py-2.5 border-b hover:bg-accent transition-colors",
              selectedMeetingId === m.id && "bg-accent"
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="flex items-center gap-2 min-w-0">
                <span className="w-2 h-2 rounded-full flex-shrink-0 mt-0.5" style={{ backgroundColor: themeColor(m.theme_id) }} />
                <span className="text-sm font-medium truncate">{m.title}</span>
              </div>
              <Badge variant={statusVariant(m.status)} className="flex-shrink-0 text-[10px]">
                {statusLabel(m.status)}
              </Badge>
            </div>
            {m.started_at && (
              <div className="flex items-center gap-1 mt-1 ml-4 text-xs text-muted-foreground">
                <Calendar size={10} />
                {formatDate(m.started_at)}
              </div>
            )}
          </button>
        ))}
      </div>
      <div className="p-2 border-t">
        {creating ? (
          <div className="flex gap-1">
            <input
              autoFocus
              value={newTitle}
              onChange={e => setNewTitle(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleCreate(); if (e.key === "Escape") setCreating(false) }}
              placeholder="Título da reunião"
              className="flex-1 text-xs border rounded px-2 py-1 bg-background"
            />
            <Button size="sm" onClick={handleCreate} disabled={createMeeting.isPending}>+</Button>
          </div>
        ) : (
          <Button variant="ghost" size="sm" className="w-full text-xs" onClick={() => setCreating(true)}>
            <Plus size={14} className="mr-1" /> Nova reunião
          </Button>
        )}
      </div>
    </div>
  )
}
```

- [ ] Run `cd frontend && npm run build` — must succeed
- [ ] Commit: `feat(frontend): implement MeetingList with status badges and inline create`

---

## Task 9: MeetingDetail component

- [ ] Replace `frontend/src/components/layout/MeetingDetail.tsx` with the full implementation:
```typescript
import { useState, useEffect, useRef } from "react"
import { Play, Square, RefreshCw, Wand2 } from "lucide-react"
import {
  useMeeting, useUpdateMeeting, useStartRecording, useStopRecording,
  useReprocess, useGenerateSummary, useGenerateKeyPoints, useGenerateTasks,
  useUpdateTask,
} from "../../hooks/useMeeting"
import { Badge } from "../ui/badge"
import { Button } from "../ui/button"
import { cn } from "../../lib/utils"

interface Props { meetingId: string | null }

type Tab = "transcript" | "summary" | "keypoints" | "tasks"

function statusVariant(s: string) {
  return s as any
}

export function MeetingDetail({ meetingId }: Props) {
  const { data: meeting } = useMeeting(meetingId)
  const [tab, setTab] = useState<Tab>("transcript")

  if (!meetingId || !meeting) {
    return (
      <div className="flex-1 h-full flex items-center justify-center text-sm text-muted-foreground">
        Selecione uma reunião
      </div>
    )
  }

  return (
    <div className="flex-1 h-full flex flex-col overflow-hidden">
      <MeetingHeader meeting={meeting} />
      <div className="border-b">
        <div className="flex">
          {(["transcript", "summary", "keypoints", "tasks"] as Tab[]).map(t => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={cn(
                "px-4 py-2 text-sm font-medium border-b-2 transition-colors",
                tab === t ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {{ transcript: "Transcrição", summary: "Resumo", keypoints: "Pontos-chave", tasks: "Tarefas" }[t]}
            </button>
          ))}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-4">
        {tab === "transcript" && <TranscriptTab meeting={meeting} />}
        {tab === "summary" && <SummaryTab meeting={meeting} />}
        {tab === "keypoints" && <KeyPointsTab meeting={meeting} />}
        {tab === "tasks" && <TasksTab meeting={meeting} />}
      </div>
    </div>
  )
}

function MeetingHeader({ meeting }: { meeting: any }) {
  const start = useStartRecording(meeting.id)
  const stop = useStopRecording(meeting.id)
  const reprocess = useReprocess(meeting.id)
  const [error, setError] = useState("")

  async function handleStart() {
    try { await start.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleStop() {
    try { await stop.mutateAsync() } catch (e: any) { setError(e.message) }
  }
  async function handleReprocess() {
    try { await reprocess.mutateAsync() } catch (e: any) { setError(e.message) }
  }

  return (
    <div className="p-4 border-b">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-base font-semibold truncate">{meeting.title}</h2>
        <Badge variant={statusVariant(meeting.status)}>{meeting.status}</Badge>
      </div>
      {error && <p className="text-xs text-destructive mt-1">{error}</p>}
      <div className="flex gap-2 mt-2">
        {(meeting.status === "pending" || meeting.status === "failed") && (
          <Button size="sm" onClick={handleStart} disabled={start.isPending}>
            <Play size={14} className="mr-1" /> Start
          </Button>
        )}
        {meeting.status === "recording" && (
          <Button size="sm" variant="destructive" onClick={handleStop} disabled={stop.isPending}>
            <Square size={14} className="mr-1" /> Stop
          </Button>
        )}
        {(meeting.status === "failed" || meeting.status === "completed") && meeting.transcript && (
          <Button size="sm" variant="outline" onClick={handleReprocess} disabled={reprocess.isPending}>
            <RefreshCw size={14} className="mr-1" /> Reprocessar
          </Button>
        )}
      </div>
    </div>
  )
}

function TranscriptTab({ meeting }: { meeting: any }) {
  const update = useUpdateMeeting(meeting.id)
  const [value, setValue] = useState(meeting.transcript ?? "")
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => { setValue(meeting.transcript ?? "") }, [meeting.transcript])

  function handleChange(v: string) {
    setValue(v)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => update.mutate({ transcript: v }), 1000)
  }

  return (
    <textarea
      value={value}
      onChange={e => handleChange(e.target.value)}
      placeholder="Nenhuma transcrição ainda..."
      className="w-full h-full min-h-[300px] text-sm bg-background border rounded p-3 resize-none focus:outline-none focus:ring-1 focus:ring-primary"
    />
  )
}

function SummaryTab({ meeting }: { meeting: any }) {
  const generate = useGenerateSummary(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          <Wand2 size={14} className="mr-1" />
          {generate.isPending ? "Gerando..." : "Gerar resumo"}
        </Button>
      </div>
      {meeting.summary ? (
        <p className="text-sm leading-relaxed whitespace-pre-wrap">{meeting.summary.content}</p>
      ) : (
        <p className="text-sm text-muted-foreground">Nenhum resumo ainda.</p>
      )}
    </div>
  )
}

function KeyPointsTab({ meeting }: { meeting: any }) {
  const generate = useGenerateKeyPoints(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          <Wand2 size={14} className="mr-1" />
          {generate.isPending ? "Gerando..." : "Gerar pontos"}
        </Button>
      </div>
      {meeting.key_points.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhum ponto-chave ainda.</p>
      ) : (
        <ol className="space-y-2">
          {meeting.key_points.map((kp: any, i: number) => (
            <li key={kp.id} className="flex gap-2 text-sm">
              <span className="text-muted-foreground w-5 flex-shrink-0">{i + 1}.</span>
              <span>{kp.content}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function TasksTab({ meeting }: { meeting: any }) {
  const generate = useGenerateTasks(meeting.id)
  return (
    <div>
      <div className="flex justify-end mb-3">
        <Button size="sm" onClick={() => generate.mutate()} disabled={generate.isPending || !meeting.transcript}>
          <Wand2 size={14} className="mr-1" />
          {generate.isPending ? "Gerando..." : "Gerar tarefas"}
        </Button>
      </div>
      {meeting.tasks.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nenhuma tarefa ainda.</p>
      ) : (
        <ul className="space-y-2">
          {meeting.tasks.map((task: any) => (
            <TaskItem key={task.id} task={task} meetingId={meeting.id} />
          ))}
        </ul>
      )}
    </div>
  )
}

function TaskItem({ task, meetingId }: { task: any; meetingId: string }) {
  const update = useUpdateTask(meetingId, task.id)
  return (
    <li className="flex items-start gap-2 text-sm">
      <input
        type="checkbox"
        checked={task.completed}
        onChange={e => update.mutate({ ...task, completed: e.target.checked })}
        className="mt-0.5"
      />
      <div className={cn("flex-1", task.completed && "line-through text-muted-foreground")}>
        <span>{task.description}</span>
        {task.assignee && <span className="ml-2 text-xs text-muted-foreground">@{task.assignee}</span>}
      </div>
    </li>
  )
}
```

- [ ] Run `cd frontend && npm run build` — must succeed
- [ ] Commit: `feat(frontend): implement MeetingDetail with tabs, recording controls, and AI generation`

---

## Task 10: Toolbar + RecordingModal

- [ ] Replace `frontend/src/components/layout/Toolbar.tsx` with the full implementation:
```typescript
import { Mic } from "lucide-react"
import { Button } from "../ui/button"
import { RecordingModal } from "../recording/RecordingModal"

interface ToolbarProps {
  onRecord: () => void
  recordingModalOpen: boolean
  onRecordingModalClose: () => void
  onMeetingCreated: (id: string) => void
}

export function Toolbar({ onRecord, recordingModalOpen, onRecordingModalClose, onMeetingCreated }: ToolbarProps) {
  return (
    <div className="h-12 border-b flex items-center px-4 gap-3 flex-shrink-0">
      <span className="font-semibold text-sm text-foreground">Meeting Notes</span>
      <div className="flex-1" />
      <Button size="sm" onClick={onRecord}>
        <Mic size={14} className="mr-1.5" /> Gravar Nova Reunião
      </Button>
      <RecordingModal
        open={recordingModalOpen}
        onClose={onRecordingModalClose}
        onMeetingCreated={onMeetingCreated}
      />
    </div>
  )
}
```

- [ ] Create directory `frontend/src/components/recording/`
- [ ] Create `frontend/src/components/recording/RecordingModal.tsx`:
```typescript
import { useState } from "react"
import { useThemes } from "../../hooks/useThemes"
import { useCreateMeeting } from "../../hooks/useMeetings"
import { useStartRecording } from "../../hooks/useMeeting"
import { Button } from "../ui/button"

interface Props {
  open: boolean
  onClose: () => void
  onMeetingCreated: (id: string) => void
}

export function RecordingModal({ open, onClose, onMeetingCreated }: Props) {
  const { data: themes = [] } = useThemes()
  const createMeeting = useCreateMeeting()
  const [title, setTitle] = useState("")
  const [themeId, setThemeId] = useState("")
  const [error, setError] = useState("")
  const [createdId, setCreatedId] = useState<string | null>(null)
  const startRecording = useStartRecording(createdId ?? "")

  if (!open) return null

  async function handleStart() {
    if (!title.trim()) { setError("Título obrigatório"); return }
    setError("")
    try {
      const m = await createMeeting.mutateAsync({ title: title.trim(), theme_id: themeId || undefined })
      setCreatedId(m.id)
      await startRecording.mutateAsync()
      onMeetingCreated(m.id)
      setTitle("")
      setThemeId("")
      setCreatedId(null)
      onClose()
    } catch (e: any) {
      if (e.message.includes("503") || e.message.toLowerCase().includes("unavailable")) {
        setError("Serviço de áudio indisponível")
      } else if (e.message.includes("409")) {
        setError("Já existe uma gravação em andamento")
      } else {
        setError(e.message)
      }
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-background rounded-lg shadow-xl w-96 p-6">
        <h2 className="text-base font-semibold mb-4">Nova Gravação</h2>
        <div className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground">Título</label>
            <input
              autoFocus
              value={title}
              onChange={e => setTitle(e.target.value)}
              onKeyDown={e => { if (e.key === "Enter") handleStart(); if (e.key === "Escape") onClose() }}
              placeholder="Daily 28/04"
              className="w-full mt-1 text-sm border rounded px-3 py-1.5 bg-background focus:outline-none focus:ring-1 focus:ring-primary"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground">Tema (opcional)</label>
            <select
              value={themeId}
              onChange={e => setThemeId(e.target.value)}
              className="w-full mt-1 text-sm border rounded px-3 py-1.5 bg-background focus:outline-none"
            >
              <option value="">Sem tema</option>
              {themes.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
            </select>
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <div className="flex gap-2 justify-end mt-5">
          <Button variant="outline" size="sm" onClick={onClose}>Cancelar</Button>
          <Button size="sm" onClick={handleStart} disabled={createMeeting.isPending || startRecording.isPending}>
            {createMeeting.isPending || startRecording.isPending ? "Iniciando..." : "Iniciar Gravação"}
          </Button>
        </div>
      </div>
    </div>
  )
}
```

**Note on `useStartRecording` in RecordingModal:** `useStartRecording` is called with `createdId ?? ""` at hook initialization time (hooks cannot be called conditionally). After `createMeeting.mutateAsync()` resolves, `setCreatedId(m.id)` is called. However, since React state updates are asynchronous, the `startRecording` hook will still hold the old `createdId` (empty string) when `startRecording.mutateAsync()` is called immediately after. To fix this, call `startRecording` with the ID obtained directly from the `createMeeting` response by using a local variable:

```typescript
// In handleStart, after createMeeting resolves:
const m = await createMeeting.mutateAsync({ title: title.trim(), theme_id: themeId || undefined })
// startRecording hook uses createdId state — we must trigger a re-render first.
// Better approach: use api() directly for the start call instead of the hook.
```

The simplest fix is to import `api` directly and call `api<void>(`/api/meetings/${m.id}/start`, { method: "POST" })` instead of using the `useStartRecording` hook in `handleStart`. This avoids the stale closure issue entirely. Update `RecordingModal.tsx`:

```typescript
import { api } from "../../hooks/useApi"
import { useQueryClient } from "@tanstack/react-query"

// Remove: const startRecording = useStartRecording(createdId ?? "")
// Add at top of component:
const qc = useQueryClient()

// In handleStart, replace startRecording.mutateAsync() with:
await api<void>(`/api/meetings/${m.id}/start`, { method: "POST" })
qc.invalidateQueries({ queryKey: ["meetings"] })
qc.invalidateQueries({ queryKey: ["meeting", m.id] })
```

Apply this corrected version when implementing the file.

- [ ] Run `cd frontend && npm run build` — must succeed
- [ ] Commit: `feat(frontend): implement Toolbar and RecordingModal`

---

## Task 11: Integration smoke test (wails dev)

- [ ] Verify Go builds:
```bash
cd F:/dev/meeting-notes
go build ./cmd/desktop/
```
Expected: compiles without errors (`meeting-notes.exe` or similar output created).

- [ ] Run `wails dev` from project root pointing at `cmd/desktop`:
```bash
wails dev -d cmd/desktop
```
This compiles Go, starts the Vite frontend dev server, and opens the Wails window. Wails v2 handles the `//go:embed` directive automatically in dev mode — no build tags needed.

- [ ] Smoke test checklist (manual, run inside the opened Wails window):
  - [ ] Window opens with title "Meeting Notes"
  - [ ] Toolbar renders with "Gravar Nova Reunião" button
  - [ ] Sidebar shows "Todos" entry and themes list
  - [ ] Selecting a theme in Sidebar filters MeetingList
  - [ ] Clicking "Novo tema" in Sidebar opens inline input; Enter creates theme and it appears in the list
  - [ ] Clicking "Nova reunião" in MeetingList opens inline input; Enter creates meeting and selects it
  - [ ] Clicking a meeting in MeetingList opens MeetingDetail on the right
  - [ ] MeetingDetail tabs work: Transcrição, Resumo, Pontos-chave, Tarefas all render
  - [ ] Transcript textarea is editable; changes auto-save after 1s
  - [ ] "Gravar Nova Reunião" button opens RecordingModal
  - [ ] RecordingModal: fill title, optionally select theme, click "Iniciar Gravação" — creates meeting, starts recording, modal closes, meeting selected in list
  - [ ] If audio service is down, RecordingModal shows "Serviço de áudio indisponível"
  - [ ] Meeting status badge in MeetingList updates in real time as pipeline progresses (transcribing → processing → completed) via Wails events — no page refresh needed

- [ ] If `wails dev` fails due to missing `frontend/dist` for embed: this should not happen in dev mode; Wails v2 uses its own asset server. If it does occur, run `cd frontend && npm run build` first to create `frontend/dist`, then retry `wails dev -d cmd/desktop`.

- [ ] Commit: `feat: complete Fatia 8 — Wails desktop app with React frontend`

---

## Self-review

**Spec coverage verification:**

| Spec requirement | Plan task |
|---|---|
| `cmd/desktop/main.go` Wails entrypoint | Task 3 |
| `cmd/desktop/app.go` with OnStartup/OnShutdown/GetPort | Task 3 |
| `cmd/desktop/app_test.go` with 3 tests | Task 3 |
| Orchestrator notifyFn field + SetNotifyFn + notify helper | Task 2 |
| notify called in RunCapturePipeline, RunAIPipeline, markFailed | Task 2 |
| `TestOrchestrator_NotifyFn_CalledOnStatusChange` test | Task 2 |
| `wails.json` config | Task 1 |
| React + Vite + TypeScript scaffold | Task 1 |
| Tailwind + CSS variables | Task 4 |
| Shadcn Badge + Button (manual, no CLI) | Task 4 |
| React Query v5 | Task 4 |
| `useApi.ts` with `initApi` and `api<T>` | Task 5 |
| `useThemes.ts` | Task 5 |
| `useMeetings.ts` with themeId filter | Task 5 |
| `useMeeting.ts` with all mutations | Task 5 |
| `usePipeline.ts` with EventsOn | Task 5 |
| App.tsx shell with GetPort boot and three-column layout | Task 6 |
| Sidebar with theme list, count badges, inline create | Task 7 |
| MeetingList with status badges, date, inline create | Task 8 |
| MeetingDetail with 4 tabs and recording controls | Task 9 |
| Transcript textarea debounced PUT | Task 9 |
| Generate buttons for summary, key_points, tasks | Task 9 |
| Task checkbox toggle | Task 9 |
| Toolbar with Gravar button | Task 10 |
| RecordingModal with 503/409 error handling | Task 10 |
| Smoke test checklist | Task 11 |
| `cmd/api/main.go` unchanged | Not modified in any task |

**Type consistency verification:**

- `useStartRecording(id: string)` — used in `MeetingDetail/MeetingHeader` with `meeting.id` (string): correct.
- `useStartRecording` in `RecordingModal` — the stale closure issue is identified and fixed in Task 10 notes: use `api()` directly instead of the hook for the post-create start call.
- `useMeetings(themeId?: string | null)` — called as `useMeetings()` in Sidebar (no arg, gets all) and `useMeetings(themeId)` in MeetingList: correct.
- `useUpdateTask(meetingId: string, taskId: string)` — called in `TaskItem` with `meetingId` and `task.id`: both strings, correct.
- `Badge variant` prop — `badgeVariants` defines variants including `recording`, `transcribing`, `processing`, `completed`, `failed`, `pending` matching all `Meeting["status"]` values: complete.

**RecordingModal `createdId` pattern — double check:**

The hook `useStartRecording(createdId ?? "")` is initialized with `""` and only gets a real ID after `createMeeting.mutateAsync()` resolves and `setCreatedId` is called. Since React batches state updates, calling `startRecording.mutateAsync()` immediately after `setCreatedId` would still use the stale `""` ID. The fix documented in Task 10 (use `api()` directly with `m.id`) is correct and avoids this race condition entirely. The `createdId` state and `useStartRecording` hook import can be removed from the final implementation.
