package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
	"smclipper-backend/internal/jobs"
)

type ProjectHandler struct {
	DB          *pgxpool.Pool
	Queue       *jobs.Queue
	StoragePath string
}

func NewProjectHandler(db *pgxpool.Pool, q *jobs.Queue, storagePath string) *ProjectHandler {
	return &ProjectHandler{DB: db, Queue: q, StoragePath: storagePath}
}

// POST /api/projects
// POST /api/projects
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.YoutubeURL == "" {
		respondError(w, http.StatusBadRequest, "youtube_url is required")
		return
	}

	// Ambil metadata (title, channel) sebelum simpan - tidak mendownload video
	title := ""
	channelName := ""
	meta, err := jobs.FetchYoutubeMetadata(req.YoutubeURL)
	if err == nil {
		title = meta.Title
		channelName = meta.Channel
	}
	// Kalau fetch metadata gagal, kita tetap lanjut simpan project dengan title kosong
	// (jangan blokir user hanya karena metadata gagal diambil)

	var project models.Project
	query := `
		INSERT INTO projects (youtube_url, title, channel_name, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, youtube_url, COALESCE(title, ''), COALESCE(channel_name, ''), status, created_at, updated_at
	`

	err = h.DB.QueryRow(context.Background(), query, req.YoutubeURL, title, channelName).Scan(
		&project.ID,
		&project.YoutubeURL,
		&project.Title,
		&project.ChannelName,
		&project.Status,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create project: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, project)
}

// GET /api/projects/{id}
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var project models.Project
	query := `
		SELECT id, youtube_url, COALESCE(title, ''), COALESCE(channel_name, ''), status, created_at, updated_at
		FROM projects
		WHERE id = $1
	`

	err := h.DB.QueryRow(context.Background(), query, id).Scan(
		&project.ID, 
		&project.YoutubeURL, 
		&project.Title, 
		&project.ChannelName,
		&project.Status, 
		&project.CreatedAt, 
		&project.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "project not found: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, project)
}

// GET /api/projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, youtube_url, COALESCE(title, ''), COALESCE(channel_name, ''), status, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`

	rows, err := h.DB.Query(context.Background(), query)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list projects: "+err.Error())
		return
	}
	defer rows.Close()

	projects := []models.Project{}
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.YoutubeURL, &p.Title, &p.ChannelName, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to scan project: "+err.Error())
			return
		}
		projects = append(projects, p)
	}

	respondJSON(w, http.StatusOK, projects)
}

// POST /api/projects/{id}/download
func (h *ProjectHandler) TriggerDownload(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var jobID string
	err := h.DB.QueryRow(context.Background(), `
		INSERT INTO jobs (project_id, job_type, status)
		VALUES ($1, 'download', 'queued')
		RETURNING id
	`, projectID).Scan(&jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create job: "+err.Error())
		return
	}

	h.Queue.Enqueue(jobID)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"job_id": jobID,
		"status": "queued",
	})
}

// POST /api/projects/{id}/transcribe
func (h *ProjectHandler) TriggerTranscribe(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var jobID string
	err := h.DB.QueryRow(context.Background(), `
		INSERT INTO jobs (project_id, job_type, status)
		VALUES ($1, 'transcribe', 'queued')
		RETURNING id
	`, projectID).Scan(&jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create job: "+err.Error())
		return
	}

	h.Queue.Enqueue(jobID)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"job_id": jobID,
		"status": "queued",
	})
}

// DELETE /api/projects/{id}
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Hapus row di database dulu (cascade otomatis hapus videos, transcripts, jobs, clip_candidates, dll)
	tag, err := h.DB.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete project: "+err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, http.StatusNotFound, "project not found")
		return
	}

	// Hapus folder fisik di disk (video, audio, transcript, clips, dll)
	projectDir := filepath.Join(h.StoragePath, id)
	if err := os.RemoveAll(projectDir); err != nil {
		// Data DB sudah terhapus, tapi file fisik gagal dihapus - beri tahu user tapi tetap anggap sukses
		respondJSON(w, http.StatusOK, map[string]string{
			"status":  "deleted",
			"warning": "database record deleted, but failed to remove files: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}