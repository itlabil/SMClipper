package models

import "time"

type Job struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	JobType      string    `json:"job_type"`
	Status       string    `json:"status"`
	Progress     int       `json:"progress"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}