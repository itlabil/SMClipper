package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
)

type ClipHandler struct {
	DB *pgxpool.Pool
}

func NewClipHandler(db *pgxpool.Pool) *ClipHandler {
	return &ClipHandler{DB: db}
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
			RETURNING id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), hook_score, source, order_index, created_at, updated_at
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
		SELECT id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), hook_score, source, order_index, created_at, updated_at
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
		RETURNING id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), hook_score, source, order_index, created_at, updated_at
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
		RETURNING id, project_id, title, start_ms, end_ms, COALESCE(reason, ''), hook_score, source, order_index, created_at, updated_at
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