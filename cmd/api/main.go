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

	// Repositories
	themeRepo := repository.NewThemeRepository(db)
	meetingRepo := repository.NewMeetingRepository(db)
	summaryRepo := repository.NewSummaryRepository(db)
	keyPointRepo := repository.NewKeyPointRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	boardColumnRepo := repository.NewBoardColumnRepository(db)
	boardCardRepo := repository.NewBoardCardRepository(db)

	aiClient := ai.NewDynamicAIClient(settingsRepo)

	// Services
	boardColumnSvc := services.NewBoardColumnService(boardColumnRepo)
	boardCardSvc := services.NewBoardCardService(boardCardRepo, boardColumnRepo, meetingRepo, summaryRepo, keyPointRepo, taskRepo)
	themeSvc := services.NewThemeService(themeRepo)
	meetingSvc := services.NewMeetingService(meetingRepo, themeRepo)
	summarySvc := services.NewSummaryService(summaryRepo, aiClient)
	keyPointSvc := services.NewKeyPointService(keyPointRepo, aiClient)
	taskSvc := services.NewTaskService(taskRepo, aiClient)
	settingsSvc := services.NewSettingsService(settingsRepo)

	// Audio + Orchestrator
	audioClient := audio.NewHTTPClient(cfg.AudioServiceURL)
	orchestrator := services.NewOrchestrator(meetingRepo, themeRepo, summarySvc, keyPointSvc, taskSvc, audioClient, settingsRepo, boardCardSvc)

	// Handlers
	boardHandler := handlers.NewBoardHandler(boardColumnSvc, boardCardSvc)
	themeHandler := handlers.NewThemeHandler(themeSvc)
	meetingHandler := handlers.NewMeetingHandler(meetingSvc, summaryRepo, keyPointRepo, taskRepo, orchestrator)
	summaryHandler := handlers.NewSummaryHandler(summarySvc, meetingSvc, themeRepo)
	keyPointHandler := handlers.NewKeyPointHandler(keyPointSvc, meetingSvc, themeRepo)
	taskHandler := handlers.NewTaskHandler(taskSvc, meetingSvc, themeRepo)
	settingsH := handlers.NewSettingsHandler(settingsSvc)

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

	r.Route("/api/settings", func(r chi.Router) {
		r.Get("/", settingsH.Get)
		r.Put("/", settingsH.Update)
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
