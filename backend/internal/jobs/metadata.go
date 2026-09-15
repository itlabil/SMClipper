package jobs

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type YoutubeMetadata struct {
	Title   string
	Channel string
}

type ytDlpInfoOutput struct {
	Title   string `json:"title"`
	Uploader string `json:"uploader"`
	Channel  string `json:"channel"`
}

// FetchYoutubeMetadata mengambil title & channel name TANPA mendownload video.
func FetchYoutubeMetadata(youtubeURL string) (*YoutubeMetadata, error) {
	cmd := exec.Command("yt-dlp",
		"--dump-json",
		"--skip-download",
		"--no-warnings",
		youtubeURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	var info ytDlpInfoOutput
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	channel := info.Channel
	if channel == "" {
		channel = info.Uploader
	}

	return &YoutubeMetadata{
		Title:   info.Title,
		Channel: channel,
	}, nil
}