package handlers

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
)

type JobHandler struct {
	DB *pgxpool.Pool
}

func NewJobHandler(db *pgxpool.Pool) *JobHandler {
	return &JobHandler{DB: db}
}

// GET /api/jobs/{id}
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var job models.Job
	query := `
		SELECT id, project_id, job_type, status, progress, error_message, created_at, updated_at
		FROM jobs WHERE id = $1
	`
	err := h.DB.QueryRow(context.Background(), query, id).Scan(
		&job.ID, &job.ProjectID, &job.JobType, &job.Status,
		&job.Progress, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}