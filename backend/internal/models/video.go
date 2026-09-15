package models

import "time"

type Video struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	SourcePath      string    `json:"source_path"`
	Resolution      string    `json:"resolution"`
	DurationSeconds int       `json:"duration_seconds"`
	FPS             int       `json:"fps"`
	ThumbnailPath   string    `json:"thumbnail_path"`
	CreatedAt       time.Time `json:"created_at"`
}