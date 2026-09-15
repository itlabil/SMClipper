package handlers

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
)

type TranscriptHandler struct {
	DB *pgxpool.Pool
}

func NewTranscriptHandler(db *pgxpool.Pool) *TranscriptHandler {
	return &TranscriptHandler{DB: db}
}

// GET /api/projects/{id}/transcript
func (h *TranscriptHandler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var t models.Transcript
	query := `
		SELECT id, project_id, raw_json_path, txt_export_path, language, created_at
		FROM transcripts WHERE project_id = $1
		ORDER BY created_at DESC LIMIT 1
	`
	err := h.DB.QueryRow(context.Background(), query, projectID).Scan(
		&t.ID, &t.ProjectID, &t.RawJSONPath, &t.TxtExportPath, &t.Language, &t.CreatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "transcript not found")
		return
	}

	respondJSON(w, http.StatusOK, t)
}

// GET /api/projects/{id}/transcript/export?format=txt|json
func (h *TranscriptHandler) DownloadTranscript(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "txt"
	}

	var rawPath, txtPath string
	query := `SELECT raw_json_path, txt_export_path FROM transcripts WHERE project_id = $1 ORDER BY created_at DESC LIMIT 1`
	err := h.DB.QueryRow(context.Background(), query, projectID).Scan(&rawPath, &txtPath)
	if err != nil {
		respondError(w, http.StatusNotFound, "transcript not found")
		return
	}

	var filePath, downloadName, contentType string
	if format == "json" {
		filePath, downloadName, contentType = rawPath, "transcript.json", "application/json"
	} else {
		filePath, downloadName, contentType = txtPath, "transcript.txt", "text/plain"
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadName+"\"")
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, filePath)
}