package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

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
	ctx       context.Context
	db        *sql.DB
	port      int
	server    *http.Server
	audioProc *exec.Cmd
}

func NewApp() *App { return &App{} }

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	cfg := config.Load()

	a.startAudioService(ctx, cfg.AudioServiceURL)

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
	if a.audioProc != nil && a.audioProc.Process != nil {
		a.audioProc.Process.Kill()
	}
}

func (a *App) GetPort() int { return a.port }

// startAudioService locates the audio-service directory and launches uvicorn via the
// project venv. It returns immediately after spawning — the caller should rely on the
// audio client's health-check for readiness.
func (a *App) startAudioService(ctx context.Context, audioURL string) {
	dir := findAudioServiceDir()
	if dir == "" {
		wailsruntime.LogWarningf(ctx, "audio-service directory not found; skipping auto-start")
		return
	}

	uvicorn := filepath.Join(dir, ".venv", "Scripts", "uvicorn.exe")
	if _, err := os.Stat(uvicorn); err != nil {
		wailsruntime.LogWarningf(ctx, "uvicorn not found at %s; skipping audio service auto-start", uvicorn)
		return
	}

	cmd := exec.Command(uvicorn, "main:app", "--port", "8765")
	cmd.Dir = dir
	// Redirect output to the Wails log so it's visible in dev console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		wailsruntime.LogErrorf(ctx, "start audio service: %v", err)
		return
	}
	a.audioProc = cmd
	wailsruntime.LogInfof(ctx, "audio service started (pid %d)", cmd.Process.Pid)

	go a.waitAudioReady(ctx, audioURL)
}

// waitAudioReady polls the audio service health endpoint and emits a frontend event
// once it's up. Gives up after 90 seconds (model load can take ~30 s on first run).
func (a *App) waitAudioReady(ctx context.Context, audioURL string) {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(audioURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				wailsruntime.LogInfof(ctx, "audio service ready")
				wailsruntime.EventsEmit(ctx, "audio:ready", nil)
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	wailsruntime.LogWarningf(ctx, "audio service did not become ready within 90 s")
}

// findAudioServiceDir searches common locations relative to the working directory.
func findAudioServiceDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(cwd, "..", "..", "audio-service"), // wails dev from cmd/desktop/
		filepath.Join(cwd, "audio-service"),             // run from project root
		filepath.Join(cwd, "..", "audio-service"),       // one level up
	}
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs
		}
	}
	return ""
}
