package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strings"

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

	var projectTitle string
	err = h.DB.QueryRow(context.Background(), `SELECT COALESCE(title, '') FROM projects WHERE id = $1`, projectID).Scan(&projectTitle)
	if err != nil || projectTitle == "" {
		projectTitle = "transcript"
	}
	baseName := sanitizeFilename(projectTitle)

	var filePath, downloadName, contentType string
	if format == "json" {
		filePath, downloadName, contentType = rawPath, baseName+".json", "application/json"
	} else {
		filePath, downloadName, contentType = txtPath, baseName+".txt", "text/plain"
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+downloadName+"\"")
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, filePath)
}

var unsafeFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)

// sanitizeFilename membersihkan title video supaya aman dipakai sebagai nama file
// di semua OS (Windows/Linux/Mac), dan membatasi panjangnya.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "transcript"
	}
	name = unsafeFilenameChars.ReplaceAllString(name, "")
	name = strings.Join(strings.Fields(name), " ") // rapikan spasi ganda

	const maxLen = 100
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	return name
}