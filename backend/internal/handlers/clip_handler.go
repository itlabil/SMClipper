package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
	"smclipper-backend/internal/jobs"
)

type ClipHandler struct {
	DB    *pgxpool.Pool
	Queue *jobs.Queue
}

func NewClipHandler(db *pgxpool.Pool, q *jobs.Queue) *ClipHandler {
	return &ClipHandler{DB: db, Queue: q}
}

// POST /api/projects/{id}/clips/import
func (h *ClipHandler) ImportClips(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var req models.ImportClipsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(req.Clips) == 0 {
		respondError(w, http.StatusBadRequest, "no clips found in payload")
		return
	}

	ctx := context.Background()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to start transaction: "+err.Error())
		return
	}
	defer tx.Rollback(ctx)

	inserted := []models.ClipCandidate{}

	for i, item := range req.Clips {
		startMs, err := parseTimestampToMs(item.Start)
		if err != nil {
			respondError(w, http.StatusBadRequest, "clip #"+string(rune(i+1))+": "+err.Error())
			return
		}
		endMs, err := parseTimestampToMs(item.End)
		if err != nil {
			respondError(w, http.StatusBadRequest, "clip #"+string(rune(i+1))+": "+err.Error())
			return
		}
		if endMs <= startMs {
			respondError(w, http.StatusBadRequest, "clip end must be after start")
			return
		}

		var clip models.ClipCandidate
		query := `
			INSERT INTO clip_candidates (project_id, title, start_ms, end_ms, reason, hook_score, source, order_index)
			VALUES ($1, $2, $3, $4, $5, $6, 'ai_import', $7)
			RETURNING id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), COALESCE(hook_score, 0), source, order_index, created_at, updated_at
		`
		err = tx.QueryRow(ctx, query, projectID, item.Title, startMs, endMs, item.Reason, item.HookScore, i).Scan(
			&clip.ID, &clip.ProjectID, &clip.Title, &clip.StartMs, &clip.EndMs,
			&clip.Reason, &clip.HookScore, &clip.Source, &clip.OrderIndex,
			&clip.CreatedAt, &clip.UpdatedAt,
		)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to insert clip: "+err.Error())
			return
		}

		inserted = append(inserted, clip)
	}

	if err := tx.Commit(ctx); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to commit transaction: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, inserted)
}

// GET /api/projects/{id}/clips
func (h *ClipHandler) ListClips(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	query := `
		SELECT id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), COALESCE(hook_score, 0), source, order_index, created_at, updated_at
		FROM clip_candidates
		WHERE project_id = $1
		ORDER BY order_index ASC, start_ms ASC
	`
	rows, err := h.DB.Query(context.Background(), query, projectID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list clips: "+err.Error())
		return
	}
	defer rows.Close()

	clips := []models.ClipCandidate{}
	for rows.Next() {
		var c models.ClipCandidate
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.StartMs, &c.EndMs,
			&c.Reason, &c.HookScore, &c.Source, &c.OrderIndex, &c.CreatedAt, &c.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to scan clip: "+err.Error())
			return
		}
		clips = append(clips, c)
	}

	respondJSON(w, http.StatusOK, clips)
}

// POST /api/projects/{id}/clips
func (h *ClipHandler) CreateClip(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	var req models.CreateClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EndMs <= req.StartMs {
		respondError(w, http.StatusBadRequest, "end_ms must be greater than start_ms")
		return
	}

	var clip models.ClipCandidate
	query := `
		INSERT INTO clip_candidates (project_id, title, start_ms, end_ms, source, order_index)
		VALUES ($1, $2, $3, $4, 'manual', (
			SELECT COALESCE(MAX(order_index), -1) + 1 FROM clip_candidates WHERE project_id = $1
		))
		RETURNING id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), COALESCE(hook_score, 0), source, order_index, created_at, updated_at
	`
	err := h.DB.QueryRow(context.Background(), query, projectID, req.Title, req.StartMs, req.EndMs).Scan(
		&clip.ID, &clip.ProjectID, &clip.Title, &clip.StartMs, &clip.EndMs,
		&clip.Reason, &clip.HookScore, &clip.Source, &clip.OrderIndex,
		&clip.CreatedAt, &clip.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create clip: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, clip)
}

// PUT /api/clips/{id}
func (h *ClipHandler) UpdateClip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req models.UpdateClipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	query := `
		UPDATE clip_candidates SET
			title = COALESCE($1, title),
			start_ms = COALESCE($2, start_ms),
			end_ms = COALESCE($3, end_ms),
			updated_at = now()
		WHERE id = $4
		RETURNING id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), COALESCE(hook_score, 0), source, order_index, created_at, updated_at
	`

	var clip models.ClipCandidate
	err := h.DB.QueryRow(context.Background(), query, req.Title, req.StartMs, req.EndMs, id).Scan(
		&clip.ID, &clip.ProjectID, &clip.Title, &clip.StartMs, &clip.EndMs,
		&clip.Reason, &clip.HookScore, &clip.Source, &clip.OrderIndex,
		&clip.CreatedAt, &clip.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "clip not found or failed to update: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, clip)
}

// DELETE /api/clips/{id}
func (h *ClipHandler) DeleteClip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tag, err := h.DB.Exec(context.Background(), `DELETE FROM clip_candidates WHERE id = $1`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete clip: "+err.Error())
		return
	}
	if tag.RowsAffected() == 0 {
		respondError(w, http.StatusNotFound, "clip not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/clips/{id}/render
func (h *ClipHandler) TriggerRender(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")
	ctx := context.Background()

	var projectID string
	err := h.DB.QueryRow(ctx, `SELECT project_id FROM clip_candidates WHERE id = $1`, clipID).Scan(&projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "clip not found")
		return
	}

	payload, _ := json.Marshal(map[string]string{"clip_id": clipID})

	var jobID string
	err = h.DB.QueryRow(ctx, `
		INSERT INTO jobs (project_id, job_type, status, payload_json)
		VALUES ($1, 'render', 'queued', $2)
		RETURNING id
	`, projectID, payload).Scan(&jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create render job: "+err.Error())
		return
	}

	h.Queue.Enqueue(jobID)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"job_id": jobID,
		"status": "queued",
	})
}

// GET /api/clips/{id}/rendered
func (h *ClipHandler) GetRenderedClip(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")

	query := `
		SELECT id, output_path, status, file_size, created_at
		FROM rendered_clips WHERE clip_candidate_id = $1
		ORDER BY created_at DESC LIMIT 1
	`
	var id, outputPath, status string
	var fileSize int64
	var createdAt interface{}

	err := h.DB.QueryRow(context.Background(), query, clipID).Scan(&id, &outputPath, &status, &fileSize, &createdAt)
	if err != nil {
		respondError(w, http.StatusNotFound, "no rendered clip found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":          id,
		"output_path": outputPath,
		"status":      status,
		"file_size":   fileSize,
	})
}

// GET /api/clips/{id}/download
func (h *ClipHandler) DownloadRenderedClip(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")

	var outputPath, title string
	err := h.DB.QueryRow(context.Background(), `
		SELECT rc.output_path, cc.title
		FROM rendered_clips rc
		JOIN clip_candidates cc ON cc.id = rc.clip_candidate_id
		WHERE rc.clip_candidate_id = $1
		ORDER BY rc.created_at DESC LIMIT 1
	`, clipID).Scan(&outputPath, &title)
	if err != nil {
		respondError(w, http.StatusNotFound, "rendered clip not found")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+title+".mp4\"")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, outputPath)
}