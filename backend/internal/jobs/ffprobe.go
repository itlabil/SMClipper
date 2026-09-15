package jobs

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

type VideoInfo struct {
	DurationSeconds int
	Width           int
	Height          int
	FPS             int
}

type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType    string `json:"codec_type"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
}

func ProbeVideo(path string) (*VideoInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, err
	}

	info := &VideoInfo{}

	durationFloat, _ := strconv.ParseFloat(probe.Format.Duration, 64)
	info.DurationSeconds = int(durationFloat)

	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			info.Width = s.Width
			info.Height = s.Height
			info.FPS = parseFrameRate(s.RFrameRate)
			break
		}
	}

	return info, nil
}

// parseFrameRate mengubah format "30000/1001" jadi int fps (dibulatkan)
func parseFrameRate(rate string) int {
	parts := strings.Split(rate, "/")
	if len(parts) != 2 {
		return 0
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return int(num / den)
}

func ResolutionLabel(height int) string {
	if height >= 1080 {
		return "1080p"
	}
	if height >= 720 {
		return "720p"
	}
	return "sd"
}