package handlers

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VideoHandler struct {
	DB *pgxpool.Pool
}

func NewVideoHandler(db *pgxpool.Pool) *VideoHandler {
	return &VideoHandler{DB: db}
}

// GET /api/projects/{id}/video
func (h *VideoHandler) StreamVideo(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var sourcePath string
	err := h.DB.QueryRow(context.Background(), `SELECT source_path FROM videos WHERE project_id = $1`, projectID).
		Scan(&sourcePath)
	if err != nil {
		respondError(w, http.StatusNotFound, "video not found")
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, sourcePath)
}