package jobs

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

var progressRegex = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)

func (q *Queue) runDownload(ctx context.Context, jobID, projectID string) error {
	var youtubeURL string
	err := q.db.QueryRow(ctx, `SELECT youtube_url FROM projects WHERE id = $1`, projectID).Scan(&youtubeURL)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	q.db.Exec(ctx, `UPDATE projects SET status = 'downloading', updated_at = now() WHERE id = $1`, projectID)

	projectDir := filepath.Join(q.storagePath, projectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	outputTemplate := filepath.Join(projectDir, "source.%(ext)s")

	cmd := exec.Command("yt-dlp",
		"-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]",
		"--merge-output-format", "mp4",
		"--newline",
		"-o", outputTemplate,
		youtubeURL,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // gabungkan stderr ke stdout biar error ke-capture juga

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	lastProgress := -1
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		matches := progressRegex.FindStringSubmatch(line)
		if len(matches) == 2 {
			pct, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				progress := int(pct)
				if progress != lastProgress {
					q.updateJobStatus(jobID, "processing", progress, nil)
					lastProgress = progress
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("yt-dlp failed: %w", err)
	}

	sourcePath := filepath.Join(projectDir, "source.mp4")
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("expected output file not found: %s", sourcePath)
	}

	info, err := ProbeVideo(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to probe video: %w", err)
	}

	_, err = q.db.Exec(ctx, `
		INSERT INTO videos (project_id, source_path, resolution, duration_seconds, fps)
		VALUES ($1, $2, $3, $4, $5)
	`, projectID, sourcePath, ResolutionLabel(info.Height), info.DurationSeconds, info.FPS)
	if err != nil {
		return fmt.Errorf("failed to save video record: %w", err)
	}

	_, err = q.db.Exec(ctx, `UPDATE projects SET status = 'downloaded', updated_at = now() WHERE id = $1`, projectID)
	if err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	return nil
}