package main

import (
	"encoding/json"
	"log"
	"net/http"

	"smclipper-backend/internal/config"
	"smclipper-backend/internal/database"
	"smclipper-backend/internal/handlers"
	"smclipper-backend/internal/jobs"
)

func main() {
	cfg := config.Load()

	db := database.Connect(cfg.DSN())
	defer db.Close()

	queue := jobs.NewQueue(db, cfg.StoragePath, cfg.WhisperBinPath, cfg.WhisperModelPath, 2)

	projectHandler := handlers.NewProjectHandler(db, queue)
	jobHandler := handlers.NewJobHandler(db)
	transcriptHandler := handlers.NewTranscriptHandler(db)
	clipHandler := handlers.NewClipHandler(db)
	renderConfigHandler := handlers.NewRenderConfigHandler(db)
	subtitleHandler := handlers.NewSubtitleHandler(db, cfg.StoragePath)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	mux.HandleFunc("POST /api/projects", projectHandler.CreateProject)
	mux.HandleFunc("GET /api/projects", projectHandler.ListProjects)
	mux.HandleFunc("GET /api/projects/{id}", projectHandler.GetProject)
	mux.HandleFunc("POST /api/projects/{id}/download", projectHandler.TriggerDownload)
	mux.HandleFunc("POST /api/projects/{id}/transcribe", projectHandler.TriggerTranscribe)

	mux.HandleFunc("GET /api/jobs/{id}", jobHandler.GetJob)

	mux.HandleFunc("GET /api/projects/{id}/transcript", transcriptHandler.GetTranscript)
	mux.HandleFunc("GET /api/projects/{id}/transcript/export", transcriptHandler.DownloadTranscript)

	mux.HandleFunc("POST /api/projects/{id}/clips/import", clipHandler.ImportClips)
	mux.HandleFunc("GET /api/projects/{id}/clips", clipHandler.ListClips)
	mux.HandleFunc("POST /api/projects/{id}/clips", clipHandler.CreateClip)
	mux.HandleFunc("PUT /api/clips/{id}", clipHandler.UpdateClip)
	mux.HandleFunc("DELETE /api/clips/{id}", clipHandler.DeleteClip)

	mux.HandleFunc("PUT /api/clips/{id}/render-config", renderConfigHandler.UpsertRenderConfig)
	mux.HandleFunc("GET /api/clips/{id}/render-config", renderConfigHandler.GetRenderConfig)

	mux.HandleFunc("POST /api/clips/{id}/subtitle/generate", subtitleHandler.GenerateSubtitle)
	mux.HandleFunc("GET /api/clips/{id}/subtitle/preview", subtitleHandler.PreviewSubtitle)

	handler := corsMiddleware(mux)

	log.Printf("Server starting on :%s", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, handler); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "smclipper-backend",
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}