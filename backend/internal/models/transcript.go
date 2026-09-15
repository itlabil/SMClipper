package models

import "time"

type Transcript struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	RawJSONPath   string    `json:"raw_json_path"`
	TxtExportPath string    `json:"txt_export_path"`
	Language      string    `json:"language"`
	CreatedAt     time.Time `json:"created_at"`
}