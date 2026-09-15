package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
	"smclipper-backend/internal/jobs"
)

type ProjectHandler struct {
	DB    *pgxpool.Pool
	Queue *jobs.Queue
}

func NewProjectHandler(db *pgxpool.Pool, q *jobs.Queue) *ProjectHandler {
	return &ProjectHandler{DB: db, Queue: q}
}

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

	var project models.Project
	query := `
		INSERT INTO projects (youtube_url, status)
		VALUES ($1, 'pending')
		RETURNING id, youtube_url, COALESCE(title, '') as title, status, created_at, updated_at
	`

	err := h.DB.QueryRow(context.Background(), query, req.YoutubeURL).Scan(
		&project.ID,
		&project.YoutubeURL,
		&project.Title,
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
		SELECT id, youtube_url, title, status, created_at, updated_at
		FROM projects
		WHERE id = $1
	`

	err := h.DB.QueryRow(context.Background(), query, id).Scan(
		&project.ID,
		&project.YoutubeURL,
		&project.Title,
		&project.Status,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "project not found")
		return
	}

	respondJSON(w, http.StatusOK, project)
}

// GET /api/projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, youtube_url, COALESCE(title, '') as title, status, created_at, updated_at
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
		if err := rows.Scan(&p.ID, &p.YoutubeURL, &p.Title, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
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