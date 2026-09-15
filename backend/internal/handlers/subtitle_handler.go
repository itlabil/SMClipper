package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/jobs"
)

type SubtitleHandler struct {
	DB          *pgxpool.Pool
	StoragePath string
}

func NewSubtitleHandler(db *pgxpool.Pool, storagePath string) *SubtitleHandler {
	return &SubtitleHandler{DB: db, StoragePath: storagePath}
}

// POST /api/clips/{id}/subtitle/generate
func (h *SubtitleHandler) GenerateSubtitle(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")
	ctx := context.Background()

	var projectID string
	var startMs, endMs int
	err := h.DB.QueryRow(ctx, `SELECT project_id, start_ms, end_ms FROM clip_candidates WHERE id = $1`, clipID).
		Scan(&projectID, &startMs, &endMs)
	if err != nil {
		respondError(w, http.StatusNotFound, "clip not found")
		return
	}

	var rawJSONPath string
	err = h.DB.QueryRow(ctx, `
		SELECT raw_json_path FROM transcripts WHERE project_id = $1 ORDER BY created_at DESC LIMIT 1
	`, projectID).Scan(&rawJSONPath)
	if err != nil {
		respondError(w, http.StatusNotFound, "transcript not found for this project")
		return
	}

	outputDir := filepath.Join(h.StoragePath, projectID, "clips", clipID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create output directory")
		return
	}
	outputPath := filepath.Join(outputDir, "subtitle.ass")

	if err := jobs.GenerateASSSubtitle(rawJSONPath, startMs, endMs, outputPath); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate subtitle: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"subtitle_path": outputPath,
	})
}

// GET /api/clips/{id}/subtitle/preview
func (h *SubtitleHandler) PreviewSubtitle(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")
	ctx := context.Background()

	var projectID string
	err := h.DB.QueryRow(ctx, `SELECT project_id FROM clip_candidates WHERE id = $1`, clipID).Scan(&projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "clip not found")
		return
	}

	subtitlePath := filepath.Join(h.StoragePath, projectID, "clips", clipID, "subtitle.ass")
	if _, err := os.Stat(subtitlePath); err != nil {
		respondError(w, http.StatusNotFound, "subtitle not generated yet")
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	http.ServeFile(w, r, subtitlePath)
}