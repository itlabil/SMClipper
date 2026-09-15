package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/jobs"
	"smclipper-backend/internal/models"
)

type SegmentHandler struct {
	DB *pgxpool.Pool
}

func NewSegmentHandler(db *pgxpool.Pool) *SegmentHandler {
	return &SegmentHandler{DB: db}
}

// POST /api/clips/{id}/segments/detect
func (h *SegmentHandler) DetectSegments(w http.ResponseWriter, r *http.Request) {
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

	var sourcePath string
	err = h.DB.QueryRow(ctx, `SELECT source_path FROM videos WHERE project_id = $1`, projectID).Scan(&sourcePath)
	if err != nil {
		respondError(w, http.StatusNotFound, "video not found")
		return
	}

	cuts, err := jobs.DetectSceneCuts(sourcePath, startMs, endMs, 0.3)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to detect scenes: "+err.Error())
		return
	}

	clipDurationMs := endMs - startMs
	boundaries := append([]int{0}, cuts...)
	boundaries = append(boundaries, clipDurationMs)

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Hapus segment lama (kalau ada), ganti dengan hasil deteksi baru
	_, err = tx.Exec(ctx, `DELETE FROM clip_segments WHERE clip_candidate_id = $1`, clipID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to clear old segments: "+err.Error())
		return
	}

	segments := []models.ClipSegment{}
	for i := 0; i < len(boundaries)-1; i++ {
		var seg models.ClipSegment
		query := `
			INSERT INTO clip_segments (clip_candidate_id, start_ms, end_ms, layout_type, crop_regions_json, order_index)
			VALUES ($1, $2, $3, 'single_crop', '[]', $4)
			RETURNING id, clip_candidate_id, start_ms, end_ms, layout_type, crop_regions_json, order_index, created_at, updated_at
		`
		var cropRaw []byte
		err = tx.QueryRow(ctx, query, clipID, boundaries[i], boundaries[i+1], i).Scan(
			&seg.ID, &seg.ClipCandidateID, &seg.StartMs, &seg.EndMs, &seg.LayoutType, &cropRaw, &seg.OrderIndex, &seg.CreatedAt, &seg.UpdatedAt,
		)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save segment: "+err.Error())
			return
		}
		json.Unmarshal(cropRaw, &seg.CropRegions)
		segments = append(segments, seg)
	}

	if err := tx.Commit(ctx); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to commit: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, segments)
}

// GET /api/clips/{id}/segments
func (h *SegmentHandler) ListSegments(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")

	query := `
		SELECT id, clip_candidate_id, start_ms, end_ms, layout_type, crop_regions_json, order_index, created_at, updated_at
		FROM clip_segments WHERE clip_candidate_id = $1
		ORDER BY order_index ASC
	`
	rows, err := h.DB.Query(context.Background(), query, clipID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list segments: "+err.Error())
		return
	}
	defer rows.Close()

	segments := []models.ClipSegment{}
	for rows.Next() {
		var seg models.ClipSegment
		var cropRaw []byte
		if err := rows.Scan(&seg.ID, &seg.ClipCandidateID, &seg.StartMs, &seg.EndMs, &seg.LayoutType, &cropRaw, &seg.OrderIndex, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to scan segment: "+err.Error())
			return
		}
		json.Unmarshal(cropRaw, &seg.CropRegions)
		segments = append(segments, seg)
	}

	respondJSON(w, http.StatusOK, segments)
}

// PUT /api/segments/{id}
func (h *SegmentHandler) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req models.UpdateSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cropJSON, err := json.Marshal(req.CropRegions)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to encode crop regions")
		return
	}

	query := `
		UPDATE clip_segments SET
			layout_type = $1,
			crop_regions_json = $2,
			updated_at = now()
		WHERE id = $3
		RETURNING id, clip_candidate_id, start_ms, end_ms, layout_type, crop_regions_json, order_index, created_at, updated_at
	`
	var seg models.ClipSegment
	var cropRaw []byte
	err = h.DB.QueryRow(context.Background(), query, req.LayoutType, cropJSON, id).Scan(
		&seg.ID, &seg.ClipCandidateID, &seg.StartMs, &seg.EndMs, &seg.LayoutType, &cropRaw, &seg.OrderIndex, &seg.CreatedAt, &seg.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "segment not found or failed to update: "+err.Error())
		return
	}
	json.Unmarshal(cropRaw, &seg.CropRegions)

	respondJSON(w, http.StatusOK, seg)
}