package models

import "time"

type Project struct {
	ID         string    `json:"id"`
	YoutubeURL string    `json:"youtube_url"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateProjectRequest struct {
	YoutubeURL string `json:"youtube_url"`
}