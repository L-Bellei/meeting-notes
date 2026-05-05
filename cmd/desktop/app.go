package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/wailsapp/wails/v2/pkg/options"
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
	ctx        context.Context
	db         *sql.DB
	port       int
	server     *http.Server
	audioProc  *exec.Cmd
	allowQuit  bool
	tray       *TrayManager
	portReady  chan struct{}
}

func NewApp() *App { return &App{portReady: make(chan struct{})} }

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	cfg := config.Load()

	// Ensure portReady is always closed when OnStartup exits, even on error.
	// On the success path it is closed early (before tray.Start) so GetPort()
	// doesn't block while the tray is setting up Win32 internals.
	var closeOnce sync.Once
	signalPort := func() { closeOnce.Do(func() { close(a.portReady) }) }
	defer signalPort()

	a.startAudioService(ctx, cfg.AudioServiceURL)

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Printf("open db: %v", err)
		return
	}
	a.db = db

	logRepo := repository.NewLogRepository(db)
	themeRepo := repository.NewThemeRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	boardColumnRepo := repository.NewBoardColumnRepository(db)
	boardCardRepo := repository.NewBoardCardRepository(db)
	searchRepo := repository.NewSearchRepository(db)

	aiClient := ai.NewDynamicAIClient(settingsRepo)

	boardColumnSvc := services.NewBoardColumnService(boardColumnRepo)
	boardCardSvc := services.NewBoardCardService(boardCardRepo, boardColumnRepo, meetingRepo, summaryRepo, keyPointRepo, taskRepo)
	themeSvc := services.NewThemeService(themeRepo)
	meetingSvc := services.NewMeetingService(meetingRepo, themeRepo, searchRepo, keyPointRepo, taskRepo, summaryRepo)
	summarySvc := services.NewSummaryService(summaryRepo, aiClient)
	keyPointSvc := services.NewKeyPointService(keyPointRepo, aiClient)
	taskSvc := services.NewTaskService(taskRepo, aiClient)
	settingsSvc := services.NewSettingsService(settingsRepo)
	searchSvc := services.NewSearchService(searchRepo, meetingRepo)

	audioClient := audio.NewHTTPClient(cfg.AudioServiceURL)
	orch := services.NewOrchestrator(meetingRepo, themeRepo, summarySvc, keyPointSvc, taskSvc, audioClient, settingsRepo, boardCardSvc)
	orch.SetSearchRepo(searchRepo)
	orch.SetLogRepo(logRepo)
	orch.SetNotifyFn(func(meetingID, status string) {
		if isWailsContext(ctx) {
			type payload struct {
				MeetingID string `json:"meeting_id"`
				Status    string `json:"status"`
			}
			wailsruntime.EventsEmit(ctx, "pipeline:status", payload{MeetingID: meetingID, Status: status})
		}
		switch status {
		case "recording":
			if a.tray != nil {
				if a.tray.overlay != nil {
					a.tray.overlay.Show(ctx, a.port, meetingID)
				}
				a.tray.UpdateState(true)
			}
		case "transcribing", "processing", "completed", "failed":
			if a.tray != nil {
				if a.tray.overlay != nil {
					a.tray.overlay.Hide()
				}
				a.tray.UpdateState(false)
			}
		}
	})

	boardHandler := handlers.NewBoardHandler(boardColumnSvc, boardCardSvc)
	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo, orch)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc, themeRepo)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc, themeRepo)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc, themeRepo)
	settingsHandler := handlers.NewSettingsHandler(settingsSvc)
	settingsHandler.SetOnUpdate(func(s map[string]string) {
		if a.tray != nil {
			a.tray.ApplySettings(s)
		}
	})
	searchHandler := handlers.NewSearchHandler(searchSvc)
	logHandler := handlers.NewLogHandler(logRepo)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		modelLoaded := false
		if h, err := audioClient.Health(r.Context()); err == nil {
			modelLoaded = h.ModelLoaded
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status":       "ok",
			"model_loaded": modelLoaded,
		})
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

	r.Route("/api/settings", func(r chi.Router) {
		r.Get("/", settingsHandler.Get)
		r.Put("/", settingsHandler.Update)
	})

	r.Route("/api/board", func(r chi.Router) {
		r.Get("/columns", boardHandler.ListColumns)
		r.Post("/columns", boardHandler.CreateColumn)
		r.Patch("/columns/reorder", boardHandler.ReorderColumns)
		r.Put("/columns/{id}", boardHandler.UpdateColumn)
		r.Delete("/columns/{id}", boardHandler.DeleteColumn)
		r.Get("/cards", boardHandler.ListCards)
		r.Post("/cards", boardHandler.CreateCard)
		r.Post("/cards/manual", boardHandler.CreateManualCard)
		r.Get("/cards/{id}", boardHandler.GetCard)
		r.Put("/cards/{id}", boardHandler.UpdateCard)
		r.Delete("/cards/{id}", boardHandler.DeleteCard)
		r.Patch("/cards/{id}/move", boardHandler.MoveCard)
		r.Patch("/cards/{id}/link", boardHandler.LinkCardToMeeting)
	})

	r.Get("/api/search", searchHandler.Search)
	r.Get("/api/logs", logHandler.List)

	aiHealthHandler := handlers.NewAIHealthHandler(func(ctx context.Context) (bool, error) {
		return ai.Ping(ctx, settingsRepo)
	})
	r.Get("/api/ai/health", aiHealthHandler.Check)

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Printf("listen: %v", err)
		return
	}
	a.port = ln.Addr().(*net.TCPAddr).Port
	a.server = &http.Server{Handler: r}
	go a.server.Serve(ln)
	signalPort() // unblock GetPort() before tray.Start(), which may be slow on first boot

	a.tray = NewTrayManager(a, orch, meetingRepo, meetingSvc, settingsRepo)
	if err := a.tray.Start(ctx); err != nil {
		log.Printf("tray: %v", err)
	}
}

func (a *App) onSecondInstanceLaunch(_ options.SecondInstanceData) {
	wailsruntime.Show(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
}

func (a *App) OnBeforeClose(ctx context.Context) bool {
	if a.allowQuit {
		return false
	}
	wailsruntime.Hide(ctx)
	return true
}

func (a *App) OnShutdown(ctx context.Context) {
	if a.tray != nil {
		a.tray.Stop()
	}
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

// GetPort blocks until OnStartup has bound the HTTP server, then returns the
// port. A 30-second timeout guards against a startup failure where the port is
// never set (returns 0, which the frontend will surface as a server error).
func (a *App) GetPort() int {
	select {
	case <-a.portReady:
	case <-time.After(30 * time.Second):
	}
	return a.port
}

// startAudioService launches the audio service. It prefers the bundled PyInstaller
// executable next to the main EXE (production), falling back to uvicorn via the
// project venv (development). Skips spawning if the port is already in use.
func (a *App) startAudioService(ctx context.Context, audioURL string) {
	if a.audioServiceAlive(audioURL) {
		log.Printf("audio service already running, skipping start")
		go a.waitAudioReady(ctx, audioURL)
		return
	}

	var cmd *exec.Cmd

	// Production: bundled audio-service.exe next to the main executable.
	if exe, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exe), "audio-service", "audio-service.exe")
		if _, err := os.Stat(bundled); err == nil {
			recordingsDir := bundledRecordingsDir()
			c := exec.Command(bundled, "--port", "8765")
			c.Env = append(os.Environ(), "RECORDINGS_DIR="+recordingsDir)
			c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			cmd = c
			log.Printf("starting bundled audio service from %s", bundled)
		}
	}

	// Development: uvicorn via the project venv.
	if cmd == nil {
		dir := findAudioServiceDir()
		if dir == "" {
			log.Printf("audio-service not found; skipping auto-start")
			return
		}
		uvicorn := filepath.Join(dir, ".venv", "Scripts", "uvicorn.exe")
		if _, err := os.Stat(uvicorn); err != nil {
			log.Printf("uvicorn not found at %s; skipping audio service auto-start", uvicorn)
			return
		}
		c := exec.Command(uvicorn, "main:app", "--port", "8765")
		c.Dir = dir
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		cmd = c
		log.Printf("starting audio service via uvicorn from %s", dir)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("start audio service: %v", err)
		return
	}
	a.audioProc = cmd
	log.Printf("audio service started (pid %d)", cmd.Process.Pid)

	go a.waitAudioReady(ctx, audioURL)
}

// bundledRecordingsDir returns %AppData%\Meeting Notes\recordings, creating it if needed.
func bundledRecordingsDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		p := filepath.Join(dir, "Meeting Notes", "recordings")
		_ = os.MkdirAll(p, 0755)
		return p
	}
	return filepath.Join(".", "recordings")
}

func (a *App) audioServiceAlive(audioURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(audioURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
				log.Printf("audio service ready")
				if isWailsContext(ctx) {
					wailsruntime.EventsEmit(ctx, "audio:ready", nil)
				}
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	log.Printf("audio service did not become ready within 90 s")
}

// findAudioServiceDir searches for the audio-service directory relative to both
// the executable path and the working directory, covering dev and production layouts.
func findAudioServiceDir() string {
	var roots []string

	// Prefer exe-relative lookup so the built binary works regardless of CWD.
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		roots = append(roots,
			exeDir,                                    // <exe-dir>/audio-service
			filepath.Join(exeDir, ".."),               // one level up
			filepath.Join(exeDir, "..", ".."),         // two levels up  (build/bin → cmd/desktop)
			filepath.Join(exeDir, "..", "..", ".."),   // three levels up (cmd/desktop → cmd)
			filepath.Join(exeDir, "..", "..", "..", ".."), // four levels up (build/bin → project root)
		)
	}

	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots,
			cwd,
			filepath.Join(cwd, ".."),
			filepath.Join(cwd, "..", ".."),
		)
	}

	for _, root := range roots {
		p := filepath.Join(root, "audio-service")
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

// isWailsContext returns true when ctx carries a valid Wails application context.
// It prevents log.Fatal panics from the Wails runtime when code runs outside
// of the Wails lifecycle (e.g. in unit tests).
func isWailsContext(ctx context.Context) bool {
	return ctx.Value("events") != nil
}
