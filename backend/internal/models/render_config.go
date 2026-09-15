package models

import (
	"encoding/json"
	"time"
)

type CropRegion struct {
	Speaker string `json:"speaker,omitempty"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	W       int    `json:"w"`
	H       int    `json:"h"`
}

type ClipRenderConfig struct {
	ID                string          `json:"id"`
	ClipCandidateID   string          `json:"clip_candidate_id"`
	LayoutType        string          `json:"layout_type"`
	AspectRatio       string          `json:"aspect_ratio"`
	CropRegions       []CropRegion    `json:"crop_regions"`
	SubtitleEnabled   bool            `json:"subtitle_enabled"`
	SubtitleStyleJSON json.RawMessage `json:"subtitle_style,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type UpsertRenderConfigRequest struct {
	LayoutType      string       `json:"layout_type"`
	AspectRatio     string       `json:"aspect_ratio"`
	CropRegions     []CropRegion `json:"crop_regions"`
	SubtitleEnabled bool         `json:"subtitle_enabled"`
}