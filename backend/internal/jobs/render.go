package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"log"

	"smclipper-backend/internal/models"
)

type renderJobPayload struct {
	ClipID string `json:"clip_id"`
}

func (q *Queue) runRender(ctx context.Context, jobID, projectID string) error {
	var payloadStr string
	err := q.db.QueryRow(ctx, `SELECT payload_json::text FROM jobs WHERE id = $1`, jobID).Scan(&payloadStr)
	if err != nil {
		return fmt.Errorf("failed to fetch job payload: %w", err)
	}
	var payload renderJobPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse job payload: %w", err)
	}
	clipID := payload.ClipID

	var startMs, endMs int
	err = q.db.QueryRow(ctx, `SELECT start_ms, end_ms FROM clip_candidates WHERE id = $1`, clipID).
		Scan(&startMs, &endMs)
	if err != nil {
		return fmt.Errorf("failed to fetch clip: %w", err)
	}

	var sourcePath string
	err = q.db.QueryRow(ctx, `SELECT source_path FROM videos WHERE project_id = $1`, projectID).Scan(&sourcePath)
	if err != nil {
		return fmt.Errorf("failed to fetch video: %w", err)
	}

	var layoutType, aspectRatio string
	var cropRaw []byte
	var subtitleEnabled bool
	err = q.db.QueryRow(ctx, `
		SELECT layout_type, aspect_ratio, crop_regions_json, subtitle_enabled
		FROM clip_render_configs WHERE clip_candidate_id = $1
	`, clipID).Scan(&layoutType, &aspectRatio, &cropRaw, &subtitleEnabled)
	if err != nil {
		return fmt.Errorf("render config not set for this clip: %w", err)
	}

	var cropRegions []models.CropRegion
	if err := json.Unmarshal(cropRaw, &cropRegions); err != nil {
		return fmt.Errorf("failed to parse crop regions: %w", err)
	}
	if len(cropRegions) == 0 {
		return fmt.Errorf("no crop regions defined in render config")
	}

	q.updateJobStatus(jobID, "processing", 10, nil)

	outputDir := filepath.Join(filepath.Dir(sourcePath), "clips", clipID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	outputPath := filepath.Join(outputDir, "final.mp4")
	subtitlePath := filepath.Join(outputDir, "subtitle.ass")

	startSec := float64(startMs) / 1000.0
	durationSec := float64(endMs-startMs) / 1000.0
	targetW, targetH := 1080, 1920

	var filterComplex string

	if layoutType == "split_top_bottom" && len(cropRegions) >= 2 {
		r1 := cropRegions[0]
		r2 := cropRegions[1]
		filterComplex = fmt.Sprintf(
			"[0:v]crop=%d:%d:%d:%d[top];[0:v]crop=%d:%d:%d:%d[bottom];[top][bottom]vstack=inputs=2,scale=%d:%d[outv]",
			r1.W, r1.H, r1.X, r1.Y,
			r2.W, r2.H, r2.X, r2.Y,
			targetW, targetH,
		)
	} else {
		r := cropRegions[0]
		filterComplex = fmt.Sprintf(
			"[0:v]crop=%d:%d:%d:%d,scale=%d:%d[outv]",
			r.W, r.H, r.X, r.Y,
			targetW, targetH,
		)
	}

	if subtitleEnabled {
		if _, err := os.Stat(subtitlePath); err == nil {
			escapedPath := escapeFFmpegPath(subtitlePath)
			filterComplex = strings.Replace(filterComplex, "[outv]", fmt.Sprintf(",ass=%s[outv]", escapedPath), 1)
		}
	}

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", sourcePath,
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-filter_complex", filterComplex,
		"-map", "[outv]",
		"-map", "0:a",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
		"-c:a", "aac",
		outputPath,
	}

	q.updateJobStatus(jobID, "processing", 30, nil)

	log.Printf("[render] filterComplex: %s", filterComplex)
	log.Printf("[render] ffmpeg args: %v", args)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg render failed: %w (output: %s)", err, string(output))
	}

	q.updateJobStatus(jobID, "processing", 90, nil)

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return fmt.Errorf("output file not found after render: %w", err)
	}

	_, err = q.db.Exec(ctx, `
		INSERT INTO rendered_clips (clip_candidate_id, job_id, output_path, status, file_size)
		VALUES ($1, $2, $3, 'done', $4)
	`, clipID, jobID, outputPath, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("failed to save rendered clip record: %w", err)
	}

	return nil
}

func escapeFFmpegPath(path string) string {
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `:`, `\:`)
	return escaped
}