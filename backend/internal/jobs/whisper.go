package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type whisperOutput struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []whisperSegment `json:"transcription"`
}

type whisperSegment struct {
	Timestamps struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"timestamps"`
	Offsets struct {
		From int `json:"from"`
		To   int `json:"to"`
	} `json:"offsets"`
	Text string `json:"text"`
}

func (q *Queue) runTranscribe(ctx context.Context, jobID, projectID string) error {
	var sourcePath string
	err := q.db.QueryRow(ctx, `SELECT source_path FROM videos WHERE project_id = $1`, projectID).Scan(&sourcePath)
	if err != nil {
		return fmt.Errorf("failed to fetch video: %w", err)
	}

	q.db.Exec(ctx, `UPDATE projects SET status = 'transcribing', updated_at = now() WHERE id = $1`, projectID)

	projectDir := filepath.Dir(sourcePath)
	audioPath := filepath.Join(projectDir, "audio.wav")
	transcriptPrefix := filepath.Join(projectDir, "transcript")
	transcriptJSONPath := transcriptPrefix + ".json"
	transcriptTxtPath := filepath.Join(projectDir, "transcript.txt")

	// 1. Extract audio jadi WAV 16kHz mono
	q.updateJobStatus(jobID, "processing", 10, nil)
	extractCmd := exec.Command("ffmpeg", "-y",
		"-i", sourcePath,
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le",
		audioPath,
	)
	if output, err := extractCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to extract audio: %w (output: %s)", err, string(output))
	}

	// 2. Jalankan whisper-cli
	q.updateJobStatus(jobID, "processing", 30, nil)
	whisperCmd := exec.Command(q.whisperBinPath,
		"-m", q.whisperModelPath,
		"-f", audioPath,
		"-l", "auto",
		"-ojf",
		"-of", transcriptPrefix,
	)
	if output, err := whisperCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("whisper-cli failed: %w (output: %s)", err, string(output))
	}

	q.updateJobStatus(jobID, "processing", 80, nil)

	// 3. Parse JSON hasil whisper
	raw, err := os.ReadFile(transcriptJSONPath)
	if err != nil {
		return fmt.Errorf("failed to read transcript json: %w", err)
	}

	var whisperOut whisperOutput
	if err := json.Unmarshal(raw, &whisperOut); err != nil {
		return fmt.Errorf("failed to parse transcript json: %w", err)
	}

	// 4. Generate txt export (per baris ada timestamp, ini yang nanti dilempar ke AI)
	var sb strings.Builder
	for _, seg := range whisperOut.Transcription {
		sb.WriteString(fmt.Sprintf("[%s --> %s] %s\n",
			seg.Timestamps.From, seg.Timestamps.To, strings.TrimSpace(seg.Text)))
	}
	if err := os.WriteFile(transcriptTxtPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write txt export: %w", err)
	}

	// 5. Simpan record ke database
	_, err = q.db.Exec(ctx, `
		INSERT INTO transcripts (project_id, raw_json_path, txt_export_path, language)
		VALUES ($1, $2, $3, $4)
	`, projectID, transcriptJSONPath, transcriptTxtPath, whisperOut.Result.Language)
	if err != nil {
		return fmt.Errorf("failed to save transcript record: %w", err)
	}

	_, err = q.db.Exec(ctx, `UPDATE projects SET status = 'transcribed', updated_at = now() WHERE id = $1`, projectID)
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return nil
}