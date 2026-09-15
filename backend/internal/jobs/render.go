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

	// Cek apakah clip ini punya segments (layout dinamis per potongan waktu)
	segRows, err := q.db.Query(ctx, `
		SELECT start_ms, end_ms, layout_type, crop_regions_json
		FROM clip_segments WHERE clip_candidate_id = $1 ORDER BY order_index ASC
	`, clipID)
	if err != nil {
		return fmt.Errorf("failed to fetch segments: %w", err)
	}
	type segmentData struct {
		StartMs, EndMs int
		LayoutType     string
		CropRegions    []models.CropRegion
	}
	var segments []segmentData
	for segRows.Next() {
		var s segmentData
		var cropRaw2 []byte
		if err := segRows.Scan(&s.StartMs, &s.EndMs, &s.LayoutType, &cropRaw2); err != nil {
			segRows.Close()
			return fmt.Errorf("failed to scan segment: %w", err)
		}
		json.Unmarshal(cropRaw2, &s.CropRegions)
		segments = append(segments, s)
	}
	segRows.Close()

	if len(cropRegions) == 0 && len(segments) == 0 {
		return fmt.Errorf("no crop regions or segments defined for this clip")
	}

	// Validasi: kalau ada segments, pastikan semua sudah punya crop region
	for i, s := range segments {
		required := 1
		if s.LayoutType == "split_top_bottom" {
			required = 2
		}
		if len(s.CropRegions) < required {
			return fmt.Errorf("segment #%d (layout: %s) belum punya crop region lengkap", i+1, s.LayoutType)
		}
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

	if len(segments) > 0 {
		// Mode segment: tiap potongan waktu punya layout sendiri, digabung pakai concat
		var parts []string
		var segLabels []string
		halfH := targetH / 2

		for i, s := range segments {
			startSec := float64(s.StartMs) / 1000.0
			endSec := float64(s.EndMs) / 1000.0
			trimLabel := fmt.Sprintf("traw%d", i)
			segLabel := fmt.Sprintf("seg%d", i)

			parts = append(parts, fmt.Sprintf(
				"[0:v]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS[%s]",
				startSec, endSec, trimLabel,
			))

			if s.LayoutType == "split_top_bottom" && len(s.CropRegions) >= 2 {
				r1 := s.CropRegions[0]
				r2 := s.CropRegions[1]
				topLabel := fmt.Sprintf("top%d", i)
				botLabel := fmt.Sprintf("bot%d", i)
				splitA := fmt.Sprintf("sA%d", i)
				splitB := fmt.Sprintf("sB%d", i)
				parts = append(parts, fmt.Sprintf("[%s]split=2[%s][%s]", trimLabel, splitA, splitB))
				parts = append(parts, fmt.Sprintf(
					"[%s]crop=%d:%d:%d:%d,scale=%d:%d,setsar=1[%s]",
					splitA, r1.W, r1.H, r1.X, r1.Y, targetW, halfH, topLabel,
				))
				parts = append(parts, fmt.Sprintf(
					"[%s]crop=%d:%d:%d:%d,scale=%d:%d,setsar=1[%s]",
					splitB, r2.W, r2.H, r2.X, r2.Y, targetW, halfH, botLabel,
				))
				parts = append(parts, fmt.Sprintf("[%s][%s]vstack=inputs=2[%s]", topLabel, botLabel, segLabel))
			} else {
				r := s.CropRegions[0]
				parts = append(parts, fmt.Sprintf(
					"[%s]crop=%d:%d:%d:%d,scale=%d:%d,setsar=1[%s]",
					trimLabel, r.W, r.H, r.X, r.Y, targetW, targetH, segLabel,
				))
			}
			segLabels = append(segLabels, "["+segLabel+"]")
		}

		concatInputs := ""
		for _, l := range segLabels {
			concatInputs += l
		}
		parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[outv]", concatInputs, len(segments)))

		filterComplex = strings.Join(parts, ";")
	} else if layoutType == "split_top_bottom" && len(cropRegions) >= 2 {
		r1 := cropRegions[0]
		r2 := cropRegions[1]
		halfH := targetH / 2
		filterComplex = fmt.Sprintf(
			"[0:v]crop=%d:%d:%d:%d,scale=%d:%d,setsar=1[top];[0:v]crop=%d:%d:%d:%d,scale=%d:%d,setsar=1[bottom];[top][bottom]vstack=inputs=2[outv]",
			r1.W, r1.H, r1.X, r1.Y, targetW, halfH,
			r2.W, r2.H, r2.X, r2.Y, targetW, halfH,
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