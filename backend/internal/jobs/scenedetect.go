package jobs

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
)

var ptsTimeRegex = regexp.MustCompile(`pts_time:([\d.]+)`)

// DetectSceneCuts mendeteksi timestamp (dalam ms, relatif terhadap awal clip)
// di mana kemungkinan terjadi pergantian kamera/scene, menggunakan FFmpeg scene filter.
// threshold 0.0-1.0, makin kecil makin sensitif (lebih banyak cut terdeteksi).
func DetectSceneCuts(sourcePath string, clipStartMs, clipEndMs int, threshold float64) ([]int, error) {
	startSec := float64(clipStartMs) / 1000.0
	durationSec := float64(clipEndMs-clipStartMs) / 1000.0

	filter := fmt.Sprintf("select='gt(scene,%.2f)',showinfo", threshold)

	cmd := exec.Command("ffmpeg",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", sourcePath,
		"-t", fmt.Sprintf("%.3f", durationSec),
		"-filter:v", filter,
		"-f", "null", "-",
	)

	output, _ := cmd.CombinedOutput() // ffmpeg selalu "error" karena output null, abaikan err, cek dari output text

	matches := ptsTimeRegex.FindAllStringSubmatch(string(output), -1)

	var cutsMs []int
	for _, m := range matches {
		sec, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		ms := int(sec * 1000)
		// Abaikan cut yang terlalu dekat dengan awal/akhir clip (< 1 detik), tidak berguna jadi segmen terpisah
		if ms < 1000 || ms > (clipEndMs-clipStartMs)-1000 {
			continue
		}
		cutsMs = append(cutsMs, ms)
	}

	sort.Ints(cutsMs)

	// Buang duplikat/cut yang terlalu berdekatan (< 1.5 detik antar cut)
	var filtered []int
	for _, c := range cutsMs {
		if len(filtered) == 0 || c-filtered[len(filtered)-1] >= 1500 {
			filtered = append(filtered, c)
		}
	}

	return filtered, nil
}