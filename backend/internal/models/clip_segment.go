package models

import "time"

type ClipSegment struct {
	ID              string       `json:"id"`
	ClipCandidateID string       `json:"clip_candidate_id"`
	StartMs         int          `json:"start_ms"`
	EndMs           int          `json:"end_ms"`
	LayoutType      string       `json:"layout_type"`
	CropRegions     []CropRegion `json:"crop_regions"`
	OrderIndex      int          `json:"order_index"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type UpdateSegmentRequest struct {
	LayoutType  string       `json:"layout_type"`
	CropRegions []CropRegion `json:"crop_regions"`
}