package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
		log.Printf("open db: %v", err)
		return
	}
	a.db = db

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
	orch.SetNotifyFn(func(meetingID, status string) {
		if isWailsContext(ctx) {
			type payload struct {
				MeetingID string `json:"meeting_id"`
				Status    string `json:"status"`
			}
			wailsruntime.EventsEmit(ctx, "pipeline:status", payload{MeetingID: meetingID, Status: status})
		}
	})

	boardHandler := handlers.NewBoardHandler(boardColumnSvc, boardCardSvc)
	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo, orch)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc, themeRepo)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc, themeRepo)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc, themeRepo)
	settingsHandler := handlers.NewSettingsHandler(settingsSvc)
	searchHandler := handlers.NewSearchHandler(searchSvc)

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

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Printf("listen: %v", err)
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
// project venv. If the service is already responding it skips spawning a new process.
func (a *App) startAudioService(ctx context.Context, audioURL string) {
	// If something is already listening on the audio port, reuse it.
	if a.audioServiceAlive(audioURL) {
		log.Printf("audio service already running, skipping start")
		go a.waitAudioReady(ctx, audioURL)
		return
	}

	dir := findAudioServiceDir()
	if dir == "" {
		log.Printf("audio-service directory not found; skipping auto-start")
		return
	}

	uvicorn := filepath.Join(dir, ".venv", "Scripts", "uvicorn.exe")
	if _, err := os.Stat(uvicorn); err != nil {
		log.Printf("uvicorn not found at %s; skipping audio service auto-start", uvicorn)
		return
	}

	cmd := exec.Command(uvicorn, "main:app", "--port", "8765")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Printf("start audio service: %v", err)
		return
	}
	a.audioProc = cmd
	log.Printf("audio service started (pid %d)", cmd.Process.Pid)

	go a.waitAudioReady(ctx, audioURL)
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

// isWailsContext returns true when ctx carries a valid Wails application context.
// It prevents log.Fatal panics from the Wails runtime when code runs outside
// of the Wails lifecycle (e.g. in unit tests).
func isWailsContext(ctx context.Context) bool {
	return ctx.Value("events") != nil
}
