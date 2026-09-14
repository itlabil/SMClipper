package main

import (
	"encoding/json"
	"log"
	"net/http"

	"smclipper-backend/internal/config"
	"smclipper-backend/internal/database"
	"smclipper-backend/internal/handlers"
)

func main() {
	cfg := config.Load()

	db := database.Connect(cfg.DSN())
	defer db.Close()

	projectHandler := handlers.NewProjectHandler(db)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	mux.HandleFunc("POST /api/projects", projectHandler.CreateProject)
	mux.HandleFunc("GET /api/projects", projectHandler.ListProjects)
	mux.HandleFunc("GET /api/projects/{id}", projectHandler.GetProject)

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

// corsMiddleware supaya nanti frontend Vue (beda port) bisa akses API ini
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