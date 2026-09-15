package models

import "time"

type ClipCandidate struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Title      string    `json:"title"`
	StartMs    int       `json:"start_ms"`
	EndMs      int       `json:"end_ms"`
	Reason     string    `json:"reason"`
	HookScore  int       `json:"hook_score"`
	Source     string    `json:"source"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Skema JSON yang diupload user, hasil dari AI eksternal
type ImportClipsRequest struct {
	Clips []ImportClipItem `json:"clips"`
}

type ImportClipItem struct {
	Title     string `json:"title"`
	Start     string `json:"start"`      // format "HH:MM:SS" atau "MM:SS"
	End       string `json:"end"`
	Reason    string `json:"reason"`
	HookScore int    `json:"hook_score"`
}

type CreateClipRequest struct {
	Title   string `json:"title"`
	StartMs int    `json:"start_ms"`
	EndMs   int    `json:"end_ms"`
}

type UpdateClipRequest struct {
	Title   *string `json:"title"`
	StartMs *int    `json:"start_ms"`
	EndMs   *int    `json:"end_ms"`
}