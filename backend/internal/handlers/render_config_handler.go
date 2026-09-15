package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"smclipper-backend/internal/models"
)

type RenderConfigHandler struct {
	DB *pgxpool.Pool
}

func NewRenderConfigHandler(db *pgxpool.Pool) *RenderConfigHandler {
	return &RenderConfigHandler{DB: db}
}

// PUT /api/clips/{id}/render-config
func (h *RenderConfigHandler) UpsertRenderConfig(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")

	var req models.UpsertRenderConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.LayoutType == "" {
		req.LayoutType = "single_crop"
	}
	if req.AspectRatio == "" {
		req.AspectRatio = "9:16"
	}

	cropJSON, err := json.Marshal(req.CropRegions)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to encode crop regions")
		return
	}

	query := `
		INSERT INTO clip_render_configs (clip_candidate_id, layout_type, aspect_ratio, crop_regions_json, subtitle_enabled)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (clip_candidate_id) DO UPDATE SET
			layout_type = EXCLUDED.layout_type,
			aspect_ratio = EXCLUDED.aspect_ratio,
			crop_regions_json = EXCLUDED.crop_regions_json,
			subtitle_enabled = EXCLUDED.subtitle_enabled,
			updated_at = now()
		RETURNING id, clip_candidate_id, layout_type, aspect_ratio, crop_regions_json, subtitle_enabled, created_at, updated_at
	`

	var cfg models.ClipRenderConfig
	var cropRaw []byte
	err = h.DB.QueryRow(context.Background(), query,
		clipID, req.LayoutType, req.AspectRatio, cropJSON, req.SubtitleEnabled,
	).Scan(&cfg.ID, &cfg.ClipCandidateID, &cfg.LayoutType, &cfg.AspectRatio, &cropRaw, &cfg.SubtitleEnabled, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save render config: "+err.Error())
		return
	}

	json.Unmarshal(cropRaw, &cfg.CropRegions)

	respondJSON(w, http.StatusOK, cfg)
}

// GET /api/clips/{id}/render-config
func (h *RenderConfigHandler) GetRenderConfig(w http.ResponseWriter, r *http.Request) {
	clipID := r.PathValue("id")

	query := `
		SELECT id, clip_candidate_id, layout_type, aspect_ratio, crop_regions_json, subtitle_enabled, created_at, updated_at
		FROM clip_render_configs WHERE clip_candidate_id = $1
	`
	var cfg models.ClipRenderConfig
	var cropRaw []byte
	err := h.DB.QueryRow(context.Background(), query, clipID).Scan(
		&cfg.ID, &cfg.ClipCandidateID, &cfg.LayoutType, &cfg.AspectRatio, &cropRaw, &cfg.SubtitleEnabled, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		respondError(w, http.StatusNotFound, "render config not found")
		return
	}
	json.Unmarshal(cropRaw, &cfg.CropRegions)

	respondJSON(w, http.StatusOK, cfg)
}