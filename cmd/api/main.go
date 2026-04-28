package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"meeting-notes/internal/ai"
	"meeting-notes/internal/audio"
	"meeting-notes/internal/config"
	"meeting-notes/internal/database"
	"meeting-notes/internal/handlers"
	"meeting-notes/internal/repository"
	"meeting-notes/internal/services"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// AI client (nil-safe — generate endpoints return 503 when API key absent)
	var aiClient ai.AIClient
	if cfg.AnthropicAPIKey != "" {
		aiClient = ai.NewAnthropicClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
	}

	// Repositories
	themeRepo := repository.NewThemeRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)

	// Services
	themeSvc := services.NewThemeService(themeRepo)
	meetingSvc := services.NewMeetingService(meetingRepo, themeRepo)
	summarySvc := services.NewSummaryService(summaryRepo, aiClient)
	keyPointSvc := services.NewKeyPointService(keyPointRepo, aiClient)
	taskSvc := services.NewTaskService(taskRepo, aiClient)

	// Audio + Orchestrator
	audioClient := audio.NewHTTPClient(cfg.AudioServiceURL)
	orchestrator := services.NewOrchestrator(meetingRepo, summarySvc, keyPointSvc, taskSvc, audioClient, cfg.WhisperLanguage)

	// Handlers
	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo, orchestrator)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/health", healthHandler(db))

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

	log.Printf("server listening on :%s", cfg.HTTPPort)
	if err := http.ListenAndServe(":"+cfg.HTTPPort, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		statusCode := http.StatusOK

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			dbStatus = "error"
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": dbStatus,
		})
	}
}
